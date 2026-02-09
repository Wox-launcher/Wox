package explorer

import "sync/atomic"

var monitorLogger atomic.Value // func(msg string)

func setExplorerMonitorLogger(logger func(msg string)) {
	if logger == nil {
		monitorLogger.Store((func(string))(nil))
		return
	}
	monitorLogger.Store(logger)
}

func logFromMonitor(msg string) {
	if v := monitorLogger.Load(); v != nil {
		if fn, ok := v.(func(string)); ok && fn != nil {
			fn(msg)
		}
	}
}
