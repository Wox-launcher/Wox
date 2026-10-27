package ui

import (
	"context"
	"errors"

	"wox/common"
	"wox/plugin"
)

// chatProjectService resolves the feature capability without extending plugin SDK APIs.
func chatProjectService(ctx context.Context) (common.AIChatProjects, error) {
	service, ok := plugin.GetPluginManager().GetAIChatPluginChater(ctx).(common.AIChatProjects)
	if !ok {
		return nil, errors.New("AI chat projects are unavailable")
	}
	return service, nil
}

// ChatProjects loads synced identities joined to this device's directory bindings.
func (s *CoreServices) ChatProjects(ctx context.Context, sessionID string) ([]common.AIChatProject, error) {
	ctx = uiServiceContext(ctx, sessionID)
	service, err := chatProjectService(ctx)
	if err != nil {
		return nil, err
	}
	return service.ListChatProjects(ctx)
}

// SaveChatProject delegates validation and atomic persistence to the feature owner.
func (s *CoreServices) SaveChatProject(ctx context.Context, sessionID string, project common.AIChatProject) (common.AIChatProject, error) {
	ctx = uiServiceContext(ctx, sessionID)
	service, err := chatProjectService(ctx)
	if err != nil {
		return project, err
	}
	return service.SaveChatProject(ctx, project)
}

// DeleteChatProject preserves the project's conversations as ordinary chats.
func (s *CoreServices) DeleteChatProject(ctx context.Context, sessionID, projectID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	service, err := chatProjectService(ctx)
	if err != nil {
		return err
	}
	return service.DeleteChatProject(ctx, projectID)
}

// MoveChatToProject updates membership without replacing conversation content.
func (s *CoreServices) MoveChatToProject(ctx context.Context, sessionID, chatID, projectID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	service, err := chatProjectService(ctx)
	if err != nil {
		return err
	}
	return service.MoveChatToProject(ctx, chatID, projectID)
}

// SetDefaultChatProject saves the project chosen for future new chats.
func (s *CoreServices) SetDefaultChatProject(ctx context.Context, sessionID, projectID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	service, err := chatProjectService(ctx)
	if err != nil {
		return err
	}
	return service.SetDefaultChatProject(ctx, projectID)
}

// ChatProjectSkills discovers local skills without adding them to the global catalog.
func (s *CoreServices) ChatProjectSkills(ctx context.Context, sessionID, projectID string) ([]common.Skill, error) {
	ctx = uiServiceContext(ctx, sessionID)
	service, ok := plugin.GetPluginManager().GetAIChatPluginChater(ctx).(common.AIChatProjectSkills)
	if !ok {
		return nil, errors.New("AI chat project skills are unavailable")
	}
	return service.ProjectSkills(ctx, projectID)
}

// RecordChatProjectSelection persists a project choice as device-local usage.
func (s *CoreServices) RecordChatProjectSelection(ctx context.Context, sessionID, projectID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	service, ok := plugin.GetPluginManager().GetAIChatPluginChater(ctx).(common.AIChatProjectUsage)
	if !ok {
		return errors.New("AI chat project usage is unavailable")
	}
	return service.RecordProjectSelection(ctx, projectID)
}
