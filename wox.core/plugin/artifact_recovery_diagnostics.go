package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"wox/util"
)

type ArtifactRecoveryKind string

const (
	ArtifactRecoveryRollback    ArtifactRecoveryKind = "rolled_back"
	ArtifactRecoveryInterrupted ArtifactRecoveryKind = "interrupted_upgrade"
)

type ArtifactRecoveryEvent struct {
	Kind      ArtifactRecoveryKind `json:"kind"`
	Timestamp int64                `json:"timestamp"`
}

func artifactRecoveryPath(kind ArtifactRecoveryKind) string {
	return filepath.Join(util.GetLocation().GetWoxDataDirectory(), "diagnostics", "plugin-"+string(kind))
}

// recordArtifactRecovery retains one timestamp per outcome without persisting plugin content.
func recordArtifactRecovery(ctx context.Context, kind ArtifactRecoveryKind, tx artifactUpgrade) {
	util.GetLogger().Warn(ctx, fmt.Sprintf("plugin artifact recovery %s plugin=%s oldVersion=%s", kind, tx.PluginID, tx.OldVersion))
	path := artifactRecoveryPath(kind)
	err := os.MkdirAll(filepath.Dir(path), 0700)
	if err == nil {
		err = os.WriteFile(path, []byte(strconv.FormatInt(util.GetSystemTimestamp(), 10)), 0600)
	}
	if err != nil {
		util.GetLogger().Warn(ctx, "save plugin recovery diagnostic: "+err.Error())
	}
}

// ArtifactRecoveryEvents returns the last rollback and last interrupted upgrade, when available.
func ArtifactRecoveryEvents() []ArtifactRecoveryEvent {
	var events []ArtifactRecoveryEvent
	for _, kind := range []ArtifactRecoveryKind{ArtifactRecoveryRollback, ArtifactRecoveryInterrupted} {
		raw, err := os.ReadFile(artifactRecoveryPath(kind))
		if err != nil {
			continue
		}
		if timestamp, err := strconv.ParseInt(string(raw), 10, 64); err == nil && timestamp > 0 {
			events = append(events, ArtifactRecoveryEvent{Kind: kind, Timestamp: timestamp})
		}
	}
	return events
}
