package pkgmanager

import (
	"errors"
)

var ErrSkipped = errors.New("already installed/configured")