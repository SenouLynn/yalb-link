// Package recording persists explicitly bounded local flights for replay.
package recording

import "strings"

const (
	// EnvEnabled controls whether recording storage and HTTP routes are active.
	EnvEnabled = "GCS_RECORDING_ENABLED"
	// EnvDBPath selects the SQLite recording database path.
	EnvDBPath = "GCS_RECORDING_DB_PATH"
)

// DefaultDBPath is used when EnvDBPath is unset or empty.
const DefaultDBPath = "./recordings.db"

// ResolveEnabled enables recording only for an explicitly set true value.
func ResolveEnabled(value string, set bool) bool {
	return set && strings.EqualFold(value, "true")
}

// ResolveDBPath returns the configured path or DefaultDBPath.
func ResolveDBPath(value string, set bool) string {
	if !set || value == "" {
		return DefaultDBPath
	}

	return value
}
