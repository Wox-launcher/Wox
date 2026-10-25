//go:build !windows && !darwin

package raycast

func detectInstalledRaycast() (string, string) {
	return "", ""
}
