package pkgmanager

import (
	"dotbuilder/pkg/constants"
	"fmt"
	"strings"
)

// --- Resolver ---

func (e *Engine) resolveCheckTpl(pm string) string {
	// 1. Custom defined in config
	if pmDef, ok := e.RegisteredPMs[pm]; ok && pmDef.PmCheckTpl != "" {
		return pmDef.PmCheckTpl
	}
	
	// 2. Constants Struct (Rich definition)
	check, _, _ := constants.GetPMTemplates(pm)
	if check != "" {
		return check
	}

	// 3. Base Map (Simple definition)
	if tpl, ok := constants.BaseCheckTemplates[pm]; ok {
		return tpl
	}
	
	return ""
}

func (e *Engine) resolveInstallTpl(pm string) string {
	// 1. Custom
	if pmDef, ok := e.RegisteredPMs[pm]; ok && pmDef.PmInstallTpl != "" {
		return pmDef.PmInstallTpl
	}

	// 2. Constants Struct
	_, install, _ := constants.GetPMTemplates(pm)
	if install != "" {
		return install
	}

	// 3. Base Map
	if tpl, ok := constants.BaseSingleTemplates[pm]; ok {
		return tpl
	}

	return ""
}

func (e *Engine) resolveBatchTpl(pm string) string {
	
	// 1. Base Map (Batch templates are mostly defined here)
	if tpl, ok := constants.BaseBatchTemplates[pm]; ok {
		return tpl
	}
	
	return ""
}

func (e *Engine) resolveUpdateCmd(pm string) string {
	// 1. Custom
	if pmDef, ok := e.RegisteredPMs[pm]; ok && pmDef.Upd != "" {
		return pmDef.Upd 
	}

	// 2. System Update Map
	if cmd, ok := constants.SystemUpdateCmds[pm]; ok {
		return cmd
	}

	return ""
}

// --- Builder ---

func (e *Engine) BuildCheckCmd(pmName, rawPkgName string) string {
	tpl := e.resolveCheckTpl(pmName)
	if tpl == "" {
		return ""
	}

	// 1. Split
	names := strings.Fields(rawPkgName)
	if len(names) == 0 {
		return "false"
	}

	// 2. Check
	var checks []string
	for _, name := range names {
		data := map[string]interface{}{
			"name": name,
			"vars": e.Vars,
		}
		checks = append(checks, RenderCmd(tpl, data))
	}

	return strings.Join(checks, " && ")
}

func (e *Engine) BuildInstallCmd(pmName, pkgName string) string {
	tpl := e.resolveInstallTpl(pmName)
	
	var cmd string
	data := map[string]interface{}{
		"name": pkgName,
		"vars": e.Vars,
	}

	if tpl != "" {
		cmd = RenderCmd(tpl, data)
	} else {
		// Default Fallback: "apt-get install git"
		cmd = fmt.Sprintf("%s install %s", pmName, pkgName)
	}

	return e.applySudo(pmName, cmd)
}

func (e *Engine) BuildBatchInstallCmd(pmName string, names []string) string {
	if len(names) == 0 {
		return ""
	}

	tpl := e.resolveBatchTpl(pmName)
	
	// Fallback logic
	if tpl == "" {
		tpl = pmName + " install {{.names}}"
	}

	data := map[string]interface{}{
		"names": strings.Join(names, " "),
		"vars":  e.Vars,
	}
	cmd := RenderCmd(tpl, data)
	
	return e.applySudo(pmName, cmd)
}

func (e *Engine) BuildSystemUpdateCmd(pmName string) string {
	tpl := e.resolveUpdateCmd(pmName)
	if tpl == "" {
		return ""
	}

	data := map[string]interface{}{"vars": e.Vars}
	cmd := RenderCmd(tpl, data)

	return e.applySudo(pmName, cmd)
}

// --- Helper ---

func (e *Engine) applySudo(pmName, cmd string) string {
	if constants.PMNeedsSudo[pmName] && !e.IsRoot {
		if !strings.HasPrefix(strings.TrimSpace(cmd), "sudo") {
			return "sudo " + cmd
		}
	}
	return cmd
}
