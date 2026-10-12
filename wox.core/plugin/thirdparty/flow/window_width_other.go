//go:build !windows

package flow

func flowBackdropSupported() bool { return false }

func flowSystemDark() bool { return false }

func flowSystemAccent() (string, bool) { return "", false }

func flowSystemAccentLight1() (string, bool) { return "", false }
