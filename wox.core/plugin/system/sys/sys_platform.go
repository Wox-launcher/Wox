package sys

import "os/exec"

func commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}
