package gemfile

import (
	"testing"
)

func TestRealGemfile(t *testing.T) {
	// Test with the actual Gemfile from examples/benchmark
	gemfilePath := "../examples/benchmark/Gemfile"

	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse real Gemfile: %v", err)
	}

	t.Logf("Parsed %d dependencies", len(parsed.Dependencies))
	t.Logf("Parsed %d sources", len(parsed.Sources))

	// Check some expected gems
	expectedGems := []struct {
		name        string
		constraints []string
		sourceType  string
	}{
		{"railties", []string{"8.1.0.beta1"}, ""},
		{"activerecord", []string{"8.1.0.beta1"}, ""},
		{"no_fly_list", []string{}, ""},
		{"lapsoss", []string{}, ""},
		{"state_machines", []string{}, ""},
		{"capistrano-puma", []string{}, ""},
		{"minitest-reporters-llm", []string{}, ""},
	}

	foundGems := make(map[string]bool)
	for _, dep := range parsed.Dependencies {
		foundGems[dep.Name] = true
	}

	for _, expected := range expectedGems {
		checkExpectedGem(t, expected, foundGems, parsed)
	}

	// Check sources
	if len(parsed.Sources) < 1 {
		t.Error("Expected at least 1 source")
	} else if parsed.Sources[0].URL != "https://rubygems.org" {
		// Should have rubygems.org as first source
		t.Errorf("Expected first source to be rubygems.org, got %s", parsed.Sources[0].URL)
	}

	// Log some examples for debugging
	t.Log("Sample dependencies:")
	for i, dep := range parsed.Dependencies {
		if i >= 5 {
			break
		}
		t.Logf("  %s %v (groups: %v, source: %v)",
			dep.Name, dep.Constraints, dep.Groups, dep.Source)
	}
}

func checkExpectedGem(t *testing.T, expected struct {
	name        string
	constraints []string
	sourceType  string
}, foundGems map[string]bool, parsed *ParsedGemfile) {
	if !foundGems[expected.name] {
		t.Errorf("Expected gem %s not found", expected.name)
		return
	}

	gem := findGem(parsed.Dependencies, expected.name)
	if gem == nil {
		return
	}

	// Check constraints
	if len(gem.Constraints) != len(expected.constraints) {
		t.Errorf("Gem %s: expected %d constraints, got %d (%v)",
			expected.name, len(expected.constraints), len(gem.Constraints), gem.Constraints)
	} else {
		for i, constraint := range expected.constraints {
			if gem.Constraints[i] != constraint {
				t.Errorf("Gem %s: expected constraint %s, got %s",
					expected.name, constraint, gem.Constraints[i])
			}
		}
	}

	// Check source type
	if expected.sourceType != "" {
		if gem.Source == nil {
			t.Errorf("Gem %s: expected source type %s, got nil",
				expected.name, expected.sourceType)
		} else if gem.Source.Type != expected.sourceType {
			t.Errorf("Gem %s: expected source type %s, got %s",
				expected.name, expected.sourceType, gem.Source.Type)
		}
	}
}
