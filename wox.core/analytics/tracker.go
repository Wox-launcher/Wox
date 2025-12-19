package analytics

import (
	"context"
	"errors"
	"fmt"
	"wox/util"

	"gorm.io/gorm"
)

var dbInstance *gorm.DB

func Init(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("analytics init failed: db is nil")
	}

	dbInstance = db
	return nil
}

func TrackUIOpened(ctx context.Context) {
	track(ctx, Event{
		Timestamp:   util.GetSystemTimestamp(),
		EventType:   EventTypeUIOpened,
		SubjectType: SubjectTypeUI,
		SubjectID:   "main",
		SubjectName: "Wox",
	})
}

func TrackAppLaunched(ctx context.Context, appID string, appName string) {
	track(ctx, Event{
		Timestamp:   util.GetSystemTimestamp(),
		EventType:   EventTypeAppLaunched,
		SubjectType: SubjectTypeApp,
		SubjectID:   appID,
		SubjectName: appName,
	})
}

func TrackActionExecuted(ctx context.Context, pluginID string, pluginName string) {
	track(ctx, Event{
		Timestamp:   util.GetSystemTimestamp(),
		EventType:   EventTypeActionExecuted,
		SubjectType: SubjectTypePlugin,
		SubjectID:   pluginID,
		SubjectName: pluginName,
	})
}

func track(ctx context.Context, e Event) {
	if dbInstance == nil {
		return
	}

	util.Go(ctx, "analytics track", func() {
		insert(ctx, e)
	})
}

func insert(ctx context.Context, e Event) {
	if err := dbInstance.Create(&e).Error; err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("analytics insert failed: %v", err))
	}
}
