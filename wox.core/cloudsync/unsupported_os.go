package cloudsync

import "errors"

// ErrSkippedUnsupportedOS records a platform-filtered installation without treating it as a failure.
var ErrSkippedUnsupportedOS = errors.New("plugin is unsupported on the current OS")
