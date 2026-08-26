package command

import (
	"strconv"
	"strings"
	"time"
)

const (
	EnvEnabled     = "GCS_COMMANDS_ENABLED"
	DefaultTimeout = 3 * time.Second
)

func ResolveEnabled(value string, set bool) bool {
	if !set {
		return false
	}
	v, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && v
}
