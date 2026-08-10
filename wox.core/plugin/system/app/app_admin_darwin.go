package app

import "errors"

func openAppAsAdministrator(path string) error {
	return errors.New("opening an app as administrator is not supported on macOS")
}
