package ui

// WindowError describes a native window operation failure.
type WindowError struct {
	Op  string
	Err string
}

func (e *WindowError) Error() string { return e.Op + ": " + e.Err }