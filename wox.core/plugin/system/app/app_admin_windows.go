package app

import "wox/util/shell"

func openAppAsAdministrator(path string) error {
	return shell.OpenAsAdministrator(path)
}
