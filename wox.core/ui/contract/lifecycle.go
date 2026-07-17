package contract

import "context"

// LifecycleServices exposes core-owned lifecycle behavior to the embedded UI.
type LifecycleServices interface {
	Ready(ctx context.Context, sessionID string) error
	RegisterInstance(ctx context.Context, view View) error
	DestroyInstance(ctx context.Context, sessionID string) error
	Shown(ctx context.Context, sessionID string) error
	Hidden(ctx context.Context, sessionID string) error
	FocusLost(ctx context.Context, sessionID string) error
	SettingViewChanged(ctx context.Context, sessionID string, inSettingView bool) error
}
