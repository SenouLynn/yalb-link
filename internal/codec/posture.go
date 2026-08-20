package codec

import "fmt"

const (
	// EnvUDPBind is the MAVLink listen address. Empty disables the socket.
	EnvUDPBind = "GCS_MAVLINK_UDP_BIND"
)

// DefaultUDPBind is where the bridge listens when EnvUDPBind is not set.
const DefaultUDPBind = "0.0.0.0:14550"

// ResolveBind distinguishes an unset variable from an explicitly disabled bind.
func ResolveBind(value string, set bool) string {
	if !set {
		return DefaultUDPBind
	}

	return value
}

// PostureWarning reports that an active MAVLink socket is unauthenticated.
func PostureWarning(bind string) string {
	if bind == "" {
		return ""
	}

	return fmt.Sprintf(
		"MAVLINK UNAUTHENTICATED: listening on %s; any host that can reach this port can inject frames",
		bind)
}
