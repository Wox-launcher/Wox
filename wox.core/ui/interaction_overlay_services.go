package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"

	aitool "wox/ai/builtintool/wox"
	"wox/common"
	"wox/plugin"
	"wox/ui/contract"
	"wox/util/emojiimage"
	"wox/util/overlay"
	"wox/util/overlay/imageoverlay"
	"wox/util/tooltip"

	"github.com/disintegration/imaging"
)

// ShowTooltip presents one native tooltip anchored in screen coordinates.
func (s *CoreServices) ShowTooltip(ctx context.Context, sessionID string, options contract.TooltipOptions) error {
	options.Name = strings.TrimSpace(options.Name)
	options.Text = strings.TrimSpace(options.Text)
	if options.Name == "" || options.Text == "" {
		return errors.New("tooltip name and text are required")
	}
	tooltip.Show(uiServiceContext(ctx, sessionID), tooltip.Options{
		Name: options.Name, Text: options.Text, Side: options.Side,
		AnchorX: options.AnchorX, AnchorY: options.AnchorY, AnchorWidth: options.AnchorWidth, AnchorHeight: options.AnchorHeight,
	})
	return nil
}

// HideTooltip closes one native tooltip by name.
func (s *CoreServices) HideTooltip(_ context.Context, _ string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("tooltip name is required")
	}
	tooltip.Close(name)
	return nil
}

// GlanceItems refreshes selected plugin glance items.
func (s *CoreServices) GlanceItems(ctx context.Context, sessionID string, keys []plugin.GlanceKey, reason plugin.GlanceRefreshReason) ([]plugin.GlanceItemUI, error) {
	return plugin.GetPluginManager().GetGlanceItems(uiServiceContext(ctx, sessionID), keys, reason), nil
}

// ExecuteGlanceAction executes one selected plugin glance action.
func (s *CoreServices) ExecuteGlanceAction(ctx context.Context, sessionID string, pluginID string, glanceID string, actionID string) error {
	if pluginID == "" || glanceID == "" || actionID == "" {
		return errors.New("pluginId, glanceId and actionId are required")
	}
	return plugin.GetPluginManager().ExecuteGlanceAction(uiServiceContext(ctx, sessionID), pluginID, glanceID, actionID)
}

// LoadLazyResultImage resolves one manager-issued lazy result image token.
func (s *CoreServices) LoadLazyResultImage(ctx context.Context, sessionID string, token string) (common.WoxImage, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return common.WoxImage{}, errors.New("token is empty")
	}
	return plugin.GetPluginManager().LoadLazyResultIcon(uiServiceContext(ctx, sessionID), token)
}

// ResolveImage converts core-owned URL, emoji, and file-icon sources into raster payloads.
func (s *CoreServices) ResolveImage(ctx context.Context, sessionID string, source common.WoxImage, size int) (common.WoxImage, error) {
	ctx = uiServiceContext(ctx, sessionID)
	if source.IsEmpty() {
		return common.WoxImage{}, errors.New("image is empty")
	}
	if size <= 0 {
		size = 128
	}
	size = min(max(size, 16), 2048)
	if source.ImageType == common.WoxImageTypeFileIcon {
		resolved := common.ConvertFileIconToAbsolutePathWithSize(ctx, source, size)
		if resolved.ImageType == common.WoxImageTypeFileIcon || resolved.IsEmpty() {
			return common.WoxImage{}, errors.New("failed to resolve file icon")
		}
		return resolved, nil
	}
	if source.ImageType != common.WoxImageTypeUrl && source.ImageType != common.WoxImageTypeEmoji {
		return common.WoxImage{}, fmt.Errorf("image type %s does not require core resolution", source.ImageType)
	}
	var decoded image.Image
	var err error
	if source.ImageType == common.WoxImageTypeEmoji {
		// Flutter renders emoji with the platform color font. Reuse that font here so glyph coverage and visual metrics match without a network fetch.
		decoded, err = emojiimage.Render(source.ImageData, size)
	}
	if decoded == nil {
		decoded, err = source.ToImageWithContext(ctx)
	}
	if err != nil {
		return common.WoxImage{}, err
	}
	if decoded.Bounds().Dx() > size || decoded.Bounds().Dy() > size {
		decoded = imaging.Fit(decoded, size, size, imaging.Lanczos)
	}
	return common.NewWoxImage(decoded)
}

// ResultPreview resolves one deferred plugin preview.
func (s *CoreServices) ResultPreview(ctx context.Context, sessionID string, querySessionID string, queryID string, resultID string) (plugin.WoxPreview, error) {
	if querySessionID == "" || queryID == "" || resultID == "" {
		return plugin.WoxPreview{}, errors.New("sessionId, queryId and id are required")
	}
	return plugin.GetPluginManager().GetResultPreview(uiServiceContext(ctx, sessionID), querySessionID, queryID, resultID)
}

// ShowPreviewImage opens one full-size image in the native overlay.
func (s *CoreServices) ShowPreviewImage(ctx context.Context, sessionID string, image common.WoxImage) error {
	if image.IsEmpty() {
		return errors.New("preview image is empty")
	}
	return imageoverlay.Show(uiServiceContext(ctx, sessionID), imageoverlay.Options{
		Image: image, FitToScreen: true, Topmost: true, Movable: true, CloseOnEscape: true, Anchor: overlay.AnchorCenter,
	})
}

// Chat starts or continues one AI chat using the active chat plugin.
func (s *CoreServices) Chat(ctx context.Context, sessionID string, chat common.AIChatData) error {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return errors.New("ai chat plugin not found")
	}
	chater.Chat(ctx, chat, 0)
	return nil
}

// ChatByID returns the full chat data for one lightweight summary.
func (s *CoreServices) ChatByID(ctx context.Context, sessionID string, chatID string) (common.AIChatData, error) {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return common.AIChatData{}, errors.New("ai chat plugin not found")
	}
	chat, ok := chater.GetChat(ctx, chatID)
	if !ok {
		return common.AIChatData{}, errors.New("chat not found")
	}
	return chat, nil
}

// DefaultChatModel returns the model selected by the active chat plugin.
func (s *CoreServices) DefaultChatModel(ctx context.Context, sessionID string) (common.Model, error) {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return common.Model{}, errors.New("ai chat plugin not found")
	}
	return chater.GetDefaultModel(ctx), nil
}

// SetDefaultChatModel persists the chat UI model selection for the next new chat draft.
func (s *CoreServices) SetDefaultChatModel(ctx context.Context, sessionID string, model common.Model) error {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return errors.New("ai chat plugin not found")
	}
	chater.SetDefaultModel(ctx, model)
	return nil
}

// DeleteChat removes one persisted chat.
func (s *CoreServices) DeleteChat(ctx context.Context, sessionID string, chatID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return errors.New("ai chat plugin not found")
	}
	if !chater.DeleteChat(ctx, chatID) {
		return errors.New("chat not found")
	}
	return nil
}

// StopChat cancels one active streaming chat.
func (s *CoreServices) StopChat(ctx context.Context, sessionID string, chatID string) (bool, error) {
	ctx = uiServiceContext(ctx, sessionID)
	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		return false, errors.New("ai chat plugin not found")
	}
	return chater.StopChat(ctx, chatID), nil
}

// AnswerAIQuestion resolves one pending tool question.
func (s *CoreServices) AnswerAIQuestion(ctx context.Context, sessionID string, questionID string, answer string) error {
	if questionID == "" {
		return errors.New("questionId is required")
	}
	ctx = uiServiceContext(ctx, sessionID)
	logger.Info(ctx, fmt.Sprintf("AI: resolving question answer for questionId=%s", questionID))
	aitool.ResolveAIQuestionAnswer(questionID, answer)
	return nil
}
