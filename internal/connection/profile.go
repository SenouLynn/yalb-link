package connection

import "errors"

// Profile intents distinguish a saved-but-never-opened target from one the
// operator explicitly connected or released. Only IntentConnected causes
// Manager to attempt automatic acquisition on the next backend start; a
// profile that was never connected (IntentIdle) or was explicitly released
// (IntentReleased) never auto-connects, matching ADR 0006's "an intentional
// disconnect stops automatic reconnection."
const (
	IntentIdle      = "IDLE"
	IntentConnected = "CONNECTED"
	IntentReleased  = "RELEASED"
)

// Profile is a saved, named connection target that survives restarts. Device
// carries the identity/metadata snapshot captured from inventory at save
// time; resolving it again against a later inventory is Manager's concern
// (matchIdentity), not Profile's.
type Profile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Device      Device   `json:"device"`
	Settings    Settings `json:"settings"`
	Intent      string   `json:"intent"`
	CreatedAtMs int64    `json:"created_at_ms"`
	UpdatedAtMs int64    `json:"updated_at_ms"`
}

var (
	// ErrProfileNotFound reports an operation against an id no longer saved.
	ErrProfileNotFound = errors.New("connection profile not found")
	// ErrProfileInvalid reports a profile missing its required name.
	ErrProfileInvalid = errors.New("connection profile requires a name")
	// ErrAmbiguous reports more than one inventory device matching a saved
	// profile's identity; ADR 0006 requires explicit selection over a guess.
	ErrAmbiguous = errors.New("more than one device matches the saved connection profile")
	// ErrProfilesDisabled reports a Manager configured without profile storage.
	ErrProfilesDisabled = errors.New("connection profiles are not configured")
)

// Store persists Profiles durably and independently of Manager's in-memory
// connection state. Implementations must be safe for concurrent use from one
// process and must never leave a partially written document in place of a
// good one (see FileStore).
type Store interface {
	// Load returns every saved profile. A store with nothing saved yet
	// returns (nil, nil); an existing but unreadable, malformed or
	// unsupported-version document returns an error rather than silently
	// treating it as empty.
	Load() ([]Profile, error)
	// Save inserts a profile with an empty ID, or updates the one an existing
	// ID names while preserving its Intent and CreatedAtMs. It returns
	// ErrProfileNotFound when a nonempty ID names no existing profile, and
	// otherwise returns the stored profile including any assigned ID and
	// timestamps.
	Save(Profile) (Profile, error)
	// Delete removes a saved profile; ErrProfileNotFound if id is unknown.
	Delete(id string) error
	// UpdateIntent persists connect/disconnect intent independently of a
	// profile's other fields; ErrProfileNotFound if id is unknown.
	UpdateIntent(id, intent string) error
}

// matchIdentity finds inventory devices whose serial number and VID/PID
// together identify the same physical device as target. A profile missing
// any of these three fields never auto-matches: ADR 0006 requires explicit
// selection rather than silently accepting a device by port path alone, and
// not every adapter exposes a serial number.
func matchIdentity(target Device, candidates []Device) []Device {
	if target.SerialNumber == "" || target.VID == "" || target.PID == "" {
		return nil
	}
	var matches []Device
	for _, d := range candidates {
		if d.Kind == "serial" && d.SerialNumber == target.SerialNumber && d.VID == target.VID && d.PID == target.PID {
			matches = append(matches, d)
		}
	}
	return matches
}
