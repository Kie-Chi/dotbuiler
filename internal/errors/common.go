package errors

import "fmt"

type SkipError struct {
	Reason string
}

func (e *SkipError) Error() string {
	return e.Reason
}

func NewSkipError(format string, a ...interface{}) error {
	return &SkipError{
		Reason: fmt.Sprintf(format, a...),
	}
}

var ErrSkipped = &SkipError{Reason: "skipped"}