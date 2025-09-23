package gemfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGemfileParser(t *testing.T) {
	// Create a test Gemfile
	testGemfile := `# Test Gemfile
source 'https://rubygems.org'

ruby '3.2.0'

gem 'rails', '~> 7.0'
gem 'puma', '>= 5.0', '< 7.0'
gem 'bootsnap', require: false

group :development, :test do
  gem 'debug'
  gem 'fabrication'
end

group :development do
  gem 'listen'
  gem 'rubocop', require: false
end

gem 'state_machines', github: 'state-machines/state_machines', branch: 'master'
gem 'my_local_gem', path: '../local_gem'
`

	// Write to temp file
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err := os.WriteFile(gemfilePath, []byte(testGemfile), 0644)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	// Parse the Gemfile
	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Test source parsing
	if len(parsed.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(parsed.Sources))
	} else {
		source := parsed.Sources[0]
		if source.Type != "rubygems" {
			t.Errorf("Expected source type 'rubygems', got %s", source.Type)
		}
		if source.URL != "https://rubygems.org" {
			t.Errorf("Expected source URL 'https://rubygems.org', got %s", source.URL)
		}
	}

	// Test ruby version parsing
	if parsed.RubyVersion != "3.2.0" {
		t.Errorf("Expected ruby version '3.2.0', got %s", parsed.RubyVersion)
	}

	// Test gem parsing
	expectedGems := map[string]struct {
		constraints []string
		groups      []string
		sourceType  string
		requireVal  *string
	}{
		"rails": {
			constraints: []string{"~> 7.0"},
			groups:      []string{"default"},
		},
		"puma": {
			constraints: []string{">= 5.0", "< 7.0"},
			groups:      []string{"default"},
		},
		"bootsnap": {
			constraints: []string{},
			groups:      []string{"default"},
			requireVal:  stringPtr(""),
		},
		"debug": {
			constraints: []string{},
			groups:      []string{"development", "test"},
		},
		"fabrication": {
			constraints: []string{},
			groups:      []string{"development", "test"},
		},
		"listen": {
			constraints: []string{},
			groups:      []string{"development"},
		},
		"rubocop": {
			constraints: []string{},
			groups:      []string{"development"},
			requireVal:  stringPtr(""),
		},
		"state_machines": {
			constraints: []string{},
			groups:      []string{"default"},
			sourceType:  "git",
		},
		"my_local_gem": {
			constraints: []string{},
			groups:      []string{"default"},
			sourceType:  "path",
		},
	}

	if len(parsed.Dependencies) != len(expectedGems) {
		t.Errorf("Expected %d gems, got %d", len(expectedGems), len(parsed.Dependencies))
	}

	for _, dep := range parsed.Dependencies {
		expected, exists := expectedGems[dep.Name]
		if !exists {
			t.Errorf("Unexpected gem: %s", dep.Name)
			continue
		}

		// Check constraints
		if len(dep.Constraints) != len(expected.constraints) {
			t.Errorf("Gem %s: expected %d constraints, got %d",
				dep.Name, len(expected.constraints), len(dep.Constraints))
		} else {
			for i, constraint := range expected.constraints {
				if dep.Constraints[i] != constraint {
					t.Errorf("Gem %s: expected constraint %s, got %s",
						dep.Name, constraint, dep.Constraints[i])
				}
			}
		}

		// Check groups
		if len(dep.Groups) != len(expected.groups) {
			t.Errorf("Gem %s: expected %d groups, got %d",
				dep.Name, len(expected.groups), len(dep.Groups))
		} else {
			for i, group := range expected.groups {
				if dep.Groups[i] != group {
					t.Errorf("Gem %s: expected group %s, got %s",
						dep.Name, group, dep.Groups[i])
				}
			}
		}

		// Check source type
		if expected.sourceType != "" {
			if dep.Source == nil {
				t.Errorf("Gem %s: expected source type %s, got nil",
					dep.Name, expected.sourceType)
			} else if dep.Source.Type != expected.sourceType {
				t.Errorf("Gem %s: expected source type %s, got %s",
					dep.Name, expected.sourceType, dep.Source.Type)
			}
		}

		// Check require option
		if expected.requireVal != nil {
			if dep.Require == nil {
				t.Errorf("Gem %s: expected require %s, got nil",
					dep.Name, *expected.requireVal)
			} else if *dep.Require != *expected.requireVal {
				t.Errorf("Gem %s: expected require %s, got %s",
					dep.Name, *expected.requireVal, *dep.Require)
			}
		}
	}
}

func TestGemfileParserSimple(t *testing.T) {
	simpleGemfile := `gem 'rails'
gem 'puma', '~> 5.0'`

	// Write to temp file
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err := os.WriteFile(gemfilePath, []byte(simpleGemfile), 0644)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	// Parse the Gemfile
	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Should parse 2 gems
	if len(parsed.Dependencies) != 2 {
		t.Errorf("Expected 2 gems, got %d", len(parsed.Dependencies))
	}

	// Check rails (no constraints)
	rails := findGem(parsed.Dependencies, "rails")
	if rails == nil {
		t.Error("Rails gem not found")
	} else if len(rails.Constraints) != 0 {
		t.Errorf("Expected rails to have 0 constraints, got %d", len(rails.Constraints))
	}

	// Check puma (one constraint)
	puma := findGem(parsed.Dependencies, "puma")
	if puma == nil {
		t.Error("Puma gem not found")
	} else if len(puma.Constraints) != 1 || puma.Constraints[0] != "~> 5.0" {
		t.Errorf("Expected puma constraint '~> 5.0', got %v", puma.Constraints)
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func findGem(deps []GemDependency, name string) *GemDependency {
	for _, dep := range deps {
		if dep.Name == name {
			return &dep
		}
	}
	return nil
}
