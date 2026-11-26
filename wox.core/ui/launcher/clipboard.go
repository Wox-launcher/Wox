package launcher

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"wox/ui/launcher/component"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/imageencode"
)

// ClipboardProvider is the text clipboard backend used by text fields for copy/cut/paste.
// Re-exported here so callers that already depend on the launcher package can wire it
// without reaching into the component subpackage.
type ClipboardProvider = component.ClipboardProvider

// SetClipboardProvider registers the clipboard backend used by all Wox text fields.
func SetClipboardProvider(provider ClipboardProvider) {
	component.SetClipboardProvider(provider)
}

// utilClipboardProvider adapts wox/util/clipboard to the component.ClipboardProvider interface.
// ReadText returns "" + nil for an empty or non-text clipboard so Cmd+V is a no-op.
type utilClipboardProvider struct{}

func (utilClipboardProvider) ReadText() (string, error) {
	return clipboard.ReadText()
}

func (utilClipboardProvider) WriteText(text string) error {
	return clipboard.WriteText(text)
}

// NewUtilClipboardProvider returns a ClipboardProvider backed by wox/util/clipboard.
func NewUtilClipboardProvider() ClipboardProvider {
	return utilClipboardProvider{}
}

func (a *App) queryViewClipboardPaste() error {
	a.pasteQueryClipboard(true)
	return nil
}

// pasteQueryClipboard keeps text paste behavior and imports one image per shortcut press.
func (a *App) pasteQueryClipboard(allowImage bool) {
	if !a.queryCanFocus() {
		return
	}
	a.querySession.selectTouchedQueryBlocks()
	text, err := clipboard.ReadText()
	if err != nil {
		return
	}
	text = normalizeQueryPaste(text)
	if text == "" {
		if allowImage {
			a.pasteQueryClipboardImage()
		}
		return
	}
	if a.querySession.queryHintEditorState.allSelected {
		a.querySession.replaceWholeQueryHint(text)
		return
	}
	previousText := a.querySession.editor.State().Text
	if a.querySession.editor.InsertTextSeparate(text) {
		a.applyQueryTextChangeLocked(a.querySession.editor.State().Text)
	}
	_ = a.window.Invalidate()
	a.previews.reconcileSelectedPreview()
	if err := a.sendCurrentQuery(); err != nil {
		util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("send query after paste: %v", err))
	}
	a.resizeLauncherForQueryLineChange(previousText)
}

// pasteQueryClipboardImage reuses copied image files and prepares bitmap data off the UI thread.
func (a *App) pasteQueryClipboardImage() {
	a.querySession.clipboardImagePasteGeneration++
	if copied, err := clipboard.ReadFilePaths(); err == nil {
		paths := cleanFileDropPaths(copied)
		if len(paths) > 0 && !slices.ContainsFunc(paths, func(path string) bool {
			return !util.IsImageFile(path) || !util.IsFileExists(path)
		}) {
			a.handleLauncherSelectionFileDrop(paths)
			return
		}
	}

	ctx := a.lifecycleCtx
	snapshot, err := clipboard.ReadImageSnapshot()
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("read query clipboard image: %v", err))
		return
	}
	if snapshot == nil {
		return
	}

	generation := a.querySession.clipboardImagePasteGeneration
	queryID := a.querySession.query.QueryID
	util.Go(ctx, "prepare pasted query image", func() {
		path, err := saveQueryClipboardImage(snapshot)
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("save query clipboard image: %v", err))
			return
		}
		used := false
		if err := a.runOnUI("apply pasted query image", func() {
			// An older paste must not replace a newer query or refocus an inactive launcher.
			if a.destroyed.Load() || !a.visible || !a.queryCanFocus() ||
				a.querySession.clipboardImagePasteGeneration != generation || a.querySession.query.QueryID != queryID {
				return
			}
			used = true
			a.handleLauncherSelectionFileDrop([]string{path})
		}); err != nil {
			util.GetLogger().Warn(ctx, err.Error())
		}
		if !used {
			_ = os.Remove(path)
		}
	})
}

// saveQueryClipboardImage keeps the file available to selection actions and normal image cache cleanup.
func saveQueryClipboardImage(snapshot *clipboard.ImageSnapshot) (string, error) {
	img, err := snapshot.Decode(40_000_000)
	if err != nil {
		return "", err
	}
	directory := util.GetLocation().GetImageCacheDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(directory); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "pasted-image-*.png")
	if err != nil {
		return "", err
	}
	if err := errors.Join(imageencode.PNG(file, img), file.Close()); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
