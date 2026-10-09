package dotnet

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/util"
	"wox/util/shell"
)

const dotnetCallTimeout = 60 * time.Second

// previewResultHold waits out a fast query before painting an in-flight result
// preview. Dictionary publishes the exact match, then returns the full list a
// few milliseconds later. Painting that one-row preview collapses the window,
// and the complete list opens it again. A query still running after the hold
// is slow enough that the early rows are worth showing.
const previewResultHold = 50 * time.Millisecond

// mainWindowHideReportDelay is the pause after the launcher is hidden before
// the plugin is told. Flow does the same so GetForegroundWindow returns the
// window that was in front, not the launcher. Window Manager minimizes that window.
const mainWindowHideReportDelay = 50 * time.Millisecond

// dotNetLaunch is everything the plugin process needs before the first query.
type dotNetLaunch struct {
	Name       string
	Directory  string
	Entry      string
	PluginID   string
	Author     string
	Version    string
	Language   string
	UILanguage string
	Keyword    string
	IcoPath    string
	DotNetPath string
	HostDir    string
	HostErr    string
	Bridge     dotnetBridge
}

type dotnetBridge interface {
	ChangeQuery(ctx context.Context, query string)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	Notify(ctx context.Context, title string, subtitle string)
	CopyText(ctx context.Context, text string)
	OpenPath(ctx context.Context, target string) error
	OpenDirectory(ctx context.Context, directory string, fileName string) error
	ShellRun(ctx context.Context, program string, command string) error
	Refresh(ctx context.Context)
	UpdateResults(ctx context.Context, search string, results []dotnetResult)
}

type dotnetSession struct {
	launch dotNetLaunch

	mu           sync.Mutex
	writeMu      sync.Mutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	alive        bool
	closed       bool
	nextID       int64
	pending      map[string]chan dotnetMessage
	done         chan struct{}
	exitErr      error
	stderr       bytes.Buffer
	eventQueue   []dotnetMessage
	eventRunning bool
	// queryInFlight is true only until the current query response is read.
	// A results event during that window is a preview of the list Query will return.
	queryInFlight   bool
	queryGeneration int64
	// previewHeld is the latest in-flight result list waiting out previewResultHold.
	// A fast query response discards it so the launcher paints the complete list once.
	previewHeld  *dotnetMessage
	previewTimer *time.Timer
}

func newDotNetSession(launch dotNetLaunch) *dotnetSession {
	return &dotnetSession{
		launch:  launch,
		pending: map[string]chan dotnetMessage{},
	}
}

// Start launches the loader and waits until the plugin has initialized.
func (s *dotnetSession) Start(ctx context.Context) error {
	if strings.TrimSpace(s.launch.HostErr) != "" {
		return errors.New(s.launch.HostErr)
	}
	if strings.TrimSpace(s.launch.DotNetPath) == "" || strings.TrimSpace(s.launch.HostDir) == "" {
		return errors.New(".NET flow host is not available")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("plugin process is closed")
	}
	if s.alive {
		s.mu.Unlock()
		return nil
	}
	if err := s.startLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	reply, err := s.call(ctx, map[string]any{
		"method":     "init",
		"directory":  s.launch.Directory,
		"entry":      s.launch.Entry,
		"pluginId":   s.launch.PluginID,
		"name":       s.launch.Name,
		"author":     s.launch.Author,
		"version":    s.launch.Version,
		"language":   s.launch.Language,
		"uiLanguage": s.launch.UILanguage,
		"keyword":    s.launch.Keyword,
		"icoPath":    s.launch.IcoPath,
	})
	if err != nil {
		s.Close()
		return err
	}
	if !reply.OK {
		s.Close()
		if reply.Error == "" {
			reply.Error = "plugin init failed"
		}
		return errors.New(reply.Error)
	}
	return nil
}

// Query runs one search. Action ids in the reply are valid until the next query.
func (s *dotnetSession) Query(ctx context.Context, search string, rawQuery string, keyword string) ([]dotnetResult, error) {
	generation := s.beginQuery()
	defer s.endQuery(generation)
	reply, err := s.call(ctx, map[string]any{
		"method":   "query",
		"search":   search,
		"rawQuery": rawQuery,
		"keyword":  keyword,
	})
	if err != nil {
		return nil, err
	}
	if !reply.OK {
		if reply.Error == "" {
			reply.Error = "plugin query failed"
		}
		return nil, errors.New(reply.Error)
	}
	return reply.Results, nil
}

// Action runs one result action that the last query registered.
func (s *dotnetSession) Action(ctx context.Context, actionID string) error {
	reply, err := s.call(ctx, map[string]any{
		"method": "action",
		"action": actionID,
	})
	if err != nil {
		return err
	}
	if !reply.OK {
		if reply.Error == "" {
			reply.Error = "plugin action failed"
		}
		return errors.New(reply.Error)
	}
	return nil
}

// Close stops the plugin process. A later call fails.
func (s *dotnetSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.discardHeldPreviewLocked()
	s.killLocked()
}

func (s *dotnetSession) startLocked() error {
	dll := filepath.Join(s.launch.HostDir, "Wox.Flow.DotNetHost.dll")
	cmd := shell.BuildCommand(s.launch.DotNetPath, nil, "exec", dll)
	cmd.Dir = s.launch.Directory
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	s.stdin = stdin
	s.alive = true
	s.exitErr = nil
	s.stderr.Reset()
	s.done = make(chan struct{})
	done := s.done
	go s.readStderr(stderr)
	go s.readStdout(stdout, cmd, done)
	util.GetLogger().Info(context.Background(), fmt.Sprintf("[flow-dotnet:%s] started %s", s.launch.Name, dll))
	return nil
}

func (s *dotnetSession) call(ctx context.Context, payload map[string]any) (dotnetMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dotnetCallTimeout)
		defer cancel()
	}
	s.mu.Lock()
	if !s.alive || s.stdin == nil {
		err := s.currentExitErrorLocked()
		s.mu.Unlock()
		if err == nil {
			err = errors.New("plugin process is not running")
		}
		return dotnetMessage{}, err
	}
	s.nextID++
	id := fmt.Sprintf("wox-%d", s.nextID)
	payload["id"] = id
	wait := make(chan dotnetMessage, 1)
	s.pending[id] = wait
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()
	if err := s.writeJSON(payload); err != nil {
		return dotnetMessage{}, err
	}
	select {
	case message, ok := <-wait:
		if !ok {
			return dotnetMessage{}, s.currentExitError()
		}
		return message, nil
	case <-ctx.Done():
		return dotnetMessage{}, ctx.Err()
	case <-s.doneChan():
		return dotnetMessage{}, s.currentExitError()
	}
}

func (s *dotnetSession) doneChan() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return s.done
}

func (s *dotnetSession) writeJSON(payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.stdin.Write(raw)
	return err
}

func (s *dotnetSession) readStdout(stdout io.Reader, cmd *exec.Cmd, done chan struct{}) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		s.dispatchLine(scanner.Text())
	}
	err := scanner.Err()
	waitErr := cmd.Wait()
	s.mu.Lock()
	s.alive = false
	if waitErr != nil {
		s.exitErr = waitErr
	} else if err != nil {
		s.exitErr = err
	}
	if s.done != nil {
		select {
		case <-s.done:
		default:
			close(s.done)
		}
	}
	pending := s.pending
	s.pending = map[string]chan dotnetMessage{}
	s.mu.Unlock()
	for _, wait := range pending {
		close(wait)
	}
}

func (s *dotnetSession) dispatchLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] != '{' {
		return
	}
	var message dotnetMessage
	if err := json.Unmarshal([]byte(line), &message); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("[flow-dotnet:%s] %s", s.launch.Name, err.Error()))
		return
	}
	if message.Event != "" {
		// The reader must keep going. A refresh can start another query, and
		// that query waits for this reader to deliver its response.
		s.noteResultUpdate(&message)
		s.enqueueEvent(message)
		return
	}
	s.mu.Lock()
	wait := s.pending[message.ID]
	s.mu.Unlock()
	if wait == nil {
		return
	}
	select {
	case wait <- message:
	default:
	}
}

func (s *dotnetSession) enqueueEvent(message dotnetMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.eventQueue = append(s.eventQueue, message)
	if s.eventRunning {
		return
	}
	s.eventRunning = true
	go s.drainEvents()
}

func (s *dotnetSession) drainEvents() {
	for {
		s.mu.Lock()
		if len(s.eventQueue) == 0 || s.closed {
			s.eventQueue = nil
			s.eventRunning = false
			s.mu.Unlock()
			return
		}
		message := s.eventQueue[0]
		s.eventQueue[0] = dotnetMessage{}
		s.eventQueue = s.eventQueue[1:]
		s.mu.Unlock()
		s.handleEvent(message)
	}
}

// reportMainWindowHidden tells the plugin the launcher is gone for this hide's generation.
// An older report is ignored once a newer query or action has marked the launcher open.
func (s *dotnetSession) reportMainWindowHidden(epoch int64) {
	s.mu.Lock()
	stdin := s.stdin
	closed := s.closed
	s.mu.Unlock()
	if stdin == nil || closed {
		return
	}
	if err := s.writeJSON(map[string]any{
		"method":  "visibility",
		"visible": false,
		"epoch":   epoch,
	}); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("[flow-dotnet:%s] visibility: %s", s.launch.Name, err.Error()))
	}
}

func (s *dotnetSession) handleEvent(message dotnetMessage) {
	bridge := s.launch.Bridge
	if bridge == nil {
		return
	}
	ctx := context.Background()
	switch message.Event {
	case "changeQuery":
		bridge.ChangeQuery(ctx, message.Query)
	case "hide":
		bridge.HideApp(ctx)
		epoch := message.Epoch
		time.AfterFunc(mainWindowHideReportDelay, func() {
			s.reportMainWindowHidden(epoch)
		})
	case "show":
		bridge.ShowApp(ctx)
	case "notify":
		bridge.Notify(ctx, message.Title, message.Subtitle)
	case "copy":
		bridge.CopyText(ctx, message.Text)
	case "open":
		if err := bridge.OpenPath(ctx, message.Path); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("[flow-dotnet:%s] open: %s", s.launch.Name, err.Error()))
		}
	case "openDirectory":
		if err := bridge.OpenDirectory(ctx, message.Directory, message.File); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("[flow-dotnet:%s] open directory: %s", s.launch.Name, err.Error()))
		}
	case "shell":
		if err := bridge.ShellRun(ctx, message.Program, message.Command); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("[flow-dotnet:%s] shell: %s", s.launch.Name, err.Error()))
		}
	case "refresh":
		bridge.Refresh(ctx)
	case "results":
		if message.Preview {
			s.holdPreviewResult(message)
			return
		}
		s.publishResultUpdate(message)
	}
}

// holdPreviewResult keeps the latest in-flight list until the query has had time to finish.
// A newer preview replaces the pending list without extending the wait.
func (s *dotnetSession) holdPreviewResult(message dotnetMessage) {
	if !s.shouldApplyResultUpdate(message) {
		s.logDroppedPreview()
		return
	}
	held := message
	s.mu.Lock()
	s.previewHeld = &held
	if s.previewTimer != nil {
		s.mu.Unlock()
		return
	}
	s.previewTimer = time.AfterFunc(previewResultHold, s.releaseHeldPreview)
	s.mu.Unlock()
}

// releaseHeldPreview paints the pending preview, or drops it when the query already returned.
func (s *dotnetSession) releaseHeldPreview() {
	s.mu.Lock()
	message := s.previewHeld
	timer := s.previewTimer
	s.previewHeld = nil
	s.previewTimer = nil
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	if message == nil {
		return
	}
	s.publishResultUpdate(*message)
}

// publishResultUpdate applies one result list, dropping a preview that lost the race with its query response.
func (s *dotnetSession) publishResultUpdate(message dotnetMessage) {
	bridge := s.launch.Bridge
	if bridge == nil {
		return
	}
	if !s.shouldApplyResultUpdate(message) {
		s.logDroppedPreview()
		return
	}
	util.GetLogger().Info(context.Background(), fmt.Sprintf("[flow-dotnet:%s] results updated: %d", s.launch.Name, len(message.Results)))
	bridge.UpdateResults(context.Background(), message.Query, message.Results)
}

func (s *dotnetSession) logDroppedPreview() {
	util.GetLogger().Debug(context.Background(), fmt.Sprintf("[flow-dotnet:%s] dropped preview results", s.launch.Name))
}

// beginQuery marks the call whose response is the complete result list.
func (s *dotnetSession) beginQuery() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discardHeldPreviewLocked()
	s.queryGeneration++
	s.queryInFlight = true
	return s.queryGeneration
}

// endQuery clears the in-flight mark for the generation that started it.
// A newer query owns the flag when generations differ.
func (s *dotnetSession) endQuery(generation int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queryGeneration == generation {
		s.queryInFlight = false
		s.discardHeldPreviewLocked()
	}
}

// discardHeldPreviewLocked drops a preview that must not paint after its query ends.
func (s *dotnetSession) discardHeldPreviewLocked() {
	if s.previewTimer != nil {
		s.previewTimer.Stop()
		s.previewTimer = nil
	}
	s.previewHeld = nil
}

// noteResultUpdate stamps a results event at read time.
// Stdout order is the only signal that the event was emitted before the query response.
func (s *dotnetSession) noteResultUpdate(message *dotnetMessage) {
	if message == nil || message.Event != "results" {
		return
	}
	s.mu.Lock()
	message.Preview = s.queryInFlight
	message.Generation = s.queryGeneration
	s.mu.Unlock()
}

// shouldApplyResultUpdate drops a preview once its query call has returned.
// The response is the complete list. Applying the earlier preview would hide rows added after the event.
func (s *dotnetSession) shouldApplyResultUpdate(message dotnetMessage) bool {
	if message.Event != "results" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !message.Preview {
		return true
	}
	return s.queryInFlight && message.Generation == s.queryGeneration
}

func (s *dotnetSession) readStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 16*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		s.mu.Lock()
		if s.stderr.Len() < 64*1024 {
			s.stderr.WriteString(line)
			s.stderr.WriteByte('\n')
		}
		s.mu.Unlock()
		util.GetLogger().Info(context.Background(), fmt.Sprintf("[flow-dotnet:%s] %s", s.launch.Name, line))
	}
}

func (s *dotnetSession) killLocked() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.alive = false
}

func (s *dotnetSession) currentExitError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentExitErrorLocked()
}

func (s *dotnetSession) currentExitErrorLocked() error {
	if s.exitErr != nil {
		text := strings.TrimSpace(s.stderr.String())
		if text != "" {
			return fmt.Errorf("%w: %s", s.exitErr, text)
		}
		return s.exitErr
	}
	text := strings.TrimSpace(s.stderr.String())
	if text != "" {
		return errors.New(text)
	}
	return errors.New("plugin process exited")
}
