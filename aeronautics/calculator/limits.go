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
	// Detail explains an unknown status. It is empty for a met or unmet check.
	Detail string
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
//
// These limits are plan-view boundaries: a doorway constrains the span the wing
// projects, not the length of the panel skin. A planform solved from panel-plane
// drivers under a nonzero dihedral carries construction lengths, which are
// larger by 1/cos(Gamma), so checking against them would report a wing as too
// wide when it fits. That case returns unknown rather than a wrong answer, and
// names Wing.CheckLimits, which has both planes and can answer it.
func (l PlanformLimits) Check(p Planform) []LimitCheck {
	if p.Plane == OutlinePanelSurface {
		return l.checks(func(name string, limit Quantity) LimitCheck {
			return LimitCheck{
				Name:   name,
				Limit:  limit,
				Status: LimitUnknown,
				Detail: "this planform holds panel-plane construction dimensions under a nonzero " +
					"dihedral; a plan-view limit cannot be checked against them. Use " +
					"Wing.CheckLimits, which reads the projected plane",
			}
		})
	}
	return l.against(p.Span, p.Area)
}

// CheckLimits compares the wing's plan-view dimensions against the supplied
// limits, whichever plane its drivers were given in. This is the form to use
// once a wing exists: Wing.Projected is the plane these limits are about.
func (w Wing) CheckLimits(l PlanformLimits) []LimitCheck {
	return l.against(w.Projected.Span, w.Projected.Area)
}

// against evaluates every supplied limit over one plane's span and area.
func (l PlanformLimits) against(span, area Quantity) []LimitCheck {
	satisfied := map[string]func() bool{
		"maximum span": func() bool { return span.si <= l.MaximumSpan.si },
		"minimum span": func() bool { return span.si >= l.MinimumSpan.si },
		"maximum area": func() bool { return area.si <= l.MaximumArea.si },
	}
	actual := map[string]Quantity{"maximum span": span, "minimum span": span, "maximum area": area}
	return l.checks(func(name string, limit Quantity) LimitCheck {
		check := LimitCheck{Name: name, Limit: limit, Actual: actual[name]}
		switch {
		case !actual[name].supplied():
			check.Status = LimitUnknown
			check.Detail = "the planform does not carry this value"
		case satisfied[name]():
			check.Status = LimitMet
		default:
			check.Status = LimitUnmet
		}
		return check
	})
}

// checks builds one entry per supplied limit, in a stable order, so that both
// the plan-view path and the refusal path report the same set of requirements.
func (l PlanformLimits) checks(build func(name string, limit Quantity) LimitCheck) []LimitCheck {
	checks := make([]LimitCheck, 0, 3)
	for _, requirement := range []struct {
		name  string
		limit Quantity
	}{
		{"maximum span", l.MaximumSpan},
		{"minimum span", l.MinimumSpan},
		{"maximum area", l.MaximumArea},
	} {
		if !requirement.limit.supplied() {
			continue
		}
		checks = append(checks, build(requirement.name, requirement.limit))
	}
	return checks
}
