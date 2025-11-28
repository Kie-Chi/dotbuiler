package taskrunner

import (
	"bytes"
	"dotbuilder/internal/config"
	"dotbuilder/internal/dag"
	"dotbuilder/internal/pkgmanager"
	"dotbuilder/pkg/logger"
	"dotbuilder/pkg/shell"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"text/template"
	"time"
	"errors"
)

type ResultMap struct {
	sync.RWMutex
	m map[string]NodeResult
}

func (r *ResultMap) Set(id string, res NodeResult) {
	r.Lock()
	defer r.Unlock()
	r.m[id] = res
}

func (r *ResultMap) Get(id string) (NodeResult, bool) {
	r.RLock()
	defer r.RUnlock()
	res, ok := r.m[id]
	return res, ok
}

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

func RunGeneric(nodes []Node, ctx *Context) map[string]NodeResult {
	logger.Info("=== Start Generic DAG Execution ===")

	results := &ResultMap{m: make(map[string]NodeResult)}
	
	// 1. Build DAG
	g := dag.New()
	nodeMap := make(map[string]Node)
	var ids []string

	for _, n := range nodes {
		id := n.ID()
		nodeMap[id] = n
		ids = append(ids, id)
	}

	// 2. Build Edges
	for _, n := range nodes {
		id := n.ID()
		for _, dep := range n.Deps() {
			if _, exists := nodeMap[dep]; !exists {
				logger.Error("Node [%s] depends on missing node [%s]. Aborting.", id, dep)
				os.Exit(1)
			}
			g.AddEdge(dep, id)
		}
	}

	// 3. Sort and Layer
	layers, err := g.SortLayers(ids)
	if err != nil {
		logger.Error("DAG Error: %v", err)
		os.Exit(1)
	}

	// 4. Loop
	for i, layer := range layers {
		logger.Info("--- Layer %d (%d items) ---", i+1, len(layer))

		batches := make(map[string][]BatchableNode)
		var singles []Node

		for _, id := range layer {
			n := nodeMap[id]
			
			isBlocked := false
			var failedDep string

			for _, dep := range n.Deps() {
				res, ok := results.Get(dep)
				if !ok || (res.Status != StatusSuccess && res.Status != StatusSkipped) {
					isBlocked = true
					failedDep = dep
					break
				}
			}

			if isBlocked {
				results.Set(id, NodeResult{
					ID:        id,
					Status:    StatusBlocked,
					Error:     fmt.Errorf("dependency '%s' not satisfied", failedDep),
					Timestamp: time.Now(),
				})
				logger.Warn("[%s] Blocked by dependency: %s", id, failedDep)
				continue
			}

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
		for groupName, batchNodes := range batches {
			var names []string
			var nodeIDs []string
			for _, bn := range batchNodes {
				rawItem := bn.GetBatchItem()
				names = append(names, strings.Fields(rawItem)...)
				nodeIDs = append(nodeIDs, bn.ID())
			}

			if len(names) == 0 { continue }

			wg.Add(1)
			go func(pm string, pkgNames []string, ids []string) {
				defer wg.Done()
				start := time.Now()
				
				err := ctx.PkgManager.InstallBatch(pm, pkgNames)
				
				status := StatusSuccess
				if err != nil {
					status = StatusFailed
				}

				for _, id := range ids {
					results.Set(id, NodeResult{
						ID:        id,
						Status:    status,
						Error:     err,
						Duration:  time.Since(start),
						Timestamp: time.Now(),
					})
				}
			}(groupName, names, nodeIDs)
		}

		for _, n := range singles {
			wg.Add(1)
			go func(node Node) {
				defer wg.Done()
				start := time.Now()
				
				err := node.Execute(ctx)
				
				status := StatusSuccess
				var finalErr error = nil

				if err != nil {
					if errors.Is(err, pkgmanager.ErrSkipped) {
						status = StatusSkipped
						finalErr = nil 
					} else {
						status = StatusFailed
						finalErr = err
					}
				}

				results.Set(node.ID(), NodeResult{
					ID:        node.ID(),
					Status:    status,
					Error:     finalErr,
					Duration:  time.Since(start),
					Timestamp: time.Now(),
				})
			}(n)
		}

		wg.Wait()
	}

	return results.m
}


func PrintSummary(results map[string]NodeResult, nodes []Node) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tDURATION\tMESSAGE")
	fmt.Fprintln(w, "--\t------\t--------\t-------")

	for _, n := range nodes {
		id := n.ID()
		res, ok := results[id]
		if !ok {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, "UNKNOWN", "-", "Result not found")
			continue
		}

		statusStr := res.Status.String()
		color := logger.Reset
		switch res.Status {
		case StatusSuccess:
			color = logger.Green
		case StatusFailed:
			color = logger.Red
		case StatusBlocked:
			color = logger.Yellow
		case StatusSkipped:
			color = logger.Cyan
		}
		
		msg := ""
		if res.Error != nil {
			msg = res.Error.Error()
		}

		fmt.Fprintf(w, "%s\t%s%s%s\t%v\t%s\n", id, color, statusStr, logger.Reset, res.Duration.Round(time.Millisecond), msg)
	}
	w.Flush()
	hasFail := false
	for _, r := range results {
		if r.Status == StatusFailed || r.Status == StatusBlocked {
			hasFail = true
			break
		}
	}
	if hasFail {
		logger.Error("Build finished with errors.")
	}
}