package util

import (
	"os"
	"strconv"
	"strings"
)

func DiagnosticLoggingEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("ANYTLS_DEBUG_VERBOSE"))
	if raw == "" {
		return false
	}
	enabled, err := strconv.ParseBool(raw)
	if err == nil {
		return enabled
	}
	switch strings.ToLower(raw) {
	case "1", "yes", "on":
		return true
	default:
		return false
	}
}
