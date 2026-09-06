package calculator

// Reynolds returns the Reynolds number at one chord, for the case's density,
// viscosity and the given true airspeed. It is a flow condition and nothing
// more: no polar, lift coefficient or drag value follows from it here.
func Reynolds(fc FlightCase, trueAirspeed, chord Quantity) (Result, error) {
	e := newEvaluation(EqReynolds)
	rho := e.density(fc)
	v := e.quantity(portSpeed.Name, trueAirspeed)
	c := e.quantity(portChord.Name, chord)
	mu := e.viscosity(fc)
	if mu == 0 {
		return e.finish(0)
	}
	return e.finish(rho * v * c / mu)
}

// ReynoldsStation names where a Reynolds number was evaluated. The station
// travels with the value because a single number is not a property of the wing.
type ReynoldsStation uint8

const (
	// StationUnknown is the zero value.
	StationUnknown ReynoldsStation = iota
	// StationRoot is the centerline chord of the reference trapezoid.
	StationRoot
	// StationMAC is the mean aerodynamic chord.
	StationMAC
	// StationTip is the tip chord.
	StationTip
)

var reynoldsStationNames = [...]string{
	StationUnknown: "unknown",
	StationRoot:    "root",
	StationMAC:     "mean aerodynamic chord",
	StationTip:     "tip",
}

// String returns the station's readable name.
func (s ReynoldsStation) String() string {
	if int(s) < len(reynoldsStationNames) {
		return reynoldsStationNames[s]
	}
	return "unknown"
}

// requiredReynoldsStations is what complete coverage means here. The chapter
// says airfoil analysis should use the mean aerodynamic chord; on a tapered RC
// wing the tip can sit in a materially lower Reynolds regime than the MAC, so
// MAC-only coverage is not accepted as covering the wing.
var requiredReynoldsStations = [...]ReynoldsStation{StationRoot, StationMAC, StationTip}

// ReynoldsSample is one station's Reynolds number with the chord it used.
type ReynoldsSample struct {
	// Trace records the evaluation that produced Reynolds.
	Trace Trace
	// Chord is the local chord the number was computed with.
	Chord Quantity
	// Reynolds is the dimensionless result.
	Reynolds float64
	// Station names where on the wing this applies.
	Station ReynoldsStation
}

// ReynoldsCoverage is the set of stations a wing's Reynolds conditions were
// evaluated at, together with the case they belong to. Coverage is explicit so
// that a single MAC number cannot stand in for the whole wing.
type ReynoldsCoverage struct {
	// Samples are the evaluated stations, root to tip.
	Samples []ReynoldsSample
	// Case is the flight condition every sample belongs to.
	Case FlightCase
	// TrueAirspeed is the speed every sample was evaluated at.
	TrueAirspeed Quantity
}

// At returns the sample for a station.
func (c ReynoldsCoverage) At(station ReynoldsStation) (ReynoldsSample, bool) {
	for _, s := range c.Samples {
		if s.Station == station {
			return s, true
		}
	}
	return ReynoldsSample{}, false
}

// MissingStations lists the required stations this coverage does not contain.
// A coverage holding only the MAC reports the root and the tip as missing.
func (c ReynoldsCoverage) MissingStations() []ReynoldsStation {
	var missing []ReynoldsStation
	for _, required := range requiredReynoldsStations {
		if _, ok := c.At(required); !ok {
			missing = append(missing, required)
		}
	}
	return missing
}

// Complete reports whether every required station is covered.
func (c ReynoldsCoverage) Complete() bool { return len(c.MissingStations()) == 0 }

// Range returns the lowest and highest Reynolds number covered. On a tapered
// wing these differ, which is the point of covering more than the MAC.
func (c ReynoldsCoverage) Range() (low, high float64, ok bool) {
	for n, s := range c.Samples {
		if n == 0 || s.Reynolds < low {
			low = s.Reynolds
		}
		if n == 0 || s.Reynolds > high {
			high = s.Reynolds
		}
	}
	return low, high, len(c.Samples) > 0
}

// ReynoldsCoverage evaluates the root, MAC and tip Reynolds numbers for the
// case at one true airspeed.
func (w Wing) ReynoldsCoverage(fc FlightCase, trueAirspeed Quantity) (ReynoldsCoverage, error) {
	rs := &resultSet{}
	coverage := ReynoldsCoverage{Case: fc, TrueAirspeed: trueAirspeed}
	for _, station := range []struct {
		chord Quantity
		at    ReynoldsStation
	}{
		{w.Planform.RootChord, StationRoot},
		{w.Planform.MAC, StationMAC},
		{w.Planform.TipChord, StationTip},
	} {
		result, err := Reynolds(fc, trueAirspeed, station.chord)
		if err != nil {
			// The stations share a case, a speed and a viscosity, so a rejected
			// input fails all three identically. Reporting it once is the whole
			// story; repeating it per station would only pad the issue set.
			if issues, ok := AsIssues(err); ok {
				rs.issues = append(rs.issues, issues...)
			}
			break
		}
		coverage.Samples = append(coverage.Samples, ReynoldsSample{
			Station:  station.at,
			Chord:    station.chord,
			Reynolds: result.Value.SI(),
			Trace:    result.Trace,
		})
	}
	if len(rs.issues) > 0 {
		return ReynoldsCoverage{}, rs.issues
	}
	return coverage, nil
}
