package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"wox/util"

	"github.com/google/uuid"
)

const (
	defaultMaxBufferBytes      = 8 * 1024 * 1024
	defaultMaxBufferLines      = 20000
	defaultChunkBytes          = 8 * 1024
	defaultInitialSnapshotByte = 64 * 1024
)

type EventEmitter func(ctx context.Context, uiSessionID string, method string, data any)

type Config struct {
	MaxBufferBytes       int
	MaxBufferLines       int
	ChunkBytes           int
	InitialSnapshotBytes int
	OutputDirectory      string
}

type subscriber struct {
	cursor int64
}

type Session struct {
	ID         string
	OutputPath string

	mu          sync.RWMutex
	state       SessionState
	ringBuffer  *RingBuffer
	subscribers map[string]*subscriber
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	config   Config
	emitter  EventEmitter
}

var (
	defaultManager *Manager
	managerOnce    sync.Once
)

func DefaultConfig() Config {
	outputDir := filepath.Join(util.GetLocation().GetLogDirectory(), "shell", "sessions")
	return Config{
		MaxBufferBytes:       defaultMaxBufferBytes,
		MaxBufferLines:       defaultMaxBufferLines,
		ChunkBytes:           defaultChunkBytes,
		InitialSnapshotBytes: defaultInitialSnapshotByte,
		OutputDirectory:      outputDir,
	}
}

func GetSessionManager() *Manager {
	managerOnce.Do(func() {
		defaultManager = NewSessionManager(DefaultConfig())
	})
	return defaultManager
}

func NewSessionManager(config Config) *Manager {
	if config.MaxBufferBytes <= 0 {
		config.MaxBufferBytes = defaultMaxBufferBytes
	}
	if config.MaxBufferLines <= 0 {
		config.MaxBufferLines = defaultMaxBufferLines
	}
	if config.ChunkBytes <= 0 {
		config.ChunkBytes = defaultChunkBytes
	}
	if config.InitialSnapshotBytes <= 0 {
		config.InitialSnapshotBytes = defaultInitialSnapshotByte
	}
	if config.OutputDirectory == "" {
		config.OutputDirectory = filepath.Join(os.TempDir(), "wox-shell-sessions")
	}

	return &Manager{
		sessions: map[string]*Session{},
		config:   config,
	}
}

func (m *Manager) SetEmitter(emitter EventEmitter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitter = emitter
}

func (m *Manager) CreateSession(ctx context.Context, params CreateSessionParams) (*Session, error) {
	if err := os.MkdirAll(m.config.OutputDirectory, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to ensure terminal output directory: %w", err)
	}

	sessionID := uuid.NewString()
	outputPath := filepath.Join(m.config.OutputDirectory, sessionID+".log")
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create terminal output file: %w", err)
	}
	file.Close()

	now := util.GetSystemTimestamp()
	session := &Session{
		ID:         sessionID,
		OutputPath: outputPath,
		state: SessionState{
			SessionID:   sessionID,
			Command:     params.Command,
			Interpreter: params.Interpreter,
			Status:      SessionStatusRunning,
			StartTime:   now,
			EndTime:     0,
			ExitCode:    0,
			Error:       "",
		},
		ringBuffer:  NewRingBuffer(m.config.MaxBufferBytes, m.config.MaxBufferLines),
		subscribers: map[string]*subscriber{},
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	return session, nil
}

func (m *Manager) GetState(sessionID string) (SessionState, bool) {
	session, ok := m.getSession(sessionID)
	if !ok {
		return SessionState{}, false
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	return session.state, true
}

func (m *Manager) SetState(ctx context.Context, sessionID string, status SessionStatus, exitCode int, errMsg string) {
	session, ok := m.getSession(sessionID)
	if !ok {
		return
	}

	session.mu.Lock()
	session.state.Status = status
	session.state.ExitCode = exitCode
	session.state.Error = errMsg
	if status == SessionStatusCompleted || status == SessionStatusFailed || status == SessionStatusKilled {
		session.state.EndTime = util.GetSystemTimestamp()
	}
	subscriberIDs := session.subscriberIDsLocked()
	state := session.state
	session.mu.Unlock()

	for _, uiSessionID := range subscriberIDs {
		m.emit(ctx, uiSessionID, "TerminalState", state)
	}
}

func (m *Manager) AppendChunk(ctx context.Context, sessionID string, content string) {
	if content == "" {
		return
	}

	session, ok := m.getSession(sessionID)
	if !ok {
		return
	}

	start, end, _ := session.ringBuffer.Append(content)
	_ = appendToFile(session.OutputPath, content)

	type emitEvent struct {
		uiSessionID string
		chunk       TerminalChunk
	}
	var events []emitEvent

	session.mu.Lock()
	for uiSessionID, sub := range session.subscribers {
		if sub.cursor < start {
			sub.cursor = start
		}
		if sub.cursor >= end {
			continue
		}

		chunk, nextCursor, truncated := session.ringBuffer.SliceFrom(sub.cursor, m.config.ChunkBytes)
		if chunk == "" {
			continue
		}
		sub.cursor = nextCursor
		events = append(events, emitEvent{
			uiSessionID: uiSessionID,
			chunk: TerminalChunk{
				SessionID:   sessionID,
				CursorStart: nextCursor - int64(len(chunk)),
				CursorEnd:   nextCursor,
				Content:     chunk,
				Truncated:   truncated,
				Timestamp:   util.GetSystemTimestamp(),
			},
		})
	}
	session.mu.Unlock()

	for _, event := range events {
		m.emit(ctx, event.uiSessionID, "TerminalChunk", event.chunk)
	}
}

func (m *Manager) Subscribe(ctx context.Context, uiSessionID string, sessionID string, cursor int64) (SessionState, error) {
	session, err := m.ensureSession(sessionID)
	if err != nil {
		return SessionState{}, err
	}

	session.mu.Lock()
	if cursor < 0 {
		endCursor := session.ringBuffer.EndCursor()
		startCursor := session.ringBuffer.StartCursor()
		cursor = endCursor - int64(m.config.InitialSnapshotBytes)
		if cursor < startCursor {
			cursor = startCursor
		}
	}
	session.subscribers[uiSessionID] = &subscriber{cursor: cursor}
	state := session.state
	session.mu.Unlock()

	m.flushSubscriber(ctx, session, uiSessionID)
	m.emit(ctx, uiSessionID, "TerminalState", state)

	return state, nil
}

func (m *Manager) Unsubscribe(uiSessionID string, sessionID string) {
	session, ok := m.getSession(sessionID)
	if !ok {
		return
	}

	session.mu.Lock()
	delete(session.subscribers, uiSessionID)
	session.mu.Unlock()
}

func (m *Manager) DeleteSession(sessionID string) error {
	if sessionID == "" {
		return nil
	}

	outputPath := filepath.Join(m.config.OutputDirectory, sessionID+".log")

	m.mu.Lock()
	if session, ok := m.sessions[sessionID]; ok {
		if session.OutputPath != "" {
			outputPath = session.OutputPath
		}
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete terminal output file: %w", err)
	}

	return nil
}

func (m *Manager) Search(ctx context.Context, req TerminalSearchRequest) (TerminalSearchResult, error) {
	session, ok := m.getSession(req.SessionID)
	if !ok {
		return TerminalSearchResult{}, fmt.Errorf("terminal session not found: %s", req.SessionID)
	}

	matchStart, matchEnd, nextCursor, found := session.ringBuffer.Search(req.Pattern, req.Cursor, req.Backward, req.CaseSensitive)
	result := TerminalSearchResult{
		SessionID:  req.SessionID,
		Found:      found,
		MatchStart: matchStart,
		MatchEnd:   matchEnd,
		NextCursor: nextCursor,
	}

	return result, nil
}

func (m *Manager) SnapshotTail(sessionID string, maxBytes int) (string, error) {
	session, ok := m.getSession(sessionID)
	if !ok {
		return "", fmt.Errorf("terminal session not found: %s", sessionID)
	}
	return session.ringBuffer.SnapshotTail(maxBytes), nil
}

func (m *Manager) getSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[sessionID]
	return session, ok
}

func (m *Manager) ensureSession(sessionID string) (*Session, error) {
	if session, ok := m.getSession(sessionID); ok {
		return session, nil
	}

	outputPath := filepath.Join(m.config.OutputDirectory, sessionID+".log")
	snapshot, err := readTail(outputPath, m.config.MaxBufferBytes)
	if err != nil {
		return nil, fmt.Errorf("terminal session not found: %s", sessionID)
	}

	session := &Session{
		ID:         sessionID,
		OutputPath: outputPath,
		state: SessionState{
			SessionID: sessionID,
			Status:    SessionStatusCompleted,
			StartTime: util.GetSystemTimestamp(),
			EndTime:   util.GetSystemTimestamp(),
		},
		ringBuffer:  NewRingBuffer(m.config.MaxBufferBytes, m.config.MaxBufferLines),
		subscribers: map[string]*subscriber{},
	}
	if snapshot != "" {
		session.ringBuffer.Append(snapshot)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.sessions[sessionID]; ok {
		return existing, nil
	}
	m.sessions[sessionID] = session
	return session, nil
}

func (m *Manager) emit(ctx context.Context, uiSessionID string, method string, data any) {
	m.mu.RLock()
	emitter := m.emitter
	m.mu.RUnlock()
	if emitter == nil {
		return
	}
	emitter(ctx, uiSessionID, method, data)
}

func (m *Manager) flushSubscriber(ctx context.Context, session *Session, uiSessionID string) {
	for {
		session.mu.RLock()
		sub, ok := session.subscribers[uiSessionID]
		if !ok {
			session.mu.RUnlock()
			return
		}
		cursor := sub.cursor
		session.mu.RUnlock()

		chunk, nextCursor, truncated := session.ringBuffer.SliceFrom(cursor, m.config.ChunkBytes)
		if chunk == "" {
			return
		}

		session.mu.Lock()
		if current, exists := session.subscribers[uiSessionID]; exists {
			current.cursor = nextCursor
		}
		session.mu.Unlock()

		m.emit(ctx, uiSessionID, "TerminalChunk", TerminalChunk{
			SessionID:   session.ID,
			CursorStart: nextCursor - int64(len(chunk)),
			CursorEnd:   nextCursor,
			Content:     chunk,
			Truncated:   truncated,
			Timestamp:   util.GetSystemTimestamp(),
		})
	}
}

func (s *Session) subscriberIDsLocked() []string {
	ids := make([]string, 0, len(s.subscribers))
	for id := range s.subscribers {
		ids = append(ids, id)
	}
	return ids
}

func appendToFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

func readTail(path string, maxBytes int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	size := stat.Size()
	offset := int64(0)
	if maxBytes > 0 && size > int64(maxBytes) {
		offset = size - int64(maxBytes)
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
