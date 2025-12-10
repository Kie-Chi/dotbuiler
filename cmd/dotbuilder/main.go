package main

import (
	"bytes"
	"dotbuilder/internal/config"
	"dotbuilder/internal/context"
	"dotbuilder/internal/pkgmanager"
	"dotbuilder/internal/taskrunner"
	"dotbuilder/pkg/logger"
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"
	"os"
)

func main() {
	configFile := flag.String("c", "configs/construct.yaml", "Path to configuration file")
	debug := flag.Bool("debug", false, "Enable debug logs")
	var dryRun bool
	flag.BoolVar(&dryRun, "n", false, "Dry-run mode")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry-run mode")
	flag.Parse()

	if *debug { logger.SetDebug(true) }

	logger.Info("Load configuration: %s", *configFile)
	cfg, err := config.Load(*configFile)
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
	}

	// Determine the base directory of the config file
	absConfigPath, err := filepath.Abs(*configFile)
	if err != nil {
		logger.Error("Failed to resolve config path: %v", err)
	}
	baseDir := filepath.Dir(absConfigPath)

	sysInfo := context.Detect()
	isRoot := context.IsRoot()

	if !isRoot && !dryRun {
		logger.Info("Refreshing sudo credentials...")
		if err := exec.Command("sudo", "-v").Run(); err != nil {
			logger.Error("Failed to refresh sudo credentials. Please run as root or ensure sudo works without password.")
		}
		go func() {
			ticker := time.NewTicker(4 * time.Minute)
			for range ticker.C {
				exec.Command("sudo", "-v").Run()
			}
		}()
	}

    logger.Info("Environment: OS=%s, User=%s, Home=%s", sysInfo.OS, sysInfo.User, sysInfo.Home)

	vars := cfg.Vars
	if vars == nil { vars = make(map[string]string) }
	vars["OS"] = sysInfo.OS
	vars["DISTRO"] = sysInfo.Distro

    if _, ok := vars["home"]; !ok {
		vars["home"] = sysInfo.Home
	}
	if _, ok := vars["user"]; !ok {
		vars["user"] = sysInfo.User
	}

	resolveVariables(vars)
	resolvePackageDefs(cfg.Pkgs, vars)

	scriptDir, err := pkgmanager.Prepare(cfg.Scrpits, vars)	
	if err != nil {
		logger.Error("Failed to prepare helper scripts: %v", err)
	}

	// Engine Init
	pmEngine := pkgmanager.NewEngine(sysInfo, vars, isRoot, dryRun)
	pmEngine.RegisterCustomPMs(cfg.Pkgs)

	if scriptDir != "" {
        currentPath := os.Getenv("PATH")
        newPath := scriptDir + string(os.PathListSeparator) + currentPath        
        pmEngine.Runner.Env["PATH"] = newPath
        logger.Debug("Injected scripts to PATH: %s", scriptDir)
    }

	ctx := &taskrunner.Context{
		Shell:      pmEngine.Runner,
		PkgManager: pmEngine,
		Vars:       vars,
		BaseDir:    baseDir, // Pass the config directory
	}

	var nodes []taskrunner.Node

	// 1. Files -> Nodes
	for i, f := range cfg.Files {
        id := f.ID

        if id == "" {
            id = f.Dest
        }

        if id == "" {
            id = fmt.Sprintf("file_%d", i)
        }

		nodes = append(nodes, &taskrunner.FileNode{
			File: f,
			Id:   id,
		})
	}

	// 2. Packages -> Nodes
	for i := range cfg.Pkgs {
		p := &cfg.Pkgs[i]
		nodes = append(nodes, &taskrunner.PkgNode{
			Pkg: p,
			Mgr: pmEngine,
		})
	}

	// 3. Tasks -> Nodes
	for _, t := range cfg.Tasks {
		nodes = append(nodes, &taskrunner.TaskNode{
			Task: t,
		})
	}

	results := taskrunner.RunGeneric(nodes, ctx)
	taskrunner.PrintSummary(results, nodes)
	logger.Success("All build tasks completed")
}

func resolveVariables(vars map[string]string) {
	for pass := 0; pass < 100; pass++ {
		changed := false
		for k, v := range vars {
			if len(v) < 3 { continue }

			if !bytes.Contains([]byte(v), []byte("{{")) {
				continue
			}

			tmpl, err := template.New("var").Parse(v)
			if err != nil { continue }

			var buf bytes.Buffer
			data := map[string]interface{}{"vars": vars}

			if err := tmpl.Execute(&buf, data); err == nil {
				newVal := buf.String()
				if newVal != v {
					vars[k] = newVal
					changed = true
				}
			}
		}
		if !changed { break }
	}
}

func resolvePackageDefs(pkgs []config.Package, vars map[string]string) {
	data := map[string]interface{}{"vars": vars}

	render := func(s string) string {
		if !bytes.Contains([]byte(s), []byte("{{")) { return s }
		tmpl, err := template.New("p").Parse(s)
		if err != nil { return s }
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err == nil {
			return buf.String()
		}
		return s
	}

	for i := range pkgs {
		pkgs[i].Name = render(pkgs[i].Name)
		pkgs[i].Def = render(pkgs[i].Def)

		// Map values
		for k, v := range pkgs[i].Map {
			pkgs[i].Map[k] = render(v)
		}
	}
}
