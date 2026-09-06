package api

import (
	"sort"

	"yalb.aero/calculator"
)

// The wire tokens are defined here rather than taken from the core's String()
// methods on purpose. Those strings are readable names — "whole aircraft",
// "mass wing loading" — meant for a person, and reusing them would tie the wire
// contract to display wording: renaming a label would then silently break every
// client. These tokens are stable identifiers, and
// TestEveryCoreConstantHasAWireToken holds that the tables stay complete, so a
// new core constant cannot reach the boundary without one.
type enumEntry[T comparable] struct {
	token string
	value T
}

// enumTable maps wire tokens onto a core constant set in both directions.
type enumTable[T comparable] struct {
	field   string
	entries []enumEntry[T]
}

// format returns the token for a core value. An unmapped value returns the
// empty string, which TestEveryCoreConstantHasAWireToken makes impossible for
// any constant the core defines.
func (e enumTable[T]) format(value T) string {
	for _, entry := range e.entries {
		if entry.value == value {
			return entry.token
		}
	}
	return ""
}

// parse resolves a token, or reports a missing or unrecognised one against the
// field it was supplied in.
func (e enumTable[T]) parse(field, token string) (T, *Issue) {
	var zero T
	if token == "" {
		return zero, &Issue{
			Field:  field,
			Kind:   "missing",
			Detail: "choose one of " + joinTokens(e.tokens()),
		}
	}
	for _, entry := range e.entries {
		if entry.token == token {
			return entry.value, nil
		}
	}
	return zero, &Issue{
		Field:  field,
		Kind:   "invalid",
		Detail: token + " is not a " + e.field + "; the contract accepts " + joinTokens(e.tokens()),
	}
}

// tokens lists the accepted tokens in table order.
func (e enumTable[T]) tokens() []string {
	out := make([]string, 0, len(e.entries))
	for _, entry := range e.entries {
		out = append(out, entry.token)
	}
	return out
}

func joinTokens(tokens []string) string {
	if len(tokens) == 0 {
		return "nothing"
	}
	out := tokens[0]
	for _, token := range tokens[1:] {
		out += ", " + token
	}
	return out
}

var (
	configurations = enumTable[calculator.Configuration]{
		field: "configuration",
		entries: []enumEntry[calculator.Configuration]{
			{"conventional-tail", calculator.ConfigurationConventionalTail},
			{"v-tail", calculator.ConfigurationVTail},
			{"flying-wing", calculator.ConfigurationFlyingWing},
		},
	}
	shapes = enumTable[calculator.PlanformShape]{
		field: "planform shape",
		entries: []enumEntry[calculator.PlanformShape]{
			{"rectangle", calculator.ShapeRectangle},
			{"trapezoid", calculator.ShapeTrapezoid},
		},
	}
	areaBases = enumTable[calculator.AreaBasis]{
		field: "area basis",
		entries: []enumEntry[calculator.AreaBasis]{
			{"reference-trapezoid", calculator.AreaBasisReferenceTrapezoid},
			{"exposed-panels", calculator.AreaBasisExposedPanels},
		},
	}
	dihedralModes = enumTable[calculator.DihedralMode]{
		field: "dihedral mode",
		entries: []enumEntry[calculator.DihedralMode]{
			{"hold-panel", calculator.DihedralHoldPanel},
			{"hold-projected", calculator.DihedralHoldProjected},
		},
	}
	coefficientScopes = enumTable[calculator.CoefficientScope]{
		field: "coefficient scope",
		entries: []enumEntry[calculator.CoefficientScope]{
			{"aircraft", calculator.ScopeAircraft},
			{"airfoil-section", calculator.ScopeAirfoilSection},
		},
	}
	priorities = enumTable[calculator.Priority]{
		field: "priority",
		entries: []enumEntry[calculator.Priority]{
			{"required", calculator.PriorityRequired},
			{"preferred", calculator.PriorityPreferred},
		},
	}
	evidenceGrades = enumTable[calculator.EvidenceQuality]{
		field: "evidence quality",
		entries: []enumEntry[calculator.EvidenceQuality]{
			{"assumed", calculator.EvidenceAssumed},
			{"measured", calculator.EvidenceMeasured},
			{"simulated", calculator.EvidenceSimulated},
		},
	}
	subjects = enumTable[calculator.RequirementSubject]{
		field: "requirement subject",
		entries: []enumEntry[calculator.RequirementSubject]{
			{"stall-speed", calculator.SubjectStallSpeed},
			{"wing-area", calculator.SubjectWingArea},
			{"span", calculator.SubjectSpan},
			{"mass", calculator.SubjectMass},
			{"aspect-ratio", calculator.SubjectAspectRatio},
			{"mass-wing-loading", calculator.SubjectWingLoadingMass},
		},
	}
	directions = enumTable[calculator.BoundDirection]{
		field: "bound direction",
		entries: []enumEntry[calculator.BoundDirection]{
			{"minimum", calculator.BoundLower},
			{"maximum", calculator.BoundUpper},
		},
	}
	limitStatuses = enumTable[calculator.LimitStatus]{
		field: "requirement status",
		entries: []enumEntry[calculator.LimitStatus]{
			{"unknown", calculator.LimitUnknown},
			{"met", calculator.LimitMet},
			{"unmet", calculator.LimitUnmet},
		},
	}
	resultStatuses = enumTable[calculator.ResultStatus]{
		field: "result status",
		entries: []enumEntry[calculator.ResultStatus]{
			{"missing", calculator.ResultMissing},
			{"computed", calculator.ResultComputed},
			{"invalid", calculator.ResultInvalid},
			{"stale", calculator.ResultStale},
		},
	}
	issueKinds = enumTable[calculator.IssueKind]{
		field: "issue kind",
		entries: []enumEntry[calculator.IssueKind]{
			{"missing", calculator.IssueMissing},
			{"invalid", calculator.IssueInvalid},
			{"unsupported", calculator.IssueUnsupported},
		},
	}
	sourceKinds = enumTable[calculator.SourceKind]{
		field: "source kind",
		entries: []enumEntry[calculator.SourceKind]{
			{"book", calculator.SourceBook},
			{"supplementary", calculator.SourceSupplementary},
			{"derived", calculator.SourceDerived},
		},
	}
	parameterRoles = enumTable[calculator.ParameterRole]{
		field: "parameter role",
		entries: []enumEntry[calculator.ParameterRole]{
			{"driver", calculator.RoleDriver},
			{"derived", calculator.RoleDerived},
		},
	}
	solveModes = enumTable[calculator.SolveMode]{
		field: "solve mode",
		entries: []enumEntry[calculator.SolveMode]{
			{"span-and-area", calculator.SolveFromSpanAndArea},
			{"span-and-aspect-ratio", calculator.SolveFromSpanAndAspectRatio},
			{"area-and-aspect-ratio", calculator.SolveFromAreaAndAspectRatio},
			{"span-and-root-chord", calculator.SolveFromSpanAndRootChord},
			{"area-and-root-chord", calculator.SolveFromAreaAndRootChord},
			{"aspect-ratio-and-root-chord", calculator.SolveFromAspectRatioAndRootChord},
		},
	}
	journeys = enumTable[calculator.Journey]{
		field: "journey",
		entries: []enumEntry[calculator.Journey]{
			{"span-first", calculator.JourneySpanFirst},
			{"mass-and-performance-first", calculator.JourneyMassAndPerformanceFirst},
			{"mass-and-size-first", calculator.JourneyMassAndSizeFirst},
			{"existing-design", calculator.JourneyExistingDesign},
			{"power-first", calculator.JourneyPowerFirst},
		},
	}
	caseScopes = enumTable[calculator.CaseScopeKind]{
		field: "case scope",
		entries: []enumEntry[calculator.CaseScopeKind]{
			{"all-required", calculator.CaseScopeAllRequired},
			{"single", calculator.CaseScopeSingle},
		},
	}
	dimensions = enumTable[calculator.Dimension]{
		field: "dimension",
		entries: []enumEntry[calculator.Dimension]{
			{"ratio", calculator.Dimensionless},
			{"mass", calculator.DimMass},
			{"force", calculator.DimForce},
			{"length", calculator.DimLength},
			{"area", calculator.DimArea},
			{"speed", calculator.DimSpeed},
			{"density", calculator.DimDensity},
			{"force-per-area", calculator.DimForcePerArea},
			{"mass-per-area", calculator.DimMassPerArea},
			{"angle", calculator.DimAngle},
			{"power", calculator.DimPower},
			{"energy", calculator.DimEnergy},
			{"dynamic-viscosity", calculator.DimDynamicViscosity},
		},
	}
)

// driverKeys are the parameter keys a driver command may name. They are the
// core's own stable keys, so the wire does not invent a second naming scheme
// for the same values.
var driverKeys = []calculator.ParameterKey{
	calculator.ParamSpanProjected,
	calculator.ParamSpanPanel,
	calculator.ParamAreaReference,
	calculator.ParamAreaPanel,
	calculator.ParamAspectRatio,
	calculator.ParamChordRoot,
}

// parseDriverKey resolves a driver key token.
func parseDriverKey(field, token string) (calculator.ParameterKey, *Issue) {
	if token == "" {
		return "", &Issue{Field: field, Kind: "missing", Detail: "name a size driver"}
	}
	for _, key := range driverKeys {
		if string(key) == token {
			return key, nil
		}
	}
	names := make([]string, 0, len(driverKeys))
	for _, key := range driverKeys {
		names = append(names, string(key))
	}
	return "", &Issue{
		Field:  field,
		Kind:   "invalid",
		Detail: token + " is not a size driver; the contract accepts " + joinTokens(names),
	}
}

// Vocabularies publishes every enum the contract accepts, so a client can build
// a valid request without reading this package's source or a hand-written
// document that can drift from it.
func Vocabularies() []Vocabulary {
	vocabularies := []Vocabulary{
		{Name: "configuration", Tokens: configurations.tokens()},
		{Name: "planformShape", Tokens: shapes.tokens()},
		{Name: "areaBasis", Tokens: areaBases.tokens()},
		{Name: "dihedralMode", Tokens: dihedralModes.tokens()},
		{Name: "coefficientScope", Tokens: coefficientScopes.tokens()},
		{Name: "priority", Tokens: priorities.tokens()},
		{Name: "evidence", Tokens: evidenceGrades.tokens()},
		{Name: "requirementSubject", Tokens: subjects.tokens()},
		{Name: "boundDirection", Tokens: directions.tokens()},
		{Name: "requirementStatus", Tokens: limitStatuses.tokens()},
		{Name: "resultStatus", Tokens: resultStatuses.tokens()},
		{Name: "issueKind", Tokens: issueKinds.tokens()},
		{Name: "sourceKind", Tokens: sourceKinds.tokens()},
		{Name: "parameterRole", Tokens: parameterRoles.tokens()},
		{Name: "solveMode", Tokens: solveModes.tokens()},
		{Name: "journey", Tokens: journeys.tokens()},
		{Name: "caseScope", Tokens: caseScopes.tokens()},
		{Name: "dimension", Tokens: dimensions.tokens()},
		{Name: "commandKind", Tokens: commandKinds()},
		{Name: "driverKey", Tokens: driverKeyTokens()},
	}
	sort.Slice(vocabularies, func(i, j int) bool { return vocabularies[i].Name < vocabularies[j].Name })
	return vocabularies
}

func driverKeyTokens() []string {
	tokens := make([]string, 0, len(driverKeys))
	for _, key := range driverKeys {
		tokens = append(tokens, string(key))
	}
	return tokens
}
