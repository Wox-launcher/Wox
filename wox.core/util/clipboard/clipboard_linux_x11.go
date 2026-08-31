//go:build linux

package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"
	"sync"
	"time"
	"wox/util"
)

const (
	x11ClipboardCommandTimeout = 3 * time.Second
	x11MimeUTF8String          = "UTF8_STRING"
)

// x11Clipboard talks to the X11 CLIPBOARD selection through xclip or xsel.
// Openbox, XFCE, i3, MATE, and Xvfb all share this path. GTK is not used here
// because Watch runs off the UI thread and Wox's Linux UI already owns GTK.
type x11Clipboard struct {
	mu               sync.Mutex
	tool             x11ClipboardTool
	toolErr          error
	toolOnce         sync.Once
	fingerprint      string
	lastNoData       time.Time
	lastNoDataReason string
	activeLog        bool
}

type x11ClipboardTool struct {
	bin  string
	kind string
}

func newX11Clipboard() *x11Clipboard {
	return &x11Clipboard{}
}

func (c *x11Clipboard) name() string {
	return "x11"
}

func (c *x11Clipboard) readContentType() Type {
	targets, err := c.readTargets()
	if err != nil {
		return ""
	}
	return x11TargetsContentType(targets)
}

func (c *x11Clipboard) readText() (string, error) {
	data, err := c.readMIME(portalMimeTextUTF8, portalMimeTextPlain, "UTF8_STRING", "STRING", "TEXT")
	if err != nil {
		return "", err
	}
	text := strings.TrimRight(string(data), "\x00")
	if text == "" {
		return "", noDataErr
	}
	return text, nil
}

func (c *x11Clipboard) readFilePaths() ([]string, error) {
	data, err := c.readMIME(portalMimeURIList)
	if err != nil {
		return nil, err
	}
	paths := parsePortalURIList(string(data))
	if len(paths) == 0 {
		return nil, noDataErr
	}
	return paths, nil
}

func (c *x11Clipboard) readImage() (image.Image, error) {
	data, err := c.readMIME(portalMimePNG)
	if err != nil {
		return nil, err
	}
	img, decodeErr := png.Decode(bytes.NewReader(data))
	if decodeErr != nil {
		return nil, fmt.Errorf("clipboard: failed to decode X11 PNG: %w", decodeErr)
	}
	return img, nil
}

func (c *x11Clipboard) writeText(text string) error {
	// X11 STRING and charset-less text/plain are Latin-1. Writing UTF-8
	// Chinese under those targets makes Wox render correctly while Firefox
	// and other paste clients decode the same bytes as mojibake. UTF8_STRING
	// is the atom GTK/Qt request for Unicode text.
	return c.writeMIME(x11MimeUTF8String, []byte(text))
}

func (c *x11Clipboard) writeFilePaths(paths []string) error {
	payload, err := buildPortalURIListPayload(paths)
	if err != nil {
		return err
	}
	return c.writeMIME(portalMimeURIList, payload)
}

func (c *x11Clipboard) writeImageBytes(pngData []byte) error {
	return c.writeMIME(portalMimePNG, pngData)
}

func (c *x11Clipboard) isChanged() bool {
	// Read TARGETS directly instead of through readContentType so an ignored
	// clipboard can report the atoms it actually advertised. Without them, an
	// image the watcher never picked up is indistinguishable from an empty
	// clipboard.
	targets, targetsErr := c.readTargets()
	if targetsErr != nil {
		c.logNoData(targets, "", targetsErr)
		return false
	}
	contentType := x11TargetsContentType(targets)
	var payload []byte
	var err error
	switch contentType {
	case ClipboardTypeText:
		var text string
		text, err = c.readText()
		payload = []byte(text)
	case ClipboardTypeFile:
		var paths []string
		paths, err = c.readFilePaths()
		payload = []byte(strings.Join(paths, "\n"))
	case ClipboardTypeImage:
		payload, err = c.readMIME(portalMimePNG)
	default:
		c.logNoData(targets, contentType, noDataErr)
		return false
	}
	if err != nil {
		c.logNoData(targets, contentType, err)
		return false
	}

	fingerprint := string(contentType) + ":" + hashLinuxClipboardBytes(payload)
	c.mu.Lock()
	defer c.mu.Unlock()
	if fingerprint == c.fingerprint {
		return false
	}
	c.fingerprint = fingerprint
	c.logActiveLocked()
	return true
}

// watchSnapshot annotates a failed read with the atoms the selection advertised.
// The content type alone could not explain a read that failed right after TARGETS
// reported data, because each xclip call queries a selection owner that may have
// already exited.
func (c *x11Clipboard) watchSnapshot() string {
	targets, err := c.readTargets()
	if err != nil {
		return fmt.Sprintf("backend=x11 targets_err=%v", err)
	}
	return fmt.Sprintf("backend=x11 type=%s targets=%v", x11TargetsContentType(targets), targets)
}

func (c *x11Clipboard) readTargets() ([]string, error) {
	tool, err := c.clipboardTool()
	if err != nil {
		return nil, err
	}
	if tool.kind == "xsel" {
		// xsel cannot list TARGETS; treat a successful text read as text.
		text, textErr := c.readText()
		if textErr != nil {
			return nil, textErr
		}
		if text == "" {
			return nil, noDataErr
		}
		return []string{portalMimeTextPlain}, nil
	}

	output, err := runX11ClipboardCommand(tool.bin, []string{"-selection", "clipboard", "-t", "TARGETS", "-o"}, nil)
	if err != nil {
		return nil, err
	}
	if len(output) == 0 {
		return nil, noDataErr
	}
	return splitClipboardLines(string(output)), nil
}

func (c *x11Clipboard) readMIME(mimeTypes ...string) ([]byte, error) {
	tool, err := c.clipboardTool()
	if err != nil {
		return nil, err
	}
	if tool.kind == "xsel" {
		output, readErr := runX11ClipboardCommand(tool.bin, []string{"--clipboard", "--output"}, nil)
		if readErr != nil {
			return nil, readErr
		}
		if len(output) == 0 {
			return nil, noDataErr
		}
		return output, nil
	}

	var lastErr error
	for _, mimeType := range mimeTypes {
		output, readErr := runX11ClipboardCommand(tool.bin, []string{"-selection", "clipboard", "-t", mimeType, "-o"}, nil)
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if len(output) == 0 {
			lastErr = noDataErr
			continue
		}
		return output, nil
	}
	if lastErr == nil {
		return nil, noDataErr
	}
	return nil, lastErr
}

func (c *x11Clipboard) writeMIME(mimeType string, payload []byte) error {
	tool, err := c.clipboardTool()
	if err != nil {
		return err
	}
	if tool.kind == "xsel" {
		return runX11ClipboardCommandErr(tool.bin, []string{"--clipboard", "--input"}, payload)
	}
	return runX11ClipboardCommandErr(tool.bin, []string{"-selection", "clipboard", "-t", mimeType, "-i"}, payload)
}

func (c *x11Clipboard) clipboardTool() (x11ClipboardTool, error) {
	c.toolOnce.Do(func() {
		c.tool, c.toolErr = detectX11ClipboardTool()
	})
	return c.tool, c.toolErr
}

// logNoData explains why the watcher ignored the clipboard. Only an unchanged
// reason is throttled, so an idle empty clipboard still cannot spam the log while
// a new observation is always reported once. Throttling by time alone muted the
// whole window where a clipboard write went missing, leaving no record of what
// the watcher saw.
func (c *x11Clipboard) logNoData(targets []string, contentType Type, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	reason := fmt.Sprintf("type=%q targets=%v: %v", contentType, targets, err)
	now := time.Now()
	if reason == c.lastNoDataReason && now.Sub(c.lastNoData) < linuxClipboardNoDataLogInterval {
		return
	}
	c.lastNoData = now
	c.lastNoDataReason = reason
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: Linux X11 watcher sees no readable clipboard content: %s", reason))
}

func (c *x11Clipboard) logActiveLocked() {
	if c.activeLog {
		return
	}
	c.activeLog = true
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("clipboard: Linux X11 backend active tool=%s", c.tool.kind))
}

func detectX11ClipboardTool() (x11ClipboardTool, error) {
	if path, err := exec.LookPath("xclip"); err == nil {
		return x11ClipboardTool{bin: path, kind: "xclip"}, nil
	}
	if path, err := exec.LookPath("xsel"); err == nil {
		return x11ClipboardTool{bin: path, kind: "xsel"}, nil
	}
	return x11ClipboardTool{}, errors.New("clipboard: no X11 clipboard tool found (xclip or xsel)")
}

func x11TargetsContentType(targets []string) Type {
	return portalMimeTypesContentType(targets)
}

func splitClipboardLines(output string) []string {
	raw := strings.Split(output, "\n")
	targets := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line != "" {
			targets = append(targets, line)
		}
	}
	return targets
}

func runX11ClipboardCommand(bin string, args []string, stdin []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), x11ClipboardCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("clipboard: X11 clipboard command timed out: %s", bin)
		}
		return nil, err
	}
	return output, nil
}

func runX11ClipboardCommandErr(bin string, args []string, stdin []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), x11ClipboardCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	// Clipboard writers fork an owner that keeps the selection alive. Run avoids
	// the output pipe that would otherwise stay open for that owner's lifetime.
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("clipboard: X11 clipboard command timed out: %s", bin)
	}
	return err
}
