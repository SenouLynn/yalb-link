package calculator_test

import (
	"testing"

	"yalb.aero/calculator"
)

// TestEveryCuratedPatternIsFullyDescribed holds that a curated workflow carries
// its rationale, required inputs, active drivers, outcome and validity limits,
// which is what makes it a pattern rather than a button.
func TestEveryCuratedPatternIsFullyDescribed(t *testing.T) {
	all := calculator.Patterns()
	if len(all) == 0 {
		t.Fatal("no curated workflows are registered")
	}
	for n := range all {
		t.Run(all[n].ID, func(t *testing.T) { checkPatternDescription(t, all[n]) })
	}
}

func checkPatternDescription(t *testing.T, p calculator.Pattern) {
	t.Helper()
	if p.Name == "" || p.Rationale == "" || p.Outcome == "" {
		t.Errorf("pattern %s is missing a name, rationale or outcome", p.ID)
	}
	if len(p.RequiredInputs) == 0 {
		t.Error("a pattern must say what it needs before it applies")
	}
	if len(p.ValidityLimits) == 0 {
		t.Error("a pattern must say where it stops being applicable")
	}
	if p.Journey == calculator.JourneyUnknown {
		t.Error("a pattern must name its entry point")
	}
	if p.Supported && len(p.ActiveDrivers) != 2 {
		t.Errorf("active drivers = %v, want the two a planform holds", p.ActiveDrivers)
	}
	looked, err := calculator.LookupPattern(p.ID)
	if err != nil {
		t.Fatalf("lookup %s: %v", p.ID, err)
	}
	if looked.Name != p.Name {
		t.Errorf("lookup returned a different pattern: %q and %q", looked.Name, p.Name)
	}
}

// TestPowerFirstIsRecordedAsUnsupported holds that the journey the task defers
// to Task 09 is listed with its absence stated, rather than left out so that
// its absence reads as an oversight.
func TestPowerFirstIsRecordedAsUnsupported(t *testing.T) {
	p, err := calculator.LookupPattern(calculator.PatternPowerFirst)
	if err != nil {
		t.Fatalf("the power-first journey should be registered: %v", err)
	}
	if p.Supported {
		t.Error("no power model exists, so this journey cannot be supported")
	}
	if p.Journey != calculator.JourneyPowerFirst {
		t.Errorf("journey = %v, want power first", p.Journey)
	}
	for _, supported := range calculator.PatternsFor(baseDesign(t)) {
		if supported.ID == calculator.PatternPowerFirst {
			t.Error("an unsupported pattern must never be offered for a design")
		}
	}
}

// TestPatternMetadataCannotBeMutated holds the same guarantee the equation
// registry gives: inspecting a pattern cannot change it for anyone else.
func TestPatternMetadataCannotBeMutated(t *testing.T) {
	first := calculator.Patterns()
	if len(first[0].ValidityLimits) == 0 {
		t.Fatal("the fixture pattern has no validity limits to mutate")
	}
	original := first[0].ValidityLimits[0]
	first[0].ValidityLimits[0] = "mutated"
	first[0].ActiveDrivers[0] = calculator.ParamChordMAC

	second := calculator.Patterns()
	if second[0].ValidityLimits[0] != original {
		t.Error("mutating an inspected pattern changed the registry")
	}
	if second[0].ActiveDrivers[0] == calculator.ParamChordMAC {
		t.Error("mutating an inspected pattern's drivers changed the registry")
	}
}

// TestPatternsForNamesTheActiveDriverPair holds that the curated workflows
// offered for a design follow its actual drivers, and that more than one can
// apply because the same pair serves several ways of arriving at it.
func TestPatternsForNamesTheActiveDriverPair(t *testing.T) {
	spanFirst := calculator.PatternsFor(baseDesign(t))
	if len(spanFirst) != 1 || spanFirst[0].ID != calculator.PatternSpanFirst {
		t.Fatalf("patterns for the span-and-aspect-ratio candidate = %v, want span first",
			patternIDs(spanFirst))
	}

	s := calculator.NewSession("patterns", baseDesign(t))
	mustDo(t, s, calculator.SizeAtStallLimit{
		Hold: calculator.ParamSpanProjected, Scope: calculator.SingleCase(caseNameN1),
	})
	sized := patternIDs(calculator.PatternsFor(s.Design()))
	if !containsString(sized, calculator.PatternMassPerformance) ||
		!containsString(sized, calculator.PatternMassAndSize) {
		t.Errorf("patterns for the span-and-area candidate = %v, want both span-and-area workflows", sized)
	}
	if containsString(sized, calculator.PatternSpanFirst) {
		t.Error("the span-and-aspect-ratio workflow no longer matches this driver pair")
	}
}

func patternIDs(patterns []calculator.Pattern) []string {
	ids := make([]string, 0, len(patterns))
	for n := range patterns {
		ids = append(ids, patterns[n].ID)
	}
	return ids
}
