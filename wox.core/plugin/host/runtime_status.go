package host

import "wox/plugin"

type runtimeExecutableError struct {
	statusCode plugin.RuntimeHostStatusCode
	message    string
	path       string
}

func (e *runtimeExecutableError) Error() string {
	return e.message
}
