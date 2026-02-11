package terminal

type SessionStatus string

const (
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusKilled    SessionStatus = "killed"
)

type SessionState struct {
	SessionID   string        `json:"SessionId"`
	Command     string        `json:"Command"`
	Interpreter string        `json:"Interpreter"`
	Status      SessionStatus `json:"Status"`
	StartTime   int64         `json:"StartTime"`
	EndTime     int64         `json:"EndTime"`
	ExitCode    int           `json:"ExitCode"`
	Error       string        `json:"Error"`
}

type TerminalChunk struct {
	SessionID   string `json:"SessionId"`
	CursorStart int64  `json:"CursorStart"`
	CursorEnd   int64  `json:"CursorEnd"`
	Content     string `json:"Content"`
	Truncated   bool   `json:"Truncated"`
	Timestamp   int64  `json:"Timestamp"`
}

type TerminalSearchRequest struct {
	SessionID     string `json:"SessionId"`
	Pattern       string `json:"Pattern"`
	Cursor        int64  `json:"Cursor"`
	Backward      bool   `json:"Backward"`
	CaseSensitive bool   `json:"CaseSensitive"`
}

type TerminalSearchResult struct {
	SessionID  string `json:"SessionId"`
	Found      bool   `json:"Found"`
	MatchStart int64  `json:"MatchStart"`
	MatchEnd   int64  `json:"MatchEnd"`
	NextCursor int64  `json:"NextCursor"`
}

type CreateSessionParams struct {
	Command     string
	Interpreter string
}
