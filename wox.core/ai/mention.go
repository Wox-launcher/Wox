package ai

import (
	"regexp"
	"strings"

	"wox/common"
)

var mentionTagPattern = regexp.MustCompile(`\{([a-z][a-z0-9_]*):([^}]*)\}`)

// StripMentionTags removes {kind:payload} placeholders for registered @mention kinds.
// Skill tags stay in the text because they are not chat @mentions.
func StripMentionTags(text string) string {
	return strings.TrimSpace(mentionTagPattern.ReplaceAllStringFunc(text, func(raw string) string {
		parts := mentionTagPattern.FindStringSubmatch(raw)
		if len(parts) < 2 || !common.IsAIMentionKind(parts[1]) {
			return raw
		}
		return ""
	}))
}
