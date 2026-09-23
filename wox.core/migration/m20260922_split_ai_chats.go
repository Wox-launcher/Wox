package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"wox/cloudsync"
	"wox/common"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

const (
	legacyAIChatsSettingKey = "ai_chats"
	aiChatSettingKeyPrefix  = "chat:"
)

func init() { Register(&splitAIChatsMigration{}) }

// splitAIChatsMigration moves the single `ai_chats` JSON array into one `chat:{id}` plugin
// setting per conversation. Independent keys let Cloud Sync merge chats from different
// devices and let the AI Chat plugin load conversation bodies on demand.
type splitAIChatsMigration struct{}

func (m *splitAIChatsMigration) ID() string { return "20260922_split_ai_chats" }

func (m *splitAIChatsMigration) Description() string {
	return "Split the AI Chat history array into one synced plugin setting per chat."
}

// aiChatRecordHeader reads only the identity fields of a chat; the raw JSON is stored as-is so
// the migration does not depend on the full AIChatData shape.
type aiChatRecordHeader struct {
	Id        string
	UpdatedAt int64
}

// Up splits the array and removes the legacy key, including from the cloud. A device that
// still runs the old version loses its cloud copy of the history at that point; this was
// chosen over keeping both layouts alive indefinitely.
func (m *splitAIChatsMigration) Up(ctx context.Context, tx *gorm.DB) error {
	var legacy database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", common.AIChatPluginID, legacyAIChatsSettingKey).First(&legacy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	var chats []json.RawMessage
	if strings.TrimSpace(legacy.Value) != "" {
		if err := json.Unmarshal([]byte(legacy.Value), &chats); err != nil {
			return fmt.Errorf("decode legacy ai_chats: %w", err)
		}
	}
	migrated := 0
	for _, raw := range chats {
		var header aiChatRecordHeader
		if err := json.Unmarshal(raw, &header); err != nil || header.Id == "" {
			continue
		}
		key := aiChatSettingKeyPrefix + header.Id
		// Another device may already have synced this chat in the new layout; keep whichever
		// copy was updated last so migration order between devices does not matter.
		var existing database.PluginSetting
		findErr := tx.Where("plugin_id = ? AND key = ?", common.AIChatPluginID, key).First(&existing).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if findErr == nil {
			var existingHeader aiChatRecordHeader
			if json.Unmarshal([]byte(existing.Value), &existingHeader) == nil && existingHeader.UpdatedAt >= header.UpdatedAt {
				continue
			}
		}
		value := string(raw)
		if err := tx.Save(&database.PluginSetting{PluginID: common.AIChatPluginID, Key: key, Value: value, IsLocal: legacy.IsLocal}).Error; err != nil {
			return err
		}
		if !legacy.IsLocal {
			if err := appendPluginSettingUpsertOplog(tx, common.AIChatPluginID, key, value); err != nil {
				return err
			}
		}
		migrated++
	}

	if err := tx.Delete(&database.PluginSetting{PluginID: common.AIChatPluginID, Key: legacyAIChatsSettingKey}).Error; err != nil {
		return err
	}
	// A queued upload of the old array would only be deleted again by the tombstone below.
	if err := tx.Model(&database.Oplog{}).
		Where("entity_type = ? AND entity_id = ? AND key = ? AND synced_to_cloud = ? AND cloud_sync_discarded = ?", cloudsync.EntityPluginSetting, common.AIChatPluginID, legacyAIChatsSettingKey, false, false).
		Update("cloud_sync_discarded", true).Error; err != nil {
		return err
	}
	if !legacy.IsLocal {
		if err := appendPluginSettingDeleteOplog(tx, common.AIChatPluginID, legacyAIChatsSettingKey); err != nil {
			return err
		}
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("migrated %d AI chats from ai_chats into per-chat settings", migrated))
	return nil
}
