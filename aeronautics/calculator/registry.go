package calculator

import (
	"errors"
	"sort"
)

// SourceKind separates a method ported from the primary reference from
// supplementary identities and from synthetic values chosen for testing. A
// supported subset of the book must never read as the whole book.
type SourceKind uint8

const (
	// SourceBook is a method ported from the CODE Lab Aircraft Design book.
	SourceBook SourceKind = iota
	// SourceSupplementary is a standard identity taken from another cited
	// reference, used where the book assumes it rather than deriving it.
	SourceSupplementary
	// SourceDerived is an algebraic inversion of an implemented equation, with
	// no independent source of its own.
	SourceDerived
)

var sourceKindNames = [...]string{
	SourceBook:          "book",
	SourceSupplementary: "supplementary",
	SourceDerived:       "derived",
}

// String returns the source kind's lowercase name.
func (k SourceKind) String() string {
	if int(k) < len(sourceKindNames) {
		return sourceKindNames[k]
	}
	return "unknown"
}

// Source records where an equation comes from and how it was adapted. Accessed
// is the date the reference was read, recorded as a literal at authoring time:
// the core must not reach the clock, so this is never "now".
type Source struct {
	Title            string
	URL              string
	Section          string
	Accessed         string
	UpstreamRevision string
	OriginalUnits    string
	Adaptation       string
	Kind             SourceKind
}

// Port names one input or output of an equation and fixes its dimension.
type Port struct {
	Name        string
	Description string
	Dimension   Dimension
}

// Equation is the immutable definition of an implemented calculation. Lookup
// returns a deep copy, so inspecting metadata cannot mutate the registry.
type Equation struct {
	ID          string
	Revision    string
	Expression  string
	Inputs      []Port
	Assumptions []string
	Source      Source
	Output      Port
}

func (e Equation) clone() Equation {
	c := e
	c.Inputs = append([]Port(nil), e.Inputs...)
	c.Assumptions = append([]string(nil), e.Assumptions...)
	return c
}

// Substitution is one actual input value used by an evaluation, recorded in the
// order the equation consumed it.
type Substitution struct {
	Name  string
	Value Quantity
}

// Trace is the inspectable record of a single evaluation: which equation at
// which revision, the expression, the values actually substituted, and the
// result. It contains no timestamp and performs no I/O.
type Trace struct {
	EquationID    string
	Revision      string
	Expression    string
	Substitutions []Substitution
	Result        Quantity
}

// Substitution returns the value recorded for the named input.
func (t Trace) Substitution(name string) (Quantity, bool) {
	for _, s := range t.Substitutions {
		if s.Name == name {
			return s.Value, true
		}
	}
	return Quantity{}, false
}

// Result pairs an evaluated value with the trace that produced it.
type Result struct {
	Trace Trace
	Value Quantity
}

// ErrUnknownEquation reports a lookup for an ID the registry does not define.
var ErrUnknownEquation = errors.New("unknown equation")

// Lookup returns the definition of the equation with the given ID. The returned
// Equation is a deep copy: mutating it does not affect the registry.
func Lookup(id string) (Equation, error) {
	eq, ok := registry[id]
	if !ok {
		return Equation{}, ErrUnknownEquation
	}
	return eq.clone(), nil
}

// EquationIDs returns every registered equation ID in sorted order.
func EquationIDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
