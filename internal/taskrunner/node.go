package taskrunner

import (
	"dotbuilder/internal/pkgmanager"
	"dotbuilder/pkg/shell"
	"time"
)

type NodeStatus int

const (
	StatusPending     NodeStatus = iota
	StatusSuccess                // Success
	StatusFailed                 // Fail for Running
	StatusSkipped                // Fail for Check
	StatusBlocked              // Fail for Dependencies
)

func (s NodeStatus) String() string {
	switch s {
	case StatusSuccess:
		return "SUCCESS"
	case StatusFailed:
		return "FAILED"
	case StatusSkipped:
		return "SKIPPED"
	case StatusBlocked:
		return "BLOCKED"
	default:
		return "PENDING"
	}
}

type NodeResult struct {
	ID        string
	Status    NodeStatus
	Error     error
	Duration  time.Duration
	Timestamp time.Time
}

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
	Group() string
}

type BatchableNode interface {
	Node
	GetBatchItem() string
}