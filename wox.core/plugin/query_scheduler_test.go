package plugin

import "testing"

func TestGetQueryFirstFlushDeadlineMsSkipsAliasRestoreJob(t *testing.T) {
	scheduler := newQueryScheduler()
	deadline := scheduler.getQueryFirstFlushDeadlineMs([]queryPluginJob{{aliasRestore: true}}, 0)
	if deadline != 20 {
		t.Fatalf("deadline = %d, want the no-history first-flush bound", deadline)
	}
}
