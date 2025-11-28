package taskrunner

import (
	"dotbuilder/internal/config"
	"dotbuilder/internal/filemanager"
	"dotbuilder/internal/pkgmanager"
)

// --- Package Node ---
type PkgNode struct {
	Pkg *config.Package
	Mgr *pkgmanager.Engine
}

func (n *PkgNode) ID() string {
    if n.Pkg.Name != "" { return n.Pkg.Name }
    return n.Pkg.Def
}
func (n *PkgNode) Deps() []string { return n.Pkg.Deps }

func (n *PkgNode) BatchGroup() string {
	batchPM := n.Mgr.GetBatchManager(n.Pkg)
	if batchPM != "" {
		return batchPM // e.g., "pip", "npm", "apt-get"
	}
	return ""
}

func (n *PkgNode) GetBatchItem() string {
    return n.Pkg.ResolveName(n.Mgr.Sys)
}


func (n *PkgNode) Execute(ctx *Context) error {
	if err := ctx.PkgManager.InstallOne(n.Pkg); err != nil {
		return err
	}
	return nil
}

// --- Task Node ---
type TaskNode struct {
	Task config.Task
}

func (n *TaskNode) ID() string     { return n.Task.ID }
func (n *TaskNode) Deps() []string { return n.Task.Deps }
func (n *TaskNode) BatchGroup() string { return "" }

func (n *TaskNode) Execute(ctx *Context) error {
	return ExecuteTaskLogic(n.Task, ctx.Shell, ctx.Vars)
}

// --- File Node ---
type FileNode struct {
    File config.File
    Id   string
}

func (n *FileNode) ID() string { return n.Id }
func (n *FileNode) Deps() []string { return n.File.Deps }
func (n *FileNode) BatchGroup() string { return "" }

func (n *FileNode) Execute(ctx *Context) error {
	var fs filemanager.FileSystem
	if ctx.Shell.DryRun {
		fs = filemanager.DryRunFS{}
	} else {
		fs = filemanager.RealFS{}
	}
    filemanager.ProcessSingleFile(n.File, ctx.Vars, fs, ctx.BaseDir, ctx.Shell)
    return nil
}
