package taskrunner

import (
	"dotbuilder/internal/config"
	"dotbuilder/internal/dag"
	"dotbuilder/internal/pkgmanager"
	"dotbuilder/pkg/logger"
	"dotbuilder/pkg/shell"
	"strconv"
)

// RunUnified merges packages and tasks into a single DAG and executes them with batch optimization
func RunUnified(pkgs []config.Package, tasks []config.Task, engine *pkgmanager.Engine, globalVars map[string]string) {
	logger.Info("=== Start unified DAG execution ===")

	g := dag.New()
	nodeMap := make(map[string]string) // id -> "pkg" | "task"

	// 1. Build Graph Nodes
	pkgIndex := make(map[string]*config.Package)
	for i := range pkgs {
		p := &pkgs[i]
		id := p.Name // assuming unique
		if id == "" {
			id = p.Def
		}
		nodeMap[id] = "pkg"
		pkgIndex[id] = p
		for _, dep := range p.Deps {
			g.AddEdge(dep, id)
		}
	}

	taskIndex := make(map[string]config.Task)
	for _, t := range tasks {
		nodeMap[t.ID] = "task"
		taskIndex[t.ID] = t
		for _, dep := range t.Deps {
			g.AddEdge(dep, t.ID)
		}
	}

	// 2. Sort
	var ids []string
	for id := range nodeMap {
		ids = append(ids, id)
	}
	sorted, err := g.Sort(ids)
	if err != nil {
		logger.Error("Unified DAG cycle: %v", err)
	}

	// 复用 Engine 中的 Runner (包含 DryRun 状态)
	runner := engine.Runner

	// 3. Execute with Batch Lookahead
	var batchBuffer []*config.Package

	// Helper to flush buffer
	flushBatch := func() {
		if len(batchBuffer) > 0 {
			engine.InstallBaseBatch(batchBuffer)
			batchBuffer = nil
		}
	}

	for _, id := range sorted {
		typ := nodeMap[id]

		if typ == "pkg" {
			p := pkgIndex[id]
			// Check if batchable
			if engine.IsBatchable(p) {
				batchBuffer = append(batchBuffer, p)
			} else {
				// Cannot batch this one, flush previous batch first
				flushBatch()
				engine.InstallOne(p)
			}
		} else if typ == "task" {
			// Task execution, flush batch first
			flushBatch()
			t := taskIndex[id]
			executeTask(t, runner, globalVars)
		}
	}
	// Flush remaining
	flushBatch()
}

func executeTask(t config.Task, runner *shell.Runner, globalVars map[string]string) {
	logger.Info("Task: [%s]", t.ID)

	// Prepare Data
	vars := make(map[string]string)
	for k, v := range globalVars {
		vars[k] = v
	}
	for k, v := range t.Vars {
		vars[k] = v
	}

	tplData := map[string]interface{}{
		"vars": vars,
		// task doesn't have "name" context usually, or use t.Name
		"name": t.ID,
	}

	// Render
	checkCmd := pkgmanager.RenderCmd(t.Check, tplData)
	runCmd := pkgmanager.RenderCmd(t.Run, tplData)

	shouldRun := true

	if checkCmd != "" {
		code := runner.ExecSilent(checkCmd)
		action := "run" // default

		if act, ok := t.On[strconv.Itoa(code)]; ok {
			action = act
		} else if act, ok := t.On["fail"]; ok && code != 0 {
			action = act
		} else if act, ok := t.On["success"]; ok && code == 0 {
			action = act
		}

		if action == "skip" {
			shouldRun = false
			logger.Success("  Check passed, skipping.")
		}
	}

	if shouldRun {
		if err := runner.ExecStream(runCmd); err != nil {
			logger.Error("  Task failed: %v", err)
		} else {
			logger.Success("  Task completed.")
		}
	}
}