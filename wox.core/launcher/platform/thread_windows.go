//go:build windows

package platform

import "runtime"

func runtimeLockOSThread() {
	runtime.LockOSThread()
}

func runtimeUnlockOSThread() {
	runtime.UnlockOSThread()
}
