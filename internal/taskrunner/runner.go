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
	"text/template"
	"time"
	commone "dotbuilder/internal/errors"
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
			return commone.NewSkipError("Check passed")
		}
	}

	if shouldRun {
		runCmd := pkgmanager.RenderCmd(t.Run, tplData)
		logger.InfoTask("Running [%s]", t.ID)
		if err := runner.ExecStream(runCmd, t.ID); err != nil {
			return err
		}
		logger.Success("[%s] Completed.", t.ID)
	}
	return nil
}

func RunGeneric(nodes []Node, ctx *Context) map[string]NodeResult {
	results := &ResultMap{m: make(map[string]NodeResult)}

	// 1. Build DAG
	g := dag.New()
	nodeMap := make(map[string]Node)
	var ids []string

	for _, n := range nodes {
		id := n.ID()
		if _, exists := nodeMap[id]; exists {
			logger.Error("Duplicate node ID detected: '%s'. Node IDs must be unique across all packages, tasks, and file 'id' fields.", id)
			os.Exit(1)
		}
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
					var skipErr *commone.SkipError
					if errors.As(err, &skipErr) {
						status = StatusSkipped
					} else {
						status = StatusFailed
					}
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

				if err != nil {
					var skipErr *commone.SkipError
					if errors.As(err, &skipErr) {
						status = StatusSkipped
					} else {
						status = StatusFailed
					}
				}

				results.Set(node.ID(), NodeResult{
					ID:        node.ID(),
					Status:    status,
					Error:     err,
					Duration:  time.Since(start),
					Timestamp: time.Now(),
				})
			}(n)
		}

		wg.Wait()
	}

	return results.m
}

func RunPhased(nodes []Node, ctx *Context) map[string]NodeResult {
    stages := map[string][]Node{
        "boot":    {},
        "default": {},
        "end":     {},
    }

    for _, n := range nodes {
        g := n.Group()
        if _, ok := stages[g]; !ok {
            g = "default" // default
        }
        stages[g] = append(stages[g], n)
    }

    order := []string{"boot", "default", "end"}
    allResults := make(map[string]NodeResult)
    previousStageFailed := false

    for _, stageName := range order {
        stageNodes := stages[stageName]
        if len(stageNodes) == 0 {
            continue
        }

        logger.Info("=== Entering Stage: %s (%d nodes) ===", stageName, len(stageNodes))
        if previousStageFailed {
            for _, n := range stageNodes {
                allResults[n.ID()] = NodeResult{
                    ID:     n.ID(),
                    Status: StatusBlocked,
                    Error:  fmt.Errorf("blocked by failure in previous stage"),
                }
            }
            continue
        }

        stageResults := RunGeneric(stageNodes, ctx)
        hasFailure := false
        for id, res := range stageResults {
            allResults[id] = res
            if res.Status == StatusFailed {
                hasFailure = true
            }
        }

        if hasFailure {
            logger.Warn("Stage [%s] failed. Blocking subsequent stages.", stageName)
            previousStageFailed = true
        }
    }

    return allResults
}

func truncateString(str string, num int) string {
	if len(str) > num {
		return str[0:num-3] + "..."
	}
	return str
}



func PrintSummary(results map[string]NodeResult, nodes []Node) {
	type rowData struct {
		id       string
		status   string
		duration string
		message  string
		rawStat  NodeStatus
	}

	var rows []rowData
	headers := []string{"ID", "STATUS", "DURATION", "MESSAGE"}
	colWidths := []int{len(headers[0]), len(headers[1]), len(headers[2]), len(headers[3])}

	for _, n := range nodes {
		id := n.ID()
		res, ok := results[id]

		r := rowData{
			id:      id,
			status:  "UNKNOWN",
			duration: "-",
			message:  "",
			rawStat:  StatusPending,
		}

		if ok {
			r.rawStat = res.Status
			r.status = res.Status.String()
			r.duration = res.Duration.Round(time.Millisecond).String()

			if res.Error != nil {
				var skipErr *commone.SkipError
				if errors.As(res.Error, &skipErr) {
					r.message = skipErr.Reason
				} else {
					r.message = truncateString(res.Error.Error(), 40)
				}
			}
            if r.rawStat == StatusSuccess {
                r.message = ""
            }
		}

		if len(r.id) > colWidths[0] { colWidths[0] = len(r.id) }
		if len(r.status) > colWidths[1] { colWidths[1] = len(r.status) }
		if len(r.duration) > colWidths[2] { colWidths[2] = len(r.duration) }
		if len(r.message) > colWidths[3] { colWidths[3] = len(r.message) }

		rows = append(rows, r)
	}

	for i := range colWidths {
		colWidths[i] += 2
	}

	drawSeparator := func(left, mid, right, line string) {
		fmt.Print(left)
		for i, w := range colWidths {
			fmt.Print(strings.Repeat(line, w))
			if i < len(colWidths)-1 {
				fmt.Print(mid)
			}
		}
		fmt.Println(right)
	}

	drawSeparator("┌", "┬", "┐", "─")
	fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │\n",
		colWidths[0]-2, headers[0],
		colWidths[1]-2, headers[1],
		colWidths[2]-2, headers[2],
		colWidths[3]-2, headers[3])
	drawSeparator("├", "┼", "┤", "─")
	for _, row := range rows {
		colorCode := logger.Reset
		switch row.rawStat {
		case StatusSuccess:
			colorCode = logger.Green
		case StatusFailed:
			colorCode = logger.Red
		case StatusBlocked:
			colorCode = logger.Yellow
		case StatusSkipped:
			colorCode = logger.Cyan
		}
		fmt.Printf("│ %-*s │ ", colWidths[0]-2, row.id)

		fmt.Print(colorCode + row.status + logger.Reset)
		padding := colWidths[1] - 2 - len(row.status)
		if padding > 0 {
			fmt.Print(strings.Repeat(" ", padding))
		}

		fmt.Printf(" │ %-*s │ %-*s │\n",
			colWidths[2]-2, row.duration,
			colWidths[3]-2, row.message)
	}
	drawSeparator("└", "┴", "┘", "─")

    var failures []NodeResult
    for _, n := range nodes {
        if res, ok := results[n.ID()]; ok {
            if res.Status == StatusFailed || res.Status == StatusBlocked {
                failures = append(failures, res)
            }
        }
    }

    if len(failures) > 0 {
        fmt.Println("\n=== Failure Details ===")
        for _, f := range failures {
            logger.Error("[%s] Full Error: %v", f.ID, f.Error)
        }
        logger.Error("Build finished with errors.")
    }
}
