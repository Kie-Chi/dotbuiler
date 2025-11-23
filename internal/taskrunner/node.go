package taskrunner

import (
	"dotbuilder/internal/pkgmanager"
	"dotbuilder/pkg/shell"
)

type Context struct {
	Shell      *shell.Runner
	PkgManager *pkgmanager.Engine
	Vars       map[string]string
	BaseDir    string // Directory of the config file, for relative paths
}

type Node interface {
	ID() string
	Deps() []string
	
	Execute(ctx *Context) error
	BatchGroup() string
}

type BatchableNode interface {
	Node
	GetBatchItem() string
}