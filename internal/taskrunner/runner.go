package taskrunner

import (
	"bytes"
	"dotbuilder/internal/config"
	"dotbuilder/internal/dag"
	"dotbuilder/internal/pkgmanager"
	"dotbuilder/pkg/logger"
	"dotbuilder/pkg/shell"
	"os"
	"strings"
	"sync"
	"text/template"
)

func resolveMapVariables(vars map[string]string) {
	data := map[string]interface{}{"vars": vars}
	for k, v := range vars {
		if strings.Contains(v, "{{") {
			tmpl, err := template.New("v").Parse(v)
			if err == nil {
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, data); err == nil {
					vars[k] = buf.String()
				}
			}
		}
	}
}

func ExecuteTaskLogic(t config.Task, runner *shell.Runner, globalVars map[string]string) error {
	logger.Debug("Task Logic: [%s]", t.ID)

	// Merge Vars
	vars := make(map[string]string)
	for k, v := range globalVars {
		vars[k] = v
	}
	for k, v := range t.Vars {
		vars[k] = v
	}

	resolveMapVariables(vars)

	tplData := map[string]interface{}{
		"vars": vars,
		"name": t.ID,
	}

	checkPassed := false
	checkRun := false

	if t.Check != "" {
		checkRun = true
		renderedCheck := pkgmanager.RenderCmd(t.Check, tplData)

		if strings.HasPrefix(renderedCheck, "exists:") {
			path := strings.TrimSpace(strings.TrimPrefix(renderedCheck, "exists:"))
			path = os.ExpandEnv(path) 
			if _, err := os.Stat(path); err == nil {
				checkPassed = true
			}
		} else {
			if runner.ExecSilent(renderedCheck) == 0 {
				checkPassed = true
			}
		}
	}

	shouldRun := true
	if checkRun {
		var action string
		if checkPassed {
			action = "skip" // 默认 Check 通过则 Skip
		} else {
			action = "run"
		}

		statusKey := "fail"
		if checkPassed {
			statusKey = "success"
		}
		
		if act, ok := t.On[statusKey]; ok {
			action = act
		}

		if action == "skip" {
			shouldRun = false
			logger.Success("[%s] Check passed (skipped).", t.ID)
		}
	}

	if shouldRun {
		runCmd := pkgmanager.RenderCmd(t.Run, tplData)
		logger.Info("Running Task: [%s]", t.ID)
		if err := runner.ExecStream(runCmd, t.ID); err != nil {
			return err
		}
		logger.Success("[%s] Completed.", t.ID)
	}
	return nil
}

func RunGeneric(nodes []Node, ctx *Context) {
	logger.Info("=== Start Generic DAG Execution ===")

	// 1. Build Graph & Map
	g := dag.New()
	nodeMap := make(map[string]Node)
	var ids []string

	for _, n := range nodes {
		id := n.ID()
		if _, exists := nodeMap[id]; exists {
			logger.Warn("Duplicate Node ID detected: %s (overwriting)", id)
		}
		nodeMap[id] = n
		ids = append(ids, id)
	}

	// Second Pass: Build Edges & Validate Deps
	for _, n := range nodes {
		id := n.ID()
		for _, dep := range n.Deps() {
			if _, exists := nodeMap[dep]; !exists {
				// [Fix] Critical: Missing dependency must cause exit, otherwise sorting loops
				logger.Error("Node [%s] depends on missing node [%s]. Execution aborted.", id, dep)
				os.Exit(1)
			}
			g.AddEdge(dep, id)
		}
	}

	// 2. Sort Layers
	layers, err := g.SortLayers(ids)
	if err != nil {
		logger.Error("DAG Cycle detected or Sort failed: %v", err)
		os.Exit(1)
	}

	for i, layer := range layers {
		logger.Info("--- Layer %d (%d items) ---", i+1, len(layer))
		
		batches := make(map[string][]BatchableNode)
		var singles []Node

		for _, id := range layer {
			n, ok := nodeMap[id]
			if !ok { continue }

			group := n.BatchGroup()
			addedToBatch := false
			if group != "" {
				if bn, ok := n.(BatchableNode); ok {	
					batches[group] = append(batches[group], bn)
					addedToBatch = true
				} 
			} 
			if !addedToBatch {
				singles = append(singles, n)
			}
		}

		var wg sync.WaitGroup
		errChan := make(chan error, len(singles)+len(batches)+1)

		for groupName, batchNodes := range batches {
			var names []string
			for _, bn := range batchNodes {
				names = append(names, bn.GetBatchItem())
			}
			
			wg.Add(1)
			go func(pm string, pkgNames []string) {
				defer wg.Done()
				ctx.PkgManager.InstallBatch(pm, pkgNames)
			}(groupName, names)
		}

		for _, n := range singles {
			wg.Add(1)
			go func(node Node) {
				defer wg.Done()
				if err := node.Execute(ctx); err != nil {
					logger.Error("[%s] Failed: %v", node.ID(), err)
					errChan <- err
				}
			}(n)
		}

		wg.Wait()
		close(errChan)

		hasError := false
		for range errChan {
			hasError = true
		}

		if hasError {
			logger.Error("Layer %d failed. Stopping execution.", i+1)
			os.Exit(1)
		}
	}
}