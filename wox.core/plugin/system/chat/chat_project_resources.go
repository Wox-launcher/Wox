package chat

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"wox/ai"
	"wox/common"
	"wox/util"
)

const projectInstructionsMaxBytes = 32 * 1024

type projectInstructionsContextKey struct{}

// prepareProjectResources snapshots project instructions and skills after the membership lock is released.
func (r *AIChatPlugin) prepareProjectResources(ctx context.Context) context.Context {
	project, ok := ctx.Value(chatProjectContextKey{}).(common.AIChatProject)
	if !ok {
		return ctx
	}
	directories := projectSkillDirectories(project)
	skills, err := ai.DiscoverProjectSkills(directories)
	if err != nil {
		util.GetLogger().Warn(ctx, "AI: discover project skills: "+err.Error())
	}
	ctx = ai.WithProjectSkills(ctx, skills)
	var instructions strings.Builder
	for _, directory := range directories {
		if strings.TrimSpace(directory) == "" {
			continue
		}
		path := filepath.Join(directory, "AGENTS.md")
		file, err := os.Open(path)
		if os.IsNotExist(err) {
			path = filepath.Join(directory, "agents.md")
			file, err = os.Open(path)
		}
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			util.GetLogger().Warn(ctx, "AI: open project instructions: "+err.Error())
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(file, projectInstructionsMaxBytes+1))
		_ = file.Close()
		if readErr != nil {
			util.GetLogger().Warn(ctx, "AI: read project instructions: "+readErr.Error())
			continue
		}
		truncated := len(data) > projectInstructionsMaxBytes
		if truncated {
			data = data[:projectInstructionsMaxBytes]
		}
		fmt.Fprintf(&instructions, "\nProject instructions for directory %q, from %q:\n%s\n", directory, path, strings.ToValidUTF8(string(data), "�"))
		if truncated {
			instructions.WriteString("[Instructions truncated at 32 KiB; read the file for the remainder.]\n")
		}
	}
	return context.WithValue(ctx, projectInstructionsContextKey{}, instructions.String())
}
