//go:build linux

package sys

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"
)

// runOmarchyLockCommand uses Omarchy's lock service and associated lock hooks.
func runOmarchyLockCommand() (*exec.Cmd, error) {
	return shell.Run("omarchy", "system", "lock")
}

// appendOmarchyCommands adds the installed Omarchy menu to system search.
// The catalog lives in Omarchy's JSONC files, including the user's extension,
// so a Wox build does not have to track menu changes.
func (r *SysPlugin) appendOmarchyCommands(ctx context.Context) {
	if omarchyDefaultMenuPath() == "" {
		return
	}
	previous := omarchyMenuFingerprint()
	r.reloadOmarchyCommands(ctx)
	stop := make(chan struct{})
	if r.api != nil {
		r.api.OnUnload(ctx, func(context.Context) { close(stop) })
	}
	// Poll both files because updates may replace either the file or its directory.
	util.Go(ctx, "watch omarchy menu", func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				current := omarchyMenuFingerprint()
				if current != previous {
					r.reloadOmarchyCommands(context.Background())
					previous = current
				}
			}
		}
	})
}

// omarchyMenuFingerprint detects replacements as well as edits to either menu.
func omarchyMenuFingerprint() [32]byte {
	h := sha256.New()
	for _, path := range []string{omarchyDefaultMenuPath(), omarchyUserMenuPath()} {
		data, err := os.ReadFile(path)
		fmt.Fprintf(h, "%d:%s:%v:", len(data), path, err)
		h.Write(data)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

// reloadOmarchyCommands publishes the catalog and its search index together.
func (r *SysPlugin) reloadOmarchyCommands(ctx context.Context) bool {
	items, err := loadOmarchyMenuItems()
	if err != nil && r.omarchyGuards == nil {
		// On startup, an invalid user extension must not hide the default menu.
		util.GetLogger().Warn(ctx, fmt.Sprintf("omarchy menu extension: %v", err))
		items, err = loadOmarchyMenuItemsWithExtension(false)
	}
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("omarchy menu commands: %v", err))
		return false
	}
	state := newOmarchyWhenState()
	guards := &omarchyGuardCache{state: state, evaluate: evaluateOmarchyGuards, script: omarchyGuardScript(items)}
	commands := buildOmarchySysCommands(items, state, omarchyCommandIcon(), func(actionCtx context.Context, id string, action string) {
		var started *exec.Cmd
		r.runSystemAction(actionCtx, id, func() (*exec.Cmd, error) {
			var err error
			started, err = shell.Run("bash", "-lc", action)
			return started, err
		})
		if started != nil {
			// Settings can change after the shell starts, so refresh after it exits.
			util.Go(actionCtx, "wait for omarchy action", func() {
				_ = started.Wait()
				guards.refreshSoon(context.Background())
			})
		}
	})
	r.commandsMu.Lock()
	base := make([]SysCommand, 0, len(r.commands)+len(commands))
	for _, command := range r.commands {
		if !strings.HasPrefix(command.ID, omarchyCommandPrefix) {
			base = append(base, command)
		}
	}
	r.commands = append(base, commands...)
	r.omarchyGuards = guards
	r.ReleasePreparedSearchText()
	r.registerOmarchyCommands(ctx, commands)
	r.commandsMu.Unlock()
	guards.refreshIfStale(ctx)
	util.GetLogger().Info(ctx, fmt.Sprintf("loaded %d omarchy menu commands", len(commands)))
	return true
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
// A missing Omarchy install returns no items. Invalid input preserves the previous catalog.
func loadOmarchyMenuItems() ([]omarchyMenuItem, error) {
	return loadOmarchyMenuItemsWithExtension(true)
}

// loadOmarchyMenuItemsWithExtension can ignore a broken extension during startup.
func loadOmarchyMenuItemsWithExtension(includeExtension bool) ([]omarchyMenuItem, error) {
	defaultPath := omarchyDefaultMenuPath()
	if defaultPath == "" {
		return nil, nil
	}
	defaultRaw, err := os.ReadFile(defaultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, err
	}
	base, err := parseOmarchyMenuJSONC(string(defaultRaw))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", defaultPath, err)
	}

	var extra []omarchyRawItem
	if userPath := omarchyUserMenuPath(); includeExtension && userPath != "" {
		userRaw, readErr := os.ReadFile(userPath)
		if readErr == nil {
			parsed, parseErr := parseOmarchyMenuJSONC(string(userRaw))
			if parseErr != nil {
				return nil, fmt.Errorf("parse %s: %w", userPath, parseErr)
			}
			extra = parsed
		} else if !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("read %s: %w", userPath, readErr)
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
