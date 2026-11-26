package window

import "sync/atomic"

var systemWakeHandler atomic.Pointer[func()]

func SetSystemWakeHandler(handler func()) { systemWakeHandler.Store(&handler) }
