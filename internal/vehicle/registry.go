package vehicle

// FleetRegistry is the durable membership set behind fleet:active.
//
// Forward definition. Tier 5 implements it over Redis (SADD/SREM on
// fleet:active); Tier 4 defines it so the fold's consumers can be written and
// tested against the interface rather than against Redis. It is deliberately
// tiny: membership is the only thing the fold produces that must outlive the
// process, and the snapshot itself is written by a different path.
type FleetRegistry interface {
	// AddVehicle records a vehicle as active. Called on VEHICLE_DISCOVERED and
	// VEHICLE_RECOVERED; it must be idempotent, because a vehicle recovering
	// from a link flap will be added again.
	AddVehicle(sysID uint8) error
	// RemoveVehicle drops a vehicle from the active set. Called on
	// VEHICLE_LOST; also idempotent.
	RemoveVehicle(sysID uint8) error
}

// NopFleetRegistry satisfies FleetRegistry and does nothing.
//
// The zero value is usable. Tier 4 has no Redis, and a test that wants to
// prove the fold's event sequence should not need one.
type NopFleetRegistry struct{}

var _ FleetRegistry = NopFleetRegistry{}

// AddVehicle does nothing and always succeeds.
func (NopFleetRegistry) AddVehicle(uint8) error { return nil }

// RemoveVehicle does nothing and always succeeds.
func (NopFleetRegistry) RemoveVehicle(uint8) error { return nil }
