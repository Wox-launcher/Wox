package common

import "context"

// AIChatProject joins synced metadata with this device's directory bindings.
// Directories and PrimaryDirectory must only be persisted in local settings.
type AIChatProject struct {
	Id               string
	Name             string
	CreatedAt        int64
	UpdatedAt        int64
	SelectionCount   int64
	Directories      []string
	PrimaryDirectory string
}

// AIChatProjects is an internal capability, separate from the published plugin API.
type AIChatProjects interface {
	ListChatProjects(context.Context) ([]AIChatProject, error)
	SaveChatProject(context.Context, AIChatProject) (AIChatProject, error)
	DeleteChatProject(context.Context, string) error
	MoveChatToProject(context.Context, string, string) error
	SetDefaultChatProject(context.Context, string) error
}

// AIChatProjectUsage records local project selection frequency.
type AIChatProjectUsage interface {
	RecordProjectSelection(context.Context, string) error
}

// AIChatProjectSkills exposes project-scoped skills without changing the plugin SDK.
type AIChatProjectSkills interface {
	ProjectSkills(context.Context, string) ([]Skill, error)
}

type aiWorkingDirectoryKey struct{}

// WithAIWorkingDirectory scopes relative file paths and commands to one request.
func WithAIWorkingDirectory(ctx context.Context, directory string) context.Context {
	return context.WithValue(ctx, aiWorkingDirectoryKey{}, directory)
}

func AIWorkingDirectory(ctx context.Context) string {
	directory, _ := ctx.Value(aiWorkingDirectoryKey{}).(string)
	return directory
}
