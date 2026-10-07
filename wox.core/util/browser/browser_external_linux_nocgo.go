//go:build linux && !cgo

package browser

import "wox/util/shell"

func openExternalURL(rawURL string, _ uintptr) error { return shell.Open(rawURL) }
