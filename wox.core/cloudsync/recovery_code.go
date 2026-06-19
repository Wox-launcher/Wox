package cloudsync

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

func GenerateRecoveryCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	return formatRecoveryCode(encoded), nil
}

func NormalizeRecoveryCode(code string) string {
	clean := strings.ToUpper(code)
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.ReplaceAll(clean, " ", "")
	return clean
}

func formatRecoveryCode(code string) string {
	segments := []string{}
	for i := 0; i < len(code); i += 4 {
		end := i + 4
		if end > len(code) {
			end = len(code)
		}
		segments = append(segments, code[i:end])
	}
	return strings.Join(segments, "-")
}
