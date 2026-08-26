package contract

import (
	"context"

	"wox/common"
	"wox/plugin"
	"wox/plugin/system/shell/terminal"
)

// TooltipOptions describes one native tooltip anchored in screen coordinates.
type TooltipOptions struct {
	Name         string
	Text         string
	Side         string
	AnchorX      float64
	AnchorY      float64
	AnchorWidth  float64
	AnchorHeight float64
	OwnerX       float64
	OwnerY       float64
	OwnerWidth   float64
	OwnerHeight  float64
	// IgnoreOwnerLeave keeps a synthetic hover visible when the OS cursor is
	// idle on the owner window but never entered the trigger.
	IgnoreOwnerLeave bool
}

// InteractionServices owns launcher actions that are not part of query execution.
type InteractionServices interface {
	QueryBoxFocused(ctx context.Context, sessionID string) error
	ExecuteToolbarMessageAction(ctx context.Context, sessionID string, toolbarMessageID string, actionID string) error
	SubscribeTerminal(ctx context.Context, uiSessionID string, terminalSessionID string, cursor int64) (terminal.SessionState, error)
	UnsubscribeTerminal(ctx context.Context, uiSessionID string, terminalSessionID string) error
	ShowTooltip(ctx context.Context, sessionID string, options TooltipOptions) error
	HideTooltip(ctx context.Context, sessionID string, name string) error
	GlanceItems(ctx context.Context, sessionID string, keys []plugin.GlanceKey, reason plugin.GlanceRefreshReason) ([]plugin.GlanceItemUI, error)
	ExecuteGlanceAction(ctx context.Context, sessionID string, pluginID string, glanceID string, actionID string) error
	LoadLazyResultImage(ctx context.Context, sessionID string, token string) (common.WoxImage, error)
	ResolveImage(ctx context.Context, sessionID string, image common.WoxImage, size int) (common.WoxImage, error)
	ResultPreview(ctx context.Context, sessionID string, querySessionID string, queryID string, resultID string) (plugin.WoxPreview, error)
	ShowPreviewImage(ctx context.Context, sessionID string, image common.WoxImage) error
	Chat(ctx context.Context, sessionID string, chat common.AIChatData) error
	ChatByID(ctx context.Context, sessionID string, chatID string) (common.AIChatData, error)
	DefaultChatModel(ctx context.Context, sessionID string) (common.Model, error)
	SetDefaultChatModel(ctx context.Context, sessionID string, model common.Model) error
	DeleteChat(ctx context.Context, sessionID string, chatID string) error
	StopChat(ctx context.Context, sessionID string, chatID string) (bool, error)
	AnswerAIQuestion(ctx context.Context, sessionID string, questionID string, answer string) error
}
