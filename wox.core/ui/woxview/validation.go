package woxview

import "fmt"

// ValidationError identifies the failing tree location without changing existing diagnostics.
// Path uses the node paths in diagnostics (root/column[0]/text), optionally followed
// by a property suffix. Message is the complete diagnostic, including its existing
// path prefix, so WindowResult.ErrMsg remains compatible with earlier callers.
type ValidationError struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// invalid attaches a location while preserving the original error's text formatting.
func invalid(path, format string, args ...any) error {
	return &ValidationError{Path: path, Message: fmt.Errorf(format, args...).Error()}
}
