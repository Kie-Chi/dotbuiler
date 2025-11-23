package main

import (
	"bytes"
	"dotbuilder/internal/config"
	"dotbuilder/internal/context"
	"dotbuilder/internal/filemanager"
	"dotbuilder/internal/pkgmanager"
	"dotbuilder/internal/taskrunner"
	"dotbuilder/pkg/logger"
	"flag"
	"text/template"
)

func main() {
	configFile := flag.String("c", "configs/construct.yaml", "Path to configuration file")
	debug := flag.Bool("debug", false, "Enable debug logs")
	dryRun := flag.Bool("n", false, "Dry-run mode (simulate only)")
	flag.BoolVar(dryRun, "dry-run", false, "Dry-run mode (simulate only)")
	flag.Parse()

	if *debug {
		logger.SetDebug(true)
	}

	if *dryRun {
		logger.Warn("!!! RUNNING IN DRY-RUN MODE !!!")
		logger.Warn("No changes will be applied to the system.")
	}

	logger.Info("Load configuration: %s", *configFile)
	cfg, err := config.Load(*configFile)
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
	}
	logger.Info("Build target: %s (v%s)", cfg.Meta.Name, cfg.Meta.Ver)

	sysInfo := context.Detect()
	isRoot := context.IsRoot()
	logger.Info("Environment: OS=%s, Distro=%s, BasePM=%s, Root=%v", sysInfo.OS, sysInfo.Distro, sysInfo.BasePM, isRoot)

	vars := cfg.Vars
	if vars == nil {
		vars = make(map[string]string)
	}
	vars["OS"] = sysInfo.OS
	vars["DISTRO"] = sysInfo.Distro

	resolveVariables(vars)

	// 1. Files
	filemanager.ProcessFiles(cfg.Files, vars, *dryRun)

	// 2. Engine
	pmEngine := pkgmanager.NewEngine(sysInfo, vars, isRoot, *dryRun)
	pmEngine.RegisterCustomPMs(cfg.Pkgs)

	// 3. Tasks & Packages
	taskrunner.RunUnified(cfg.Pkgs, cfg.Tasks, pmEngine, vars)

	logger.Success("All build tasks completed")
}

func resolveVariables(vars map[string]string) {
	for pass := 0; pass < 3; pass++ {
		changed := false
		for k, v := range vars {
			if len(v) < 3 { continue }
			
			tmpl, err := template.New("var").Parse(v)
			if err != nil { continue }

			var buf bytes.Buffer
			data := map[string]interface{}{"vars": vars} // Standard format
			if err := tmpl.Execute(&buf, data); err == nil {
				newVal := buf.String()
				if newVal != v {
					vars[k] = newVal
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
}