package pkgmanager

import (
	"dotbuilder/internal/config"
	"dotbuilder/internal/context"
	"dotbuilder/pkg/constants"
	"dotbuilder/pkg/logger"
	"dotbuilder/pkg/shell"
	"fmt"
	"strings"
)

// Engine
type Engine struct {
	Sys           *context.SystemInfo
	Vars          map[string]string
	RegisteredPMs map[string]*config.Package
	Runner        *shell.Runner
	IsRoot        bool
	UpdatedPMs    map[string]bool
}

// NewEngine
func NewEngine(sys *context.SystemInfo, vars map[string]string, isRoot bool, dryRun bool) *Engine {
	return &Engine{
		Sys:           sys,
		Vars:          vars,
		RegisteredPMs: make(map[string]*config.Package),
		Runner:        shell.NewRunner(dryRun),
		IsRoot:        isRoot,
		UpdatedPMs:    make(map[string]bool),
	}
}

func (e *Engine) EnsurePMUpdated(pmName string) {
	// 1. 如果已经更新过，直接跳过
	if e.UpdatedPMs[pmName] {
		return
	}

	e.UpdatedPMs[pmName] = true

	var updateCmd string
	
	if customPM, ok := e.RegisteredPMs[pmName]; ok {
		if customPM.Upd != "" {
			tplData := map[string]interface{}{"vars": e.Vars}
			updateCmd = RenderCmd(customPM.Upd, tplData)
		}
	} else {
		if cmd, ok := constants.SystemUpdateCmds[pmName]; ok {
			updateCmd = cmd
			if constants.PMNeedsSudo[pmName] && !e.IsRoot {
				updateCmd = "sudo " + updateCmd
			}
		}
	}

	if updateCmd != "" {
		logger.Info("Updating metadata for PM: %s", pmName)
		if err := e.Runner.ExecStream(updateCmd); err != nil {
			logger.Warn("Failed to update PM %s: %v", pmName, err)
		}
	}
}

func (e *Engine) RegisterCustomPMs(pkgs []config.Package) {
	for i := range pkgs {
		p := &pkgs[i]
		if p.PmInstallTpl != "" {
			logger.Debug("Register custom PM: %s", p.Name)
			e.RegisteredPMs[p.Name] = p
		}
	}
}

func (e *Engine) IsBatchable(p *config.Package) bool {
	if p.Pre != "" || p.Post != "" || p.Exec != "" || p.Check != "" {
		return false
	}
	mgr := p.GetManager()
	if mgr != "" && mgr != e.Sys.BasePM {
		return false
	}
	return true
}

func (e *Engine) InstallBaseBatch(pkgs []*config.Package) {
	if e.Sys.BasePM == "unknown" {
		logger.Warn("Base PM unknown, skipping batch install")
		return
	}
	if len(pkgs) == 0 {
		return
	}

	e.EnsurePMUpdated(e.Sys.BasePM)
	var names []string
	for _, p := range pkgs {
		real := p.Def
		if val, ok := p.Map[e.Sys.Distro]; ok {
			real = val
		}
		if real == "" {
			real = p.Name
		}
		names = append(names, real)
	}

	logger.Info("Batch installing: %v", names)
	cmd := constants.BuildBatchInstallCmd(e.Sys.BasePM, names, e.IsRoot)

	if err := e.Runner.ExecStream(cmd); err != nil {
		logger.Error("Batch install failed: %v", err)
	}
}

func (e *Engine) InstallOne(p *config.Package) {
	managers := strings.Split(p.GetManager(), ";")
	if len(managers) == 0 || managers[0] == "" {
		managers = []string{""}
	}

	var lastErr error
	success := false

	for _, pm := range managers {
		if err := e.tryInstall(p, pm); err == nil {
			success = true
			break
		} else {
			lastErr = err
			logger.Debug("PM '%s' failed for '%s': %v", pm, p.Name, err)
		}
	}

	if !success {
		if p.Ignore {
			logger.Warn("Failed to install '%s', ignoring (ignore=true).", p.Name)
		} else {
			logger.Error("Failed to install '%s': %v", p.Name, lastErr)
		}
	}
}

func (e *Engine) tryInstall(p *config.Package, pm string) error {
	realPM := pm
	if realPM == "apt" && e.Sys.BasePM == "apt-get" {
		realPM = "apt-get"
	}

	targetPM := realPM
	if targetPM == "" {
		targetPM = e.Sys.BasePM
	}
	
	displayPM := targetPM
	if displayPM == "" {
		displayPM = "System"
	}
	logger.Debug("Package: %s (PM: %s)", p.Name, displayPM)

	tplData := map[string]interface{}{
		"vars": e.Vars,
		"name": p.Name,
		"os":   e.Sys.OS,
	}

	// 1. Pre Hook
	if p.Pre != "" {
		if err := e.Runner.ExecStream(RenderCmd(p.Pre, tplData)); err != nil {
			return err
		}
	}

	// 2. Check
	installed := false
	if p.Check != "" {
		if e.Runner.ExecSilent(RenderCmd(p.Check, tplData)) == 0 {
			installed = true
		}
	} else if pmDef, ok := e.RegisteredPMs[realPM]; ok {
		checkCmd := RenderCmd(pmDef.PmCheckTpl, tplData)
		if e.Runner.ExecSilent(checkCmd) == 0 {
			installed = true
		}
	} else {
		checkTpl, _, _ := constants.GetPMTemplates(realPM)
		if checkTpl != "" {
			cmd := RenderCmd(checkTpl, tplData)
			if e.Runner.ExecSilent(cmd) == 0 {
				installed = true
			}
		}
	}

	if installed {
		logger.Success("  [%s] Already installed.", p.Name)
		return nil
	}

	e.EnsurePMUpdated(targetPM)

	// 3. Install
	var installCmd string
	if p.Exec != "" {
		installCmd = RenderCmd(p.Exec, tplData)
	} else if pmDef, ok := e.RegisteredPMs[realPM]; ok {
		installCmd = RenderCmd(pmDef.PmInstallTpl, tplData)
	} else {
		_, installTpl, _ := constants.GetPMTemplates(realPM)
		if installTpl != "" {
			installCmd = RenderCmd(installTpl, tplData)
		} else {
			// Fallback to system PM
			if realPM == "" || realPM == e.Sys.BasePM {
				real := p.Def
				if val, ok := p.Map[e.Sys.Distro]; ok {
					real = val
				}
				if real == "" {
					real = p.Name
				}
				installCmd = constants.BuildSingleInstallCmd(e.Sys.BasePM, real, e.IsRoot)
			} else {
				return fmt.Errorf("unknown PM: %s", realPM)
			}
		}
	}

	if err := e.Runner.ExecStream(installCmd); err != nil {
		return err
	}

	// 4. Post Hook
	if p.Post != "" {
		if err := e.Runner.ExecStream(RenderCmd(p.Post, tplData)); err != nil {
			return err
		}
	}

	return nil
}