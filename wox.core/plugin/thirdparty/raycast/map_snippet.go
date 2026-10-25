package raycast

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const snippetKeywordPrefix = "Keyword: "

// snippetBody is the note text for one snippet. Keeping the keyword changes the copied body.
func snippetBody(text, keyword string, keep bool) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !keep || strings.TrimSpace(keyword) == "" {
		return text
	}
	return snippetKeywordPrefix + strings.TrimSpace(keyword) + "\n\n" + text
}

func snippetHasDynamic(text string) bool {
	return strings.Contains(text, "{") && strings.Contains(text, "}")
}

// snippetFingerprint identifies one source snippet and the body option, independent of catalog order.
func snippetFingerprint(sourceID, title, text, keyword string, keep bool) string {
	keepText := "0"
	if keep {
		keepText = "1"
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{sourceID, title, text, keyword, keepText}, "\n")))
	return hex.EncodeToString(sum[:])
}
