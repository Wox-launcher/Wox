//go:build !windows

package util

func SetAppUserModelID(_ string) error { return nil }

