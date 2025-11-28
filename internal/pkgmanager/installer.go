package pkgmanager

import (
	"dotbuilder/internal/config"
	"dotbuilder/internal/context"
	"dotbuilder/pkg/constants"
	"dotbuilder/pkg/logger"
	"dotbuilder/pkg/shell"
	"fmt"
	"strings"
	"sync"
	"dotbuilder/internal/errors"
)

// Engine
type Engine struct {
	Sys           *context.SystemInfo
	Vars          map[string]string
	RegisteredPMs map[string]*config.Package
	Runner        *shell.Runner
	IsRoot        bool
	mu            sync.Mutex
	UpdatedPMs    map[string]bool
	pmLocks       sync.Map
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

func (e *Engine) acquireLock(pmName string) func() {
	lockKey, ok := constants.PMLockGroups[pmName]

	if !ok {
		return func() {}
	}

	val, _ := e.pmLocks.LoadOrStore(lockKey, &sync.Mutex{})
	mu := val.(*sync.Mutex)

	mu.Lock()
	return func() { mu.Unlock() }
}

func (e *Engine) EnsurePMUpdated(pmName string) {
	e.mu.Lock()
	if e.UpdatedPMs[pmName] {
		e.mu.Unlock()
		return
	}
	e.UpdatedPMs[pmName] = true
	e.mu.Unlock()

	cmd := e.BuildSystemUpdateCmd(pmName)
	if cmd == "" {
		return 
	}

	logger.Info("Updating metadata for PM: %s", pmName)
	unlock := e.acquireLock(pmName)
	defer unlock()

	if err := e.Runner.ExecStream(cmd, pmName); err != nil {
		logger.Warn("Failed to update PM %s: %v", pmName, err)
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

func (e *Engine) GetBatchManager(p *config.Package) string {
	if p.Pre != "" || p.Post != "" || p.Exec != "" || p.Check != "" {
		return ""
	}

	mgr := p.GetManager()

	if mgr == "" || mgr == e.Sys.BasePM {
		if _, ok := constants.BaseBatchTemplates[e.Sys.BasePM]; ok {
			return e.Sys.BasePM
		}
		return ""
	}

	if _, ok := constants.BaseBatchTemplates[mgr]; ok {
		return mgr
	}

	return ""
}

func (e *Engine) IsBatchable(p *config.Package) bool {
	return e.GetBatchManager(p) != ""
}

func (e *Engine) InstallBatch(pmName string, names []string) error {
	if len(names) == 0 {
		return nil
	}

	var toInstall []string
	for _, name := range names {
		checkCmd := e.BuildCheckCmd(pmName, name)
		if checkCmd != "" && e.Runner.ExecSilent(checkCmd) == 0 {
			logger.Debug("[%s] Check passed for '%s'", pmName, name)
			continue
		}
		toInstall = append(toInstall, name)
	}

	if len(toInstall) == 0 {
		logger.Info("[%s] All batch items installed, Skipping.", pmName)
		return errors.NewSkipError("All installed")
	}

	e.EnsurePMUpdated(pmName)

	unlock := e.acquireLock(pmName)
	defer unlock()

	logger.Info("Batch installing [%s]: %v", pmName, names)
	cmd := e.BuildBatchInstallCmd(pmName, toInstall)
	return e.Runner.ExecStream(cmd, fmt.Sprintf("%s-batch", pmName))
}


func (e *Engine) InstallOne(p *config.Package) error {
	managerStr := p.GetManager()
	if managerStr == "" {
		if p.Exec != "" {
			managerStr = "non-pm"
		} else if e.Sys.BasePM != "" && e.Sys.BasePM != "unknown" {
			managerStr = e.Sys.BasePM
		} else {
			logger.Warn("No manager specified for package '%s' and system BasePM is unknown.", p.Name)
			return fmt.Errorf("no manager specified for package '%s'", p.Name)
		}
	}

	managers := strings.Split(managerStr, ";")

	tplData := map[string]interface{}{
		"vars": e.Vars,
		"name": p.Name,
		"os":   e.Sys.OS,
	}

	if p.Pre != "" {
		logger.Debug("Running Pre-Hook for %s", p.Name)
		if err := e.Runner.ExecStream(RenderCmd(p.Pre, tplData), p.Name); err != nil {
			logger.Warn("[%s] Pre-hook failed: %v", p.Name, err)
			return err
		}
	}

	var lastErr error
	installSuccess := false
	alreadyInstalled := false

	for _, pm := range managers {
		pm = strings.TrimSpace(pm)
		if pm == "" { continue }

		skipped, err := e.tryInstallCore(p, pm, tplData)

		if err == nil {
			if skipped {
				alreadyInstalled = true
			} else {
				installSuccess = true
			}
			break
		} else {
			lastErr = err
			logger.Debug("PM '%s' failed for '%s': %v", pm, p.Name, err)
		}
	}

	if alreadyInstalled {
		logger.Success("[%s] Already installed (Checked).", p.Name)
		return errors.NewSkipError("Already installed")
	}

	if !installSuccess {
		if p.Ignore {
			logger.Warn("Failed to install '%s', ignoring (ignore=true).", p.Name)
			return nil
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("installation failed or no valid PM found")
		}
		logger.Warn("Failed to install '%s': %v", p.Name, lastErr)
		return lastErr
	}

	if p.Post != "" {
		logger.Debug("Running Post-Hook for %s", p.Name)
		if err := e.Runner.ExecStream(RenderCmd(p.Post, tplData), p.Name); err != nil {
			logger.Warn("[%s] Post-hook failed: %v", p.Name, err)
			return err
		}
	}

	return nil
}


func (e *Engine) tryInstallCore(p *config.Package, pm string, tplData map[string]interface{}) (bool, error) {
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

	nameForPM := p.ResolveName(e.Sys)
	systemCheckCmd := e.BuildCheckCmd(targetPM, nameForPM)
	if systemCheckCmd == "" {
		systemCheckCmd = "false"
	}

	isInstalled := false

    checkTplData := make(map[string]interface{})
    for k, v := range tplData {
        checkTplData[k] = v
    }

	if p.Check != "" {
        checkTplData["super"] = map[string]interface{}{
			"check": systemCheckCmd,
		}
		
		userCheckCmd := RenderCmd(p.Check, checkTplData)
		if e.Runner.ExecSilent(userCheckCmd) == 0 {
			isInstalled = true
		}
    } else {
        if systemCheckCmd != "false" && e.Runner.ExecSilent(systemCheckCmd) == 0 {
			isInstalled = true
		}
    }

	if isInstalled {
		return true, nil // Skipped, No Error
	}

	logger.Info("Installing %s via %s...", p.Name, displayPM)

    if pm != "none" {
        e.EnsurePMUpdated(targetPM)
    }

	if pm != "none" {
        unlock := e.acquireLock(realPM)
	    defer unlock()
    }

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
		    if realPM == "" || realPM == e.Sys.BasePM {
                nameForInstall := p.ResolveName(e.Sys)
				installCmd = e.BuildInstallCmd(e.Sys.BasePM, nameForInstall)
			} else {
				return false, fmt.Errorf("unknown PM: %s", realPM)
			}
        }
	}

	if err := e.Runner.ExecStream(installCmd, p.Name); err != nil {
		return false, err
	}

	return false, nil // Not skipped (Installed), No Error
}
