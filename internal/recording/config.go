// Package recording persists explicitly bounded local flights for replay.
package recording

import (
	"strconv"
	"strings"
	"time"
)

const (
	// EnvEnabled controls whether recording storage and HTTP routes are active.
	EnvEnabled = "GCS_RECORDING_ENABLED"
	// EnvDBPath selects the SQLite recording database path.
	EnvDBPath = "GCS_RECORDING_DB_PATH"
	// EnvMaxTotalBytes overrides the aggregate live-page byte bound.
	EnvMaxTotalBytes = "GCS_RECORDING_MAX_TOTAL_BYTES"
	// EnvMaxTotalAge overrides the aggregate recording age bound.
	EnvMaxTotalAge = "GCS_RECORDING_MAX_TOTAL_AGE"
)

// DefaultDBPath is used when EnvDBPath is unset or empty.
const DefaultDBPath = "./recordings.db"

// ResolveEnabled enables recording only for an explicitly set true value.
func ResolveEnabled(value string, set bool) bool {
	return set && strings.EqualFold(value, "true")
}

// ResolveMaxTotalBytes parses an aggregate byte bound, or returns the default.
func ResolveMaxTotalBytes(value string, set bool) int64 {
	if !set || value == "" {
		return DefaultMaxTotalBytes
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return DefaultMaxTotalBytes
	}
	return parsed
}

// ResolveMaxTotalAge parses a Go duration, or returns the default.
func ResolveMaxTotalAge(value string, set bool) time.Duration {
	if !set || value == "" {
		return DefaultMaxTotalAge
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return DefaultMaxTotalAge
	}
	return parsed
}

// ResolveDBPath returns the configured path or DefaultDBPath.
func ResolveDBPath(value string, set bool) string {
	if !set || value == "" {
		return DefaultDBPath
	}

	return value
}
