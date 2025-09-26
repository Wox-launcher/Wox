package database

import (
    "context"
    "time"
    "wox/util"
)

// ToolbarMute stores snooze/mute state for toolbar messages in backend DB.
// Key is MD5(message text) to avoid large keys and keep it stable across locales.
// Until is a unix epoch in milliseconds; 0 means forever.
type ToolbarMute struct {
    Key   string `gorm:"primaryKey"`
    Until int64  // epoch millis; 0 => forever
}

// IsToolbarTextMuted returns true if the given text is currently muted.
func IsToolbarTextMuted(ctx context.Context, text string) bool {
    if text == "" {
        return false
    }
    key := util.Md5([]byte(text))
    var rec ToolbarMute
    err := GetDB().First(&rec, "key = ?", key).Error
    if err != nil {
        return false
    }
    if rec.Until == 0 {
        return true
    }
    now := time.Now().UnixMilli()
    return now < rec.Until
}

// SnoozeToolbarText mutes the given text until the provided time.
// If untilMillis is 0, it means forever.
func SnoozeToolbarText(ctx context.Context, text string, untilMillis int64) error {
    if text == "" {
        return nil
    }
    key := util.Md5([]byte(text))
    rec := ToolbarMute{Key: key, Until: untilMillis}
    return GetDB().Save(&rec).Error
}

// UnmuteToolbarText removes any mute entry for the given text.
func UnmuteToolbarText(ctx context.Context, text string) error {
    if text == "" {
        return nil
    }
    key := util.Md5([]byte(text))
    return GetDB().Where("key = ?", key).Delete(&ToolbarMute{}).Error
}

