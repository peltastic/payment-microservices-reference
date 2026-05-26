package logsafe

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func ShortHash(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	hash := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(hash[:])[:12]
}
