package supervisor

import "os"

// ArgDebugMode enables diagnostics for this launch session, never a synced preference.
const ArgDebugMode = "--debug-mode"

func DebugModeEnabled() bool {
	return hasArg(os.Args, ArgDebugMode)
}
