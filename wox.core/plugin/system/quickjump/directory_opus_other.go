//go:build !windows

package quickjump

func getDirectoryOpusListerPath(pid int, windowID string) (string, bool) {
	return "", false
}
