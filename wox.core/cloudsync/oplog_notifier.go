package cloudsync

import "sync"

var (
	oplogNotifyMu sync.Mutex
	oplogNotifyCh = make(chan struct{}, 1)
)

func NotifyOplogChanged() {
	oplogNotifyMu.Lock()
	defer oplogNotifyMu.Unlock()

	select {
	case oplogNotifyCh <- struct{}{}:
	default:
	}
}

func SubscribeOplogChanges() <-chan struct{} {
	return oplogNotifyCh
}
