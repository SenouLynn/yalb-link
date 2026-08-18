package codec

import "fmt"

// Environment variables that decide the inbound path's trust posture.
const (
	// EnvUDPBind is the MAVLink listen address. Empty disables the socket.
	EnvUDPBind = "GCS_MAVLINK_UDP_BIND"
	// EnvSigningKey is the MAVLink 2 signing key. Empty means unauthenticated.
	EnvSigningKey = "GCS_MAVLINK_SIGNING_KEY"
)

// DefaultUDPBind is where the bridge listens when EnvUDPBind is not set.
const DefaultUDPBind = "0.0.0.0:14550"

// ResolveBind applies the unset-versus-empty rule for EnvUDPBind.
//
// Unset means the default address; explicitly empty means no socket at all
// (test mode). Collapsing the two — as os.Getenv alone does — either disables
// the bridge for everyone who never set the variable, or opens a socket in the
// tests that asked for none. Callers pass os.LookupEnv's two results.
func ResolveBind(value string, set bool) string {
	if !set {
		return DefaultUDPBind
	}

	return value
}

// PostureWarning describes the inbound path's trust posture, or returns "" when
// there is nothing to warn about.
//
// The bridge binds a UDP port and accepts frames from anything that can reach
// it. With no signing key there is no authentication either: a host on the same
// network can inject a HEARTBEAT and create a phantom vehicle, or feed
// STATUSTEXT an operator will act on. Inside Docker Compose that is acceptable —
// the socket is on a private bridge network — and it stops being acceptable the
// moment the backend runs on a field box on shared WiFi.
//
// What closes it is Node.InKey: gomavlib validates MAVLink 2 signatures and
// drops unsigned frames, at no per-packet cost in application code. Until then
// the posture is printed at startup, because a default that is silent is a
// default nobody revisits.
func PostureWarning(bind, signingKey string) string {
	if bind == "" {
		return "" // socket disabled; nothing is listening to warn about
	}

	if signingKey != "" {
		return ""
	}

	return fmt.Sprintf(
		"MAVLINK UNAUTHENTICATED: listening on %s with no signing key (%s unset); "+
			"any host that can reach this port can inject frames",
		bind, EnvSigningKey)
}
