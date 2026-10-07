//go:build darwin

package app

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"wox/util/shell"
)

const macMoveAppToTrashScript = `use framework "Foundation"
on run argv
  set appPath to item 1 of argv
  set appURL to current application's NSURL's fileURLWithPath:appPath
  set fileManager to current application's NSFileManager's defaultManager()
  set {moved, trashError} to fileManager's trashItemAtURL:appURL resultingItemURL:(missing value) |error|:(reference)
  if moved as boolean then
    return "ok"
  end if
  error (trashError's localizedDescription() as text)
end run`

var (
	errMacUninstallNotFound   = errors.New("mac app bundle not found")
	errMacUninstallNotAllowed = errors.New("mac app cannot be uninstalled")
)

func isAppUninstallNotFound(err error) bool {
	return errors.Is(err, errMacUninstallNotFound)
}

func isAppUninstallNotAllowed(err error) bool {
	return errors.Is(err, errMacUninstallNotAllowed)
}

// executeAppUninstall moves the indexed bundle to the Trash.
// NSFileManager deletes the path itself, so a Homebrew symlink in /Applications
// is removed without following it into the Caskroom copy.
func executeAppUninstall(ctx context.Context, info appInfo) error {
	if !shouldOfferMacUninstall(info) {
		return errMacUninstallNotAllowed
	}
	return moveMacAppToTrash(info.Path)
}

func moveMacAppToTrash(appPath string) error {
	if _, err := os.Lstat(appPath); err != nil {
		if os.IsNotExist(err) {
			return errMacUninstallNotFound
		}
		return err
	}

	_, err := shell.RunOutput("osascript", "-e", macMoveAppToTrashScript, appPath)
	if err == nil {
		return nil
	}
	if _, statErr := os.Lstat(appPath); os.IsNotExist(statErr) {
		return errMacUninstallNotFound
	}
	return errors.New(cleanMacUninstallCommandError(err))
}

func cleanMacUninstallCommandError(err error) string {
	message := strings.TrimSpace(err.Error())
	const marker = "execution error: "
	if index := strings.LastIndex(message, marker); index >= 0 {
		message = strings.TrimSpace(message[index+len(marker):])
	}
	if index := strings.LastIndex(message, " ("); index > 0 && strings.HasSuffix(message, ")") {
		if _, convErr := strconv.Atoi(message[index+2 : len(message)-1]); convErr == nil {
			message = strings.TrimSpace(message[:index])
		}
	}
	return message
}
