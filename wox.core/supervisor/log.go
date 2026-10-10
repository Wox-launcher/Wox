package supervisor

import (
	"fmt"
	"os"
	"time"
)

// logf appends one supervisor line to ~/.wox/log/supervisor.log.
func (m *Manager) logf(format string, args ...any) {
	if err := os.MkdirAll(m.LogDirectory(), 0755); err != nil {
		return
	}
	file, err := os.OpenFile(m.SupervisorLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "[%s] %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
