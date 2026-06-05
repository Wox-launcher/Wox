package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"wox/common"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

type AttentionActionType string

const (
	AttentionActionTypeChangeQuery AttentionActionType = "change_query"

	attentionReadRetention       = 30 * 24 * time.Hour
	attentionMaxStoredItems      = 500
	attentionDatabaseMaxAttempts = 5
	attentionDatabaseRetryDelay  = 100 * time.Millisecond
)

// PushAttentionRequest is the plugin-facing request for a persistent attention item.
type PushAttentionRequest struct {
	Key         string           `json:"key"`
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	Icon        *common.WoxImage `json:"icon,omitempty"`
	Action      *AttentionAction `json:"action,omitempty"`
}

// AttentionAction describes the user action attached to an attention item.
type AttentionAction struct {
	Type  AttentionActionType `json:"type"`
	Query string              `json:"query,omitempty"`
}

// AttentionPluginSource carries core-resolved plugin identity for a pushed attention item.
type AttentionPluginSource struct {
	PluginID        string
	PluginDirectory string
	DefaultIcon     common.WoxImage
}

type AttentionListResult struct {
	Unread []database.AttentionItem
	Read   []database.AttentionItem
}

type AttentionManager struct {
	db *gorm.DB
}

// NewAttentionManager creates a manager bound to the provided database handle.
func NewAttentionManager(db *gorm.DB) *AttentionManager {
	return &AttentionManager{db: db}
}

// GetAttentionManager returns the default manager backed by the application database.
func GetAttentionManager() *AttentionManager {
	return NewAttentionManager(database.GetDB())
}

// Push creates or updates a persistent attention item for a plugin key.
func (m *AttentionManager) Push(ctx context.Context, source AttentionPluginSource, request PushAttentionRequest) (database.AttentionItem, error) {
	if m == nil || m.db == nil {
		return database.AttentionItem{}, errors.New("attention manager database is not initialized")
	}

	source.PluginID = strings.TrimSpace(source.PluginID)
	request.Key = strings.TrimSpace(request.Key)
	request.Title = strings.TrimSpace(request.Title)
	if source.PluginID == "" {
		return database.AttentionItem{}, errors.New("plugin id is required")
	}
	if request.Key == "" {
		return database.AttentionItem{}, errors.New("attention key is required")
	}
	if request.Title == "" {
		return database.AttentionItem{}, errors.New("attention title is required")
	}

	identityKey := buildAttentionIdentityKey(source.PluginID, request.Key)
	now := util.GetSystemTimestamp()
	fingerprint := attentionContentFingerprint(request.Title, request.Description)
	icon := resolveAttentionIcon(ctx, source, request.Icon)
	action, marshalErr := marshalAttentionAction(request.Action)
	if marshalErr != nil {
		return database.AttentionItem{}, marshalErr
	}

	var saved database.AttentionItem
	err := retryAttentionDatabaseWrite(ctx, func() error {
		return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var existing database.AttentionItem
			findErr := tx.First(&existing, "identity_key = ?", identityKey).Error
			if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}

			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				saved = database.AttentionItem{
					IdentityKey:        identityKey,
					PluginID:           source.PluginID,
					Key:                request.Key,
					Title:              request.Title,
					Description:        request.Description,
					Icon:               icon.String(),
					Action:             action,
					ContentFingerprint: fingerprint,
					IsRead:             false,
					CreatedTimestamp:   now,
					UpdatedTimestamp:   now,
				}
				return tx.Create(&saved).Error
			}

			shouldStayRead := existing.IsRead && existing.ContentFingerprint == fingerprint
			existing.Title = request.Title
			existing.Description = request.Description
			existing.Icon = icon.String()
			existing.Action = action
			existing.ContentFingerprint = fingerprint
			existing.UpdatedTimestamp = now
			if existing.IsRead && !shouldStayRead {
				existing.IsRead = false
				existing.ReadTimestamp = 0
			}

			saved = existing
			return tx.Save(&saved).Error
		})
	})
	if err != nil {
		return database.AttentionItem{}, err
	}

	if cleanupErr := m.Cleanup(ctx); cleanupErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to cleanup attention items: %v", cleanupErr))
	}

	return saved, nil
}

// MarkRead marks an attention item as read when it exists.
func (m *AttentionManager) MarkRead(ctx context.Context, identityKey string) error {
	if m == nil || m.db == nil {
		return errors.New("attention manager database is not initialized")
	}

	identityKey = strings.TrimSpace(identityKey)
	if identityKey == "" {
		return errors.New("attention identity key is required")
	}

	now := util.GetSystemTimestamp()
	return retryAttentionDatabaseWrite(ctx, func() error {
		return m.db.WithContext(ctx).
			Model(&database.AttentionItem{}).
			Where("identity_key = ? AND is_read = ?", identityKey, false).
			Updates(map[string]any{
				"is_read":           true,
				"read_timestamp":    now,
				"updated_timestamp": now,
			}).Error
	})
}

// UnreadCount returns the current number of unread attention items.
func (m *AttentionManager) UnreadCount(ctx context.Context) (int64, error) {
	if m == nil || m.db == nil {
		return 0, errors.New("attention manager database is not initialized")
	}

	var count int64
	if err := m.db.WithContext(ctx).Model(&database.AttentionItem{}).Where("is_read = ?", false).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// List returns unread and read attention items in their display order.
func (m *AttentionManager) List(ctx context.Context) (AttentionListResult, error) {
	if m == nil || m.db == nil {
		return AttentionListResult{}, errors.New("attention manager database is not initialized")
	}

	var unread []database.AttentionItem
	if err := m.db.WithContext(ctx).Where("is_read = ?", false).Order("updated_timestamp DESC").Find(&unread).Error; err != nil {
		return AttentionListResult{}, err
	}

	var read []database.AttentionItem
	if err := m.db.WithContext(ctx).Where("is_read = ?", true).Order("read_timestamp DESC").Limit(attentionMaxStoredItems).Find(&read).Error; err != nil {
		return AttentionListResult{}, err
	}

	return AttentionListResult{Unread: unread, Read: read}, nil
}

// Cleanup removes old read items and caps stored history.
func (m *AttentionManager) Cleanup(ctx context.Context) error {
	if m == nil || m.db == nil {
		return errors.New("attention manager database is not initialized")
	}

	cutoff := util.GetSystemTimestamp() - int64(attentionReadRetention/time.Millisecond)
	if err := m.db.WithContext(ctx).Where("is_read = ? AND read_timestamp > 0 AND read_timestamp < ?", true, cutoff).Delete(&database.AttentionItem{}).Error; err != nil {
		return err
	}

	var total int64
	if err := m.db.WithContext(ctx).Model(&database.AttentionItem{}).Count(&total).Error; err != nil {
		return err
	}
	if total <= attentionMaxStoredItems {
		return nil
	}

	overLimit := int(total - attentionMaxStoredItems)
	var removable []database.AttentionItem
	if err := m.db.WithContext(ctx).Where("is_read = ?", true).Order("read_timestamp ASC").Limit(overLimit).Find(&removable).Error; err != nil {
		return err
	}
	for _, item := range removable {
		if err := m.db.WithContext(ctx).Delete(&database.AttentionItem{}, "identity_key = ?", item.IdentityKey).Error; err != nil {
			return err
		}
	}

	return nil
}

// retryAttentionDatabaseWrite smooths over transient SQLite write locks from concurrent core activity.
func retryAttentionDatabaseWrite(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 0; attempt < attentionDatabaseMaxAttempts; attempt++ {
		err = operation()
		if err == nil || !isAttentionDatabaseLockedError(err) || attempt == attentionDatabaseMaxAttempts-1 {
			return err
		}

		timer := time.NewTimer(attentionDatabaseRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

// isAttentionDatabaseLockedError identifies SQLite lock errors across driver message variants.
func isAttentionDatabaseLockedError(err error) bool {
	if err == nil {
		return false
	}

	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "database is locked") ||
		strings.Contains(errText, "database table is locked") ||
		strings.Contains(errText, "sqlite_busy")
}

func buildAttentionIdentityKey(pluginID string, key string) string {
	return fmt.Sprintf("%s:%s", pluginID, key)
}

func attentionContentFingerprint(title string, description string) string {
	return util.Md5([]byte(title + description))
}

func resolveAttentionIcon(ctx context.Context, source AttentionPluginSource, requestedIcon *common.WoxImage) common.WoxImage {
	icon := source.DefaultIcon
	if requestedIcon != nil && !requestedIcon.IsEmpty() {
		icon = *requestedIcon
	}
	if icon.IsEmpty() {
		icon = common.WoxIcon
	}
	return common.ConvertIcon(ctx, icon, source.PluginDirectory)
}

func marshalAttentionAction(action *AttentionAction) (string, error) {
	if action == nil || action.Type == "" {
		return "", nil
	}
	if action.Type != AttentionActionTypeChangeQuery {
		return "", fmt.Errorf("unsupported attention action type: %s", action.Type)
	}
	if strings.TrimSpace(action.Query) == "" {
		return "", errors.New("change_query attention action requires query")
	}

	actionJSON, err := json.Marshal(action)
	if err != nil {
		return "", err
	}
	return string(actionJSON), nil
}

// ParseAttentionAction decodes a stored attention action payload.
func ParseAttentionAction(raw string) (*AttentionAction, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var action AttentionAction
	if err := json.Unmarshal([]byte(raw), &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// ParseAttentionIcon decodes a stored attention icon and falls back to the Wox icon.
func ParseAttentionIcon(raw string) common.WoxImage {
	icon, err := common.ParseWoxImage(raw)
	if err != nil || icon.IsEmpty() {
		return common.WoxIcon
	}
	return icon
}
