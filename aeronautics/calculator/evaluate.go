package calculator

// evaluation accumulates the substitutions and issues of one equation
// evaluation. It exists so that a trace cannot drift from the values actually
// used: every input reaches the arithmetic through a recording method.
type evaluation struct {
	eq     Equation
	subs   []Substitution
	issues Issues
}

// newEvaluation starts an evaluation of the registered equation id.
// TestRegistryDefinesEveryEvaluatedEquation asserts that each id used here is
// registered, so this does not panic on a missing definition.
func newEvaluation(id string) *evaluation {
	eq := registry[id]
	return &evaluation{eq: eq.clone(), subs: make([]Substitution, 0, len(eq.Inputs))}
}

func (e *evaluation) add(field string, kind IssueKind, detail string) {
	e.issues = append(e.issues, Issue{Field: field, Kind: kind, Detail: detail})
}

func (e *evaluation) port(name string) (Port, bool) {
	for _, p := range e.eq.Inputs {
		if p.Name == name {
			return p, true
		}
	}
	return Port{}, false
}

// quantity validates a dimensional input, records the substitution, and returns
// its SI value. A failed check returns zero and records an issue; the caller
// continues so that one pass can report every bad field.
func (e *evaluation) quantity(name string, q Quantity) float64 {
	port, ok := e.port(name)
	if !ok {
		e.add(name, IssueInvalid, "input is not declared by equation "+e.eq.ID)
		return 0
	}
	if q.dim != port.Dimension {
		if q.dim == Dimensionless && q.si == 0 {
			e.add(name, IssueMissing, "a "+port.Dimension.String()+" value is required")
			return 0
		}
		e.add(name, IssueInvalid, "expected a "+port.Dimension.String()+" value but received "+q.dim.String())
		return 0
	}
	if !isFinite(q.si) {
		e.add(name, IssueInvalid, "value is not finite")
		return 0
	}
	e.subs = append(e.subs, Substitution{Name: name, Value: q})
	e.requirePositive(name, q.si, port.Dimension.String())
	return q.si
}

// signedQuantity validates a dimensional input whose sign or zero value is
// meaningful — an angle, a spanwise station, a chordwise offset — and records
// the substitution without applying requirePositive. Range checks for such an
// input belong to the equation that consumes it, because the acceptable range
// is a property of the relationship rather than of the dimension.
func (e *evaluation) signedQuantity(name string, q Quantity) float64 {
	port, ok := e.port(name)
	if !ok {
		e.add(name, IssueInvalid, "input is not declared by equation "+e.eq.ID)
		return 0
	}
	if q.dim != port.Dimension {
		if q.dim == Dimensionless && q.si == 0 {
			e.add(name, IssueMissing, "a "+port.Dimension.String()+" value is required")
			return 0
		}
		e.add(name, IssueInvalid, "expected a "+port.Dimension.String()+" value but received "+q.dim.String())
		return 0
	}
	if !isFinite(q.si) {
		e.add(name, IssueInvalid, "value is not finite")
		return 0
	}
	e.subs = append(e.subs, Substitution{Name: name, Value: q})
	return q.si
}

// requireWithin rejects a recorded value outside the closed range the consuming
// equation supports. It reports IssueUnsupported rather than IssueInvalid when
// the value is well formed but outside the implemented model.
func (e *evaluation) requireWithin(name string, si, low, high float64, unit, detail string) {
	if si >= low && si <= high {
		return
	}
	e.add(name, IssueUnsupported, detail+"; received "+formatFloat(si)+" "+unit)
}

// scalar validates and records a dimensionless input such as load factor.
func (e *evaluation) scalar(name string, v float64) float64 {
	if _, ok := e.port(name); !ok {
		e.add(name, IssueInvalid, "input is not declared by equation "+e.eq.ID)
		return 0
	}
	if !isFinite(v) {
		e.add(name, IssueInvalid, "value is not finite")
		return 0
	}
	e.subs = append(e.subs, Substitution{Name: name, Value: Quantity{si: v, dim: Dimensionless}})
	e.requirePositive(name, v, "")
	return v
}

// scalarSigned validates and records a dimensionless input whose zero or
// negative value is meaningful, such as a chord fraction of 0 at the leading
// edge. Range checks belong to the consuming equation.
func (e *evaluation) scalarSigned(name string, v float64) float64 {
	if _, ok := e.port(name); !ok {
		e.add(name, IssueInvalid, "input is not declared by equation "+e.eq.ID)
		return 0
	}
	if !isFinite(v) {
		e.add(name, IssueInvalid, "value is not finite")
		return 0
	}
	e.subs = append(e.subs, Substitution{Name: name, Value: Quantity{si: v, dim: Dimensionless}})
	return v
}

// requirePositive rejects zero and negative values. Every input in this
// equation set is a positive physical value or a denominator, so this is
// applied uniformly; that is also how zero divisors are caught before dividing.
func (e *evaluation) requirePositive(name string, si float64, unit string) {
	if si > 0 {
		return
	}
	suffix := ""
	if unit != "" {
		suffix = " " + unit
	}
	e.add(name, IssueInvalid, "must be greater than zero, received "+formatFloat(si)+suffix)
}

// finishSigned completes an evaluation whose result may legitimately be zero or
// negative, such as a leading-edge sweep angle on an unswept or forward-swept
// wing. It applies the same suppression and finiteness rules as finish and
// differs only in that it makes no sign demand of the result.
func (e *evaluation) finishSigned(si float64) (Result, error) {
	if len(e.issues) > 0 {
		return Result{}, e.issues
	}
	if !isFinite(si) {
		return Result{}, Issues{{
			Field:  e.eq.Output.Name,
			Kind:   IssueInvalid,
			Detail: "result is not finite; the inputs overflow the model's range",
		}}
	}
	return e.complete(si), nil
}

// finish validates the computed result and returns it with its trace. Any issue
// recorded during input handling suppresses the result entirely, so a caller
// never receives a number derived from a rejected field.
func (e *evaluation) finish(si float64) (Result, error) {
	if len(e.issues) > 0 {
		return Result{}, e.issues
	}
	out := e.eq.Output
	if !isFinite(si) {
		return Result{}, Issues{{
			Field:  out.Name,
			Kind:   IssueInvalid,
			Detail: "result is not finite; the inputs overflow the model's range",
		}}
	}
	if si <= 0 {
		return Result{}, Issues{{
			Field:  out.Name,
			Kind:   IssueInvalid,
			Detail: "result underflowed to " + formatFloat(si) + " " + out.Dimension.String(),
		}}
	}
	return e.complete(si), nil
}

// complete builds the result and its trace for an already validated value.
func (e *evaluation) complete(si float64) Result {
	out := e.eq.Output
	return Result{
		Value: Quantity{si: si, dim: out.Dimension},
		Trace: Trace{
			EquationID: e.eq.ID,
			Revision:   e.eq.Revision,
			Expression: e.eq.Expression,
			// Ownership transfer: newEvaluation allocates subs per evaluation and
			// nothing retains e afterwards, so the trace takes the slice directly.
			// TestEvaluationDoesNotMutateCallerData guards this if evaluation is
			// ever pooled or reused, which is when a copy would become necessary.
			Substitutions: e.subs,
			Result:        Quantity{si: si, dim: out.Dimension},
		},
	}
}
