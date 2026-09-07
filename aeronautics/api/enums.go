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
	value T
	token string
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
			{calculator.ConfigurationConventionalTail, "conventional-tail"},
			{calculator.ConfigurationVTail, "v-tail"},
			{calculator.ConfigurationFlyingWing, "flying-wing"},
		},
	}
	shapes = enumTable[calculator.PlanformShape]{
		field: "planform shape",
		entries: []enumEntry[calculator.PlanformShape]{
			{calculator.ShapeRectangle, "rectangle"},
			{calculator.ShapeTrapezoid, "trapezoid"},
		},
	}
	areaBases = enumTable[calculator.AreaBasis]{
		field: "area basis",
		entries: []enumEntry[calculator.AreaBasis]{
			{calculator.AreaBasisReferenceTrapezoid, "reference-trapezoid"},
			{calculator.AreaBasisExposedPanels, "exposed-panels"},
		},
	}
	dihedralModes = enumTable[calculator.DihedralMode]{
		field: "dihedral mode",
		entries: []enumEntry[calculator.DihedralMode]{
			{calculator.DihedralHoldPanel, "hold-panel"},
			{calculator.DihedralHoldProjected, "hold-projected"},
		},
	}
	coefficientScopes = enumTable[calculator.CoefficientScope]{
		field: "coefficient scope",
		entries: []enumEntry[calculator.CoefficientScope]{
			{calculator.ScopeAircraft, "aircraft"},
			{calculator.ScopeAirfoilSection, "airfoil-section"},
		},
	}
	priorities = enumTable[calculator.Priority]{
		field: "priority",
		entries: []enumEntry[calculator.Priority]{
			{calculator.PriorityRequired, "required"},
			{calculator.PriorityPreferred, "preferred"},
		},
	}
	evidenceGrades = enumTable[calculator.EvidenceQuality]{
		field: "evidence quality",
		entries: []enumEntry[calculator.EvidenceQuality]{
			{calculator.EvidenceAssumed, "assumed"},
			{calculator.EvidenceMeasured, "measured"},
			{calculator.EvidenceSimulated, "simulated"},
		},
	}
	subjects = enumTable[calculator.RequirementSubject]{
		field: "requirement subject",
		entries: []enumEntry[calculator.RequirementSubject]{
			{calculator.SubjectStallSpeed, "stall-speed"},
			{calculator.SubjectWingArea, "wing-area"},
			{calculator.SubjectSpan, "span"},
			{calculator.SubjectMass, "mass"},
			{calculator.SubjectAspectRatio, "aspect-ratio"},
			{calculator.SubjectWingLoadingMass, "mass-wing-loading"},
		},
	}
	directions = enumTable[calculator.BoundDirection]{
		field: "bound direction",
		entries: []enumEntry[calculator.BoundDirection]{
			{calculator.BoundLower, "minimum"},
			{calculator.BoundUpper, "maximum"},
		},
	}
	limitStatuses = enumTable[calculator.LimitStatus]{
		field: "requirement status",
		entries: []enumEntry[calculator.LimitStatus]{
			{calculator.LimitUnknown, "unknown"},
			{calculator.LimitMet, "met"},
			{calculator.LimitUnmet, "unmet"},
		},
	}
	resultStatuses = enumTable[calculator.ResultStatus]{
		field: "result status",
		entries: []enumEntry[calculator.ResultStatus]{
			{calculator.ResultMissing, "missing"},
			{calculator.ResultComputed, "computed"},
			{calculator.ResultInvalid, "invalid"},
			{calculator.ResultStale, "stale"},
		},
	}
	issueKinds = enumTable[calculator.IssueKind]{
		field: "issue kind",
		entries: []enumEntry[calculator.IssueKind]{
			{calculator.IssueMissing, "missing"},
			{calculator.IssueInvalid, "invalid"},
			{calculator.IssueUnsupported, "unsupported"},
		},
	}
	sourceKinds = enumTable[calculator.SourceKind]{
		field: "source kind",
		entries: []enumEntry[calculator.SourceKind]{
			{calculator.SourceBook, "book"},
			{calculator.SourceSupplementary, "supplementary"},
			{calculator.SourceDerived, "derived"},
		},
	}
	parameterRoles = enumTable[calculator.ParameterRole]{
		field: "parameter role",
		entries: []enumEntry[calculator.ParameterRole]{
			{calculator.RoleDriver, "driver"},
			{calculator.RoleDerived, "derived"},
		},
	}
	solveModes = enumTable[calculator.SolveMode]{
		field: "solve mode",
		entries: []enumEntry[calculator.SolveMode]{
			{calculator.SolveFromSpanAndArea, "span-and-area"},
			{calculator.SolveFromSpanAndAspectRatio, "span-and-aspect-ratio"},
			{calculator.SolveFromAreaAndAspectRatio, "area-and-aspect-ratio"},
			{calculator.SolveFromSpanAndRootChord, "span-and-root-chord"},
			{calculator.SolveFromAreaAndRootChord, "area-and-root-chord"},
			{calculator.SolveFromAspectRatioAndRootChord, "aspect-ratio-and-root-chord"},
		},
	}
	journeys = enumTable[calculator.Journey]{
		field: "journey",
		entries: []enumEntry[calculator.Journey]{
			{calculator.JourneySpanFirst, "span-first"},
			{calculator.JourneyMassAndPerformanceFirst, "mass-and-performance-first"},
			{calculator.JourneyMassAndSizeFirst, "mass-and-size-first"},
			{calculator.JourneyExistingDesign, "existing-design"},
			{calculator.JourneyPowerFirst, "power-first"},
		},
	}
	componentRoles = enumTable[calculator.ComponentRole]{
		field: "component role",
		entries: []enumEntry[calculator.ComponentRole]{
			{calculator.ComponentAirframe, "airframe"},
			{calculator.ComponentBattery, "battery"},
			{calculator.ComponentMotor, "motor"},
			{calculator.ComponentAvionics, "avionics"},
			{calculator.ComponentPayload, "payload"},
			{calculator.ComponentOther, "other"},
		},
	}
	massModes = enumTable[calculator.MassMode]{
		field: "mass mode",
		entries: []enumEntry[calculator.MassMode]{
			{calculator.MassModeEntered, "entered"},
			{calculator.MassModeComponents, "components"},
		},
	}
	viewKinds = enumTable[calculator.ViewKind]{
		field: "view",
		entries: []enumEntry[calculator.ViewKind]{
			{calculator.ViewPlan, "plan-view"},
			{calculator.ViewFront, "front-view"},
			{calculator.ViewSide, "side-view"},
		},
	}
	sketchRoles = enumTable[calculator.SketchRole]{
		field: "sketch role",
		entries: []enumEntry[calculator.SketchRole]{
			{calculator.SketchOutline, "outline"},
			{calculator.SketchCenterline, "centerline"},
			{calculator.SketchAxis, "axis"},
			{calculator.SketchConstruction, "construction"},
			{calculator.SketchReference, "reference"},
		},
	}
	dimensionKinds = enumTable[calculator.DimensionKind]{
		field: "dimension kind",
		entries: []enumEntry[calculator.DimensionKind]{
			{calculator.DimensionLinear, "linear"},
			{calculator.DimensionAngular, "angular"},
		},
	}
	outlinePlanes = enumTable[calculator.OutlinePlane]{
		field: "outline plane",
		entries: []enumEntry[calculator.OutlinePlane]{
			{calculator.OutlinePlanView, "plan-view"},
			{calculator.OutlinePanelSurface, "panel-surface"},
		},
	}
	caseScopes = enumTable[calculator.CaseScopeKind]{
		field: "case scope",
		entries: []enumEntry[calculator.CaseScopeKind]{
			{calculator.CaseScopeAllRequired, "all-required"},
			{calculator.CaseScopeSingle, "single"},
		},
	}
	dimensions = enumTable[calculator.Dimension]{
		field: "dimension",
		entries: []enumEntry[calculator.Dimension]{
			{calculator.Dimensionless, "ratio"},
			{calculator.DimMass, "mass"},
			{calculator.DimForce, "force"},
			{calculator.DimLength, "length"},
			{calculator.DimArea, "area"},
			{calculator.DimSpeed, "speed"},
			{calculator.DimDensity, "density"},
			{calculator.DimForcePerArea, "force-per-area"},
			{calculator.DimMassPerArea, "mass-per-area"},
			{calculator.DimAngle, "angle"},
			{calculator.DimPower, "power"},
			{calculator.DimEnergy, "energy"},
			{calculator.DimDynamicViscosity, "dynamic-viscosity"},
			{calculator.DimMassMoment, "mass-moment"},
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

// sweepDriverKeys are the parameter keys a sweep may move. They are the size
// drivers plus the entered all-up mass, which is a value the builder enters and
// a sweep can move even though it is not a wing parameter.
var sweepDriverKeys = append([]calculator.ParameterKey{calculator.ParamAllUpMass}, driverKeys...)

// parseSweepDriverKey resolves a swept driver key token. Whether the design
// actually holds that driver is the core's answer, not this one: refusing it
// here would duplicate the rule and could disagree with it.
func parseSweepDriverKey(field, token string) (calculator.ParameterKey, *Issue) {
	if token == "" {
		return "", &Issue{Field: field, Kind: "missing", Detail: "name the driver to sweep"}
	}
	for _, key := range sweepDriverKeys {
		if string(key) == token {
			return key, nil
		}
	}
	return "", &Issue{
		Field:  field,
		Kind:   "invalid",
		Detail: token + " cannot be swept; the contract accepts " + joinTokens(sweepDriverKeyTokens()),
	}
}

func sweepDriverKeyTokens() []string {
	tokens := make([]string, 0, len(sweepDriverKeys))
	for _, key := range sweepDriverKeys {
		tokens = append(tokens, string(key))
	}
	return tokens
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
		{Name: "componentRole", Tokens: componentRoles.tokens()},
		{Name: "massMode", Tokens: massModes.tokens()},
		{Name: "viewKind", Tokens: viewKinds.tokens()},
		{Name: "sketchRole", Tokens: sketchRoles.tokens()},
		{Name: "dimensionKind", Tokens: dimensionKinds.tokens()},
		{Name: "outlinePlane", Tokens: outlinePlanes.tokens()},
		{Name: "sweepDriverKey", Tokens: sweepDriverKeyTokens()},
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
