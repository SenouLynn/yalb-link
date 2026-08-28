package command

import (
	"strconv"
	"strings"
	"time"
)

const (
	EnvEnabled     = "GCS_COMMANDS_ENABLED"
	DefaultTimeout = 3 * time.Second

	// DefaultResolutionQuarantine bounds the stale-ACK window that reopens when
	// an operator clears a poisoned key.
	//
	// Deliberately independent of DefaultTimeout. Late-ACK latency is a property
	// of the link, so deriving this from the command timeout would let a tuned
	// timeout silently shrink the protection.
	DefaultResolutionQuarantine = 30 * time.Second

	// OperatorLabel is stamped on every transaction and attestation. It is a
	// fixed local label, not an authenticated identity: nothing on the HTTP
	// surface verifies who the caller is.
	OperatorLabel = "local-operator"
)

func ResolveEnabled(value string, set bool) bool {
	if !set {
		return false
	}
	v, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && v
}
