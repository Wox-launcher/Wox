package notifier

import (
	"strings"
	"wox/util"
)

func normalizeNotificationMessage(message string) string {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.Split(message, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func Notify(message string) {
	msg := normalizeNotificationMessage(message)
	if msg == "" {
		return
	}

	util.Go(util.NewTraceContext(), "notifier.Notify", func() {
		ShowNotification(msg)
	})
}
