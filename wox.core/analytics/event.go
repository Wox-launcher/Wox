package analytics

type EventType string

const (
	EventTypeUIOpened       EventType = "ui.opened"
	EventTypeAppLaunched    EventType = "app.launched"
	EventTypeActionExecuted EventType = "action.executed"
)

type SubjectType string

const (
	SubjectTypeUI     SubjectType = "ui"
	SubjectTypeApp    SubjectType = "app"
	SubjectTypePlugin SubjectType = "plugin"
)

type Event struct {
	ID          uint        `gorm:"primaryKey;autoIncrement"`
	Timestamp   int64       `gorm:"not null;index:idx_event_ts,priority:1;index:idx_event_type_ts,priority:2"`
	EventType   EventType   `gorm:"not null;index:idx_event_type_ts,priority:1;index:idx_event_type_subject,priority:1"`
	SubjectType SubjectType `gorm:"not null;index:idx_event_type_subject,priority:2;index:idx_event_subject,priority:1"`
	SubjectID   string      `gorm:"not null;index:idx_event_type_subject,priority:3;index:idx_event_subject,priority:2"`
	SubjectName string      `gorm:""`
	Meta        string      `gorm:""`
}
