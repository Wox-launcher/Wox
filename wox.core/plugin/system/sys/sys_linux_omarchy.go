//go:build linux

package sys

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"
)

// appendOmarchyCommands adds the installed Omarchy menu to system search.
// The catalog lives in Omarchy's JSONC files, including the user's extension,
// so a Wox build does not have to track menu changes.
func (r *SysPlugin) appendOmarchyCommands(ctx context.Context) {
	items, err := loadOmarchyMenuItems()
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("omarchy menu commands: %v", err))
		return
	}
	if len(items) == 0 {
		return
	}

	state := newOmarchyWhenState()
	commands := buildOmarchySysCommands(items, state, omarchyCommandIcon(), func(actionCtx context.Context, id string, action string) {
		r.runSystemAction(actionCtx, id, func() (*exec.Cmd, error) {
			// Login shell, matching Omarchy's menu launcher, so actions see
			// the same functions and PATH as `omarchy menu`.
			return shell.Run("bash", "-lc", action)
		})
		if r.omarchyGuards != nil {
			r.omarchyGuards.refreshSoon(actionCtx)
		}
	})
	if len(commands) == 0 {
		return
	}
	r.commands = append(r.commands, commands...)
	r.omarchyGuards = &omarchyGuardCache{
		state:    state,
		evaluate: evaluateOmarchyGuards,
		script:   omarchyGuardScript(items),
	}
	r.omarchyGuards.refreshIfStale(ctx)
	r.registerOmarchyCommands(ctx, commands)
	util.GetLogger().Info(ctx, fmt.Sprintf("loaded %d omarchy menu commands", len(commands)))
}

func (r *SysPlugin) registerOmarchyCommands(ctx context.Context, commands []SysCommand) {
	if r.api == nil {
		return
	}
	metadata := make([]plugin.MetadataCommand, 0, len(commands))
	for _, command := range commands {
		metadata = append(metadata, plugin.MetadataCommand{
			Command:     command.ID,
			Aliases:     command.Aliases,
			Description: common.I18nString(command.Title),
		})
	}
	r.api.RegisterQueryCommands(ctx, metadata)
}

// loadOmarchyMenuItems reads the default menu and overlays the user extension.
// A missing Omarchy install returns no items. A broken user file keeps the defaults.
func loadOmarchyMenuItems() ([]omarchyMenuItem, error) {
	defaultPath := omarchyDefaultMenuPath()
	if defaultPath == "" {
		return nil, nil
	}
	defaultRaw, err := os.ReadFile(defaultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	base, err := parseOmarchyMenuJSONC(string(defaultRaw))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", defaultPath, err)
	}

	var extra []omarchyRawItem
	if userPath := omarchyUserMenuPath(); userPath != "" {
		userRaw, readErr := os.ReadFile(userPath)
		if readErr == nil {
			parsed, parseErr := parseOmarchyMenuJSONC(string(userRaw))
			if parseErr != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("omarchy menu extension %s: %v", userPath, parseErr))
			} else {
				extra = parsed
			}
		}
	}
	return mergeOmarchyMenu(base, extra), nil
}

func omarchyRoot() string {
	if dir := os.Getenv("OMARCHY_PATH"); omarchyDirExists(dir) {
		return dir
	}
	if omarchyDirExists("/usr/share/omarchy") {
		return "/usr/share/omarchy"
	}
	return ""
}

func omarchyDefaultMenuPath() string {
	root := omarchyRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "default", "omarchy", "omarchy-menu.jsonc")
}

func omarchyUserMenuPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "omarchy", "extensions", "omarchy-menu.jsonc")
}

func omarchyCommandIcon() common.WoxImage {
	root := omarchyRoot()
	if root == "" {
		return sysIcon
	}
	path := filepath.Join(root, "icon.png")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return sysIcon
	}
	return common.NewWoxImageAbsolutePath(path)
}

func omarchyDirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// evaluateOmarchyGuards runs every menu condition in one bash process.
func evaluateOmarchyGuards(ctx context.Context, body string) (map[string]bool, error) {
	program := omarchyGuardProgram(body)
	if program == "" {
		return map[string]bool{}, nil
	}
	parent := ctx
	if parent == nil {
		parent = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(parent, omarchyGuardTimeout)
	defer cancel()
	cmd := shell.BuildCommandContext(timeoutCtx, "bash", nil, "-c", program)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseOmarchyGuardOutput(string(out)), nil
}
