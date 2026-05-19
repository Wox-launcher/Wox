package util

import (
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`^(http://www\.|https://www\.|http://|https://)?([a-z0-9]+([\-\.][a-z0-9]+)*\.[a-z]{2,5}|((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))(:[0-9]{1,5})?(/.*)?$`)

// IsUrl keeps Wox's URL recognition rule in one place so direct URL queries
// and clipboard link filtering do not drift. The pattern is intentionally the
// same whole-string rule that the URL plugin used before this shared helper.
func IsUrl(raw string) bool {
	return len(urlPattern.FindStringIndex(strings.TrimSpace(raw))) > 0
}

// NormalizeUrl prepares a URL-like value for shell opening. Bare domains and IP
// addresses keep the URL plugin's existing HTTPS default, while explicit HTTP
// and HTTPS links are left unchanged.
func NormalizeUrl(raw string) string {
	normalized := strings.TrimSpace(raw)
	if strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://") {
		return normalized
	}
	return "https://" + normalized
}
