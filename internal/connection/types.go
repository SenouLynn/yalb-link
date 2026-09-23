// Package connection owns local acquisition paths, independently of vehicle liveness.
package connection

import (
	"context"
	"errors"
)

// Device is one OS-reported serial candidate or the configured UDP endpoint.
// Its ID is snapshot identity, not a persistent hardware matching key.
type Device struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Path         string `json:"path"`
	Description  string `json:"description,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	VID          string `json:"vid,omitempty"`
	PID          string `json:"pid,omitempty"`
}

// Settings selects the serial baud rate; framing is 8N1. UDP ignores it.
type Settings struct {
	BaudRate int `json:"baud_rate"`
}

// VehicleKey identifies a MAVLink sender, independently of its transport.
type VehicleKey struct {
	SystemID    uint8 `json:"system_id"`
	ComponentID uint8 `json:"component_id"`
}

// Status separates acquisition evidence from heartbeat and reading freshness.
// VehicleKeys is current route attribution; timestamps remain historical after release.
type Status struct {
	ID            string       `json:"id"`
	Device        Device       `json:"device"`
	Settings      Settings     `json:"settings"`
	State         string       `json:"state"`
	DetailedError string       `json:"detailed_error,omitempty"`
	ErrorCode     string       `json:"error_code,omitempty"`
	OpenedAtMs    int64        `json:"opened_at_ms"`
	LastFrameAtMs int64        `json:"last_frame_at_ms"`
	VehicleKeys   []VehicleKey `json:"vehicle_keys"`
}

// Connection states describe observed port and frame evidence.
const (
	Idle          = "IDLE"
	Opening       = "OPENING"
	Awaiting      = "OPEN_AWAITING_TRAFFIC"
	Reporting     = "REPORTING"
	Interrupted   = "INTERRUPTED"
	AccessFailed  = "ACCESS_FAILED"
	DeviceMissing = "DEVICE_MISSING"
	DeviceLost    = "DEVICE_LOST"
	Released      = "RELEASED"
	// An I/O failure alone does not establish physical removal.
	TransportFailed = "TRANSPORT_FAILED"
	// More than one inventory device matches a saved profile's identity; none
	// is chosen automatically (ADR 0006: "ambiguous identity requires selection").
	Ambiguous = "AMBIGUOUS"
)

// Request errors preserve the distinction between absence and access failure.
var (
	ErrMissing  = errors.New("device is not in the current inventory")
	ErrConflict = errors.New("device is already open with different settings")
	ErrSettings = errors.New("baud_rate must be between 1 and 4000000")
	ErrClosed   = errors.New("connection manager is closed")
)

// Inventory returns current candidates, without opening any device.
type Inventory interface {
	List(context.Context) ([]Device, error)
}
