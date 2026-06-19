package cloudsync

import (
	"strings"
	"time"
)

// OplogSyncPolicy describes when a newly written local oplog can be uploaded.
type OplogSyncPolicy struct {
	Delay time.Duration
}

// ResolveOplogSyncPolicy returns the built-in upload policy for a local oplog identity.
func ResolveOplogSyncPolicy(entityType string, entityID string, key string, op string) OplogSyncPolicy {
	if op != OpUpsert {
		return OplogSyncPolicy{}
	}

	if entityType == EntityWoxSetting {
		switch key {
		case "ActionedResults", "QueryHistories":
			return OplogSyncPolicy{Delay: 10 * time.Minute}
		default:
			return OplogSyncPolicy{}
		}
	}

	// Rss reader plugin
	if entityType == EntityPluginSetting && entityID == "9575a1fc-d81b-4947-bcce-bd075f118f3e" {
		if strings.HasPrefix(key, "feedItems") {
			return OplogSyncPolicy{Delay: 30 * time.Minute}
		}
	}

	return OplogSyncPolicy{}
}
