package system

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"wox/account"
	"wox/cloudsync"
)

func TestCloudSyncPendingTail(t *testing.T) {
	p := &CloudSyncPlugin{}
	ctx := context.Background()
	accountStatus := account.Status{SyncEnabled: true, Plan: "pro"}
	status := cloudsync.ServiceStatus{PendingCount: 0}
	if subtitle := p.statusSubtitle(ctx, accountStatus, status); subtitle != "" {
		t.Fatalf("normal status subtitle = %q, want empty", subtitle)
	}
	for _, tc := range []struct {
		pending   int
		error     string
		statusKey string
	}{
		{0, "", "plugin_cloudsync_tail_active"},
		{3, "request failed", "plugin_cloudsync_tail_error"},
	} {
		status.PendingCount = tc.pending
		status.State = &cloudsync.CloudSyncStateView{LastError: tc.error}
		tails := p.statusTails(ctx, accountStatus, status)
		wantPending := fmt.Sprintf("%s %d", p.tr(ctx, "plugin_cloudsync_label_pending"), tc.pending)
		if len(tails) != 2 || tails[0].Text != wantPending || tails[1].Text != p.tr(ctx, tc.statusKey) {
			t.Fatalf("status tails = %+v", tails)
		}
		if tc.error != "" && !strings.Contains(p.statusSubtitle(ctx, accountStatus, status), tc.error) {
			t.Fatal("moving pending count lost the error subtitle")
		}
	}
}

func TestCloudSyncHistoryDetailSize(t *testing.T) {
	p := &CloudSyncPlugin{}
	ctx := context.Background()
	for _, tc := range []struct {
		size int
		want string
	}{
		{0, ""},
		{123, "123 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
	} {
		subtitle := p.historyDetailSubtitle(ctx, cloudsync.CloudSyncHistoryRecordDetail{EntityType: cloudsync.EntityWoxSetting, SizeBytes: tc.size})
		label := p.tr(ctx, "plugin_cloudsync_history_label_size") + ": "
		if tc.size == 0 {
			if strings.Contains(subtitle, label) {
				t.Fatalf("legacy detail displays a size: %s", subtitle)
			}
		} else if !strings.Contains(subtitle, label+tc.want) {
			t.Fatalf("subtitle = %q, want %q", subtitle, label+tc.want)
		}
	}
}

func TestCloudSyncHistorySummarySize(t *testing.T) {
	p := &CloudSyncPlugin{}
	ctx := context.Background()
	record := cloudsync.CloudSyncHistoryRecord{
		Operation:    cloudsync.CloudSyncProgressOperationPull,
		Status:       cloudsync.CloudSyncHistoryStatusSucceeded,
		Reason:       "periodic-pull",
		ItemCount:    2,
		DurationMs:   4700,
		EntityCounts: map[string]int{cloudsync.EntityWoxSetting: 2},
		Details:      []cloudsync.CloudSyncHistoryRecordDetail{{SizeBytes: 1024}, {SizeBytes: 512}},
	}
	wantTitle := p.tr(ctx, "plugin_cloudsync_history_pull_succeeded")
	if got := p.historyTitle(ctx, record); got != wantTitle {
		t.Fatalf("title = %q, want %q", got, wantTitle)
	}
	wantTails := []string{
		"1.5 KB", "4.7s", p.formatTimestamp(ctx, historyTimestamp(record)),
		p.tr(ctx, "plugin_cloudsync_history_tail_succeeded"),
	}
	tails := p.historyTails(ctx, record)
	if len(tails) != len(wantTails) {
		t.Fatalf("tails = %+v, want %v", tails, wantTails)
	}
	for i, want := range wantTails {
		if tails[i].Text != want {
			t.Fatalf("tail[%d] = %q, want %q", i, tails[i].Text, want)
		}
	}
	wantSubtitle := strings.Join([]string{
		p.labelValue(ctx, "plugin_cloudsync_history_label_source", p.historyReasonLabel(ctx, record.Reason)),
		p.labelValue(ctx, "plugin_cloudsync_history_label_types", p.formatHistoryEntityCounts(ctx, record.EntityCounts)),
	}, " | ")
	if got := p.historySubtitle(ctx, record); got != wantSubtitle {
		t.Fatalf("subtitle = %q, want %q", got, wantSubtitle)
	}
	for _, details := range [][]cloudsync.CloudSyncHistoryRecordDetail{
		nil, {{SizeBytes: 1024}}, {{SizeBytes: 1024}, {}},
	} {
		record.Details = details
		for _, tail := range p.historyTails(ctx, record) {
			if strings.Contains(tail.Text, " KB") {
				t.Fatalf("incomplete history displays a total size: %q", tail.Text)
			}
		}
	}
	record.Status = cloudsync.CloudSyncHistoryStatusFailed
	record.Error = "request failed"
	if got := p.historySubtitle(ctx, record); !strings.Contains(got, "request failed") {
		t.Fatalf("failure subtitle lost its error: %q", got)
	}
}
