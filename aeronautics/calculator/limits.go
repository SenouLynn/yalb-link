package calculator

// LimitStatus reports a requirement's outcome separately from whether the
// numbers were computable. A design that satisfies every equation can still
// fail a requirement, and an unsolved value is neither met nor unmet.
type LimitStatus uint8

const (
	// LimitUnknown means the value the limit applies to is not available.
	LimitUnknown LimitStatus = iota
	// LimitMet means the solved value satisfies the limit.
	LimitMet
	// LimitUnmet means the solved value violates the limit.
	LimitUnmet
)

var limitStatusNames = [...]string{
	LimitUnknown: "unknown",
	LimitMet:     "met",
	LimitUnmet:   "unmet",
}

// String returns the status's readable name.
func (s LimitStatus) String() string {
	if int(s) < len(limitStatusNames) {
		return limitStatusNames[s]
	}
	return "unknown"
}

// PlanformLimits are requirements a planform must satisfy. They are a separate
// type from PlanformDrivers on purpose: "span at most 1.4 m" is a boundary, not
// a choice of span, and nothing in this package converts one into the other. A
// builder who decides to use the whole allowance says so by entering that span
// as a driver, and that decision is then visible as a driver in the parameters.
type PlanformLimits struct {
	// MaximumSpan is the largest acceptable projected span, for example a
	// doorway, a car boot or a contest rule.
	MaximumSpan Quantity
	// MinimumSpan is the smallest acceptable projected span.
	MinimumSpan Quantity
	// MaximumArea is the largest acceptable reference area.
	MaximumArea Quantity
}

// LimitCheck is one requirement's outcome against a solved planform.
type LimitCheck struct {
	// Name identifies the requirement.
	Name string
	// Limit is the boundary that was requested.
	Limit Quantity
	// Actual is the solved value it was compared against.
	Actual Quantity
	// Status is met, unmet, or unknown.
	Status LimitStatus
}

// Check compares a solved planform against the supplied limits. Limits that
// were not supplied produce no check; a limit whose value the planform does not
// carry produces an unknown one.
func (l PlanformLimits) Check(p Planform) []LimitCheck {
	checks := make([]LimitCheck, 0, 3)
	add := func(name string, limit, actual Quantity, satisfied func() bool) {
		if !limit.supplied() {
			return
		}
		check := LimitCheck{Name: name, Limit: limit, Actual: actual}
		switch {
		case !actual.supplied():
			check.Status = LimitUnknown
		case satisfied():
			check.Status = LimitMet
		default:
			check.Status = LimitUnmet
		}
		checks = append(checks, check)
	}
	add("maximum span", l.MaximumSpan, p.Span, func() bool { return p.Span.si <= l.MaximumSpan.si })
	add("minimum span", l.MinimumSpan, p.Span, func() bool { return p.Span.si >= l.MinimumSpan.si })
	add("maximum area", l.MaximumArea, p.Area, func() bool { return p.Area.si <= l.MaximumArea.si })
	return checks
}
