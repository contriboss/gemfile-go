package gemfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test constants
const (
	testGroup = "test"
)

func TestGemspecIntegration(t *testing.T) {
	// Create a temporary directory for our test
	tmpDir := t.TempDir()

	// Create a test gemspec file
	gemspecContent := `
Gem::Specification.new do |spec|
  spec.name = "integration_test_gem"
  spec.version = "2.5.0"
  spec.summary = "Integration test gem"
  spec.authors = ["Test Dev"]
  spec.email = ["test@integration.com"]

  spec.add_runtime_dependency "rails", "~> 7.0"
  spec.add_runtime_dependency "pg", ">= 1.0"

  spec.add_development_dependency "rspec", "~> 3.12"
  spec.add_development_dependency "pry"
end
`
	gemspecPath := filepath.Join(tmpDir, "integration_test_gem.gemspec")
	err := os.WriteFile(gemspecPath, []byte(gemspecContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test gemspec: %v", err)
	}

	// Create a test Gemfile with gemspec directive
	gemfileContent := `source 'https://rubygems.org'

ruby '3.1.0'

gemspec

gem 'redis', '~> 5.0'

group :production do
  gem 'sidekiq', '~> 7.0'
end
`
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err = os.WriteFile(gemfilePath, []byte(gemfileContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test Gemfile: %v", err)
	}

	// Parse the Gemfile
	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Verify gemspec was parsed
	if len(parsed.Gemspecs) != 1 {
		t.Errorf("Expected 1 gemspec, got %d", len(parsed.Gemspecs))
	}

	// Verify dependencies from gemspec were loaded
	expectedDeps := map[string]struct {
		constraints []string
		groups      []string
	}{
		"integration_test_gem": {
			constraints: []string{},
			groups:      []string{"default"},
		},
		"rails": {
			constraints: []string{"~> 7.0"},
			groups:      []string{"default"},
		},
		"pg": {
			constraints: []string{">= 1.0"},
			groups:      []string{"default"},
		},
		"rspec": {
			constraints: []string{"~> 3.12"},
			groups:      []string{"development"},
		},
		"pry": {
			constraints: []string{},
			groups:      []string{"development"},
		},
		"redis": {
			constraints: []string{"~> 5.0"},
			groups:      []string{"default"},
		},
		"sidekiq": {
			constraints: []string{"~> 7.0"},
			groups:      []string{"production"},
		},
	}

	// Check all dependencies
	for _, dep := range parsed.Dependencies {
		expected, ok := expectedDeps[dep.Name]
		if !ok {
			t.Errorf("Unexpected dependency: %s", dep.Name)
			continue
		}

		// Check constraints
		if len(dep.Constraints) != len(expected.constraints) {
			t.Errorf("Gem %s: expected %d constraints, got %d", dep.Name, len(expected.constraints), len(dep.Constraints))
		} else {
			for i, constraint := range dep.Constraints {
				if constraint != expected.constraints[i] {
					t.Errorf("Gem %s: expected constraint %s, got %s", dep.Name, expected.constraints[i], constraint)
				}
			}
		}

		// Check groups
		if len(dep.Groups) != len(expected.groups) {
			t.Errorf("Gem %s: expected groups %v, got %v", dep.Name, expected.groups, dep.Groups)
		} else {
			for i, group := range dep.Groups {
				if group != expected.groups[i] {
					t.Errorf("Gem %s: expected group %s, got %s", dep.Name, expected.groups[i], group)
				}
			}
		}
	}

	// Verify we got all expected dependencies
	if len(parsed.Dependencies) != len(expectedDeps) {
		t.Errorf("Expected %d dependencies, got %d", len(expectedDeps), len(parsed.Dependencies))
	}
}

func TestGemspecWithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectory for gems
	gemsDir := filepath.Join(tmpDir, "gems")
	err := os.Mkdir(gemsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create gems directory: %v", err)
	}

	// Create a gemspec in the subdirectory
	gemspecContent := `
Gem::Specification.new do |spec|
  spec.name = "custom_gem"
  spec.version = "1.0.0"
  spec.add_runtime_dependency "thor", "~> 1.2"
  spec.add_development_dependency "minitest", "~> 5.0"
end
`
	gemspecPath := filepath.Join(gemsDir, "custom_gem.gemspec")
	err = os.WriteFile(gemspecPath, []byte(gemspecContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create gemspec: %v", err)
	}

	// Create another gemspec to test name filtering
	anotherGemspecContent := `
Gem::Specification.new do |spec|
  spec.name = "another_custom_gem"
  spec.version = "2.0.0"
end
`
	anotherGemspecPath := filepath.Join(gemsDir, "another_custom_gem.gemspec")
	err = os.WriteFile(anotherGemspecPath, []byte(anotherGemspecContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create second gemspec: %v", err)
	}

	// Test with path and name options
	t.Run("gemspec with path and name", func(t *testing.T) {
		gemfileContent := `source 'https://rubygems.org'

gemspec path: "gems", name: "custom_gem", development_group: :test
`
		gemfilePath := filepath.Join(tmpDir, "Gemfile.path_name")
		err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create Gemfile: %v", err)
		}

		parser := NewGemfileParser(gemfilePath)
		parsed, err := parser.Parse()
		if err != nil {
			t.Fatalf("Failed to parse Gemfile: %v", err)
		}

		// Check gemspec reference
		if len(parsed.Gemspecs) != 1 {
			t.Errorf("Expected 1 gemspec, got %d", len(parsed.Gemspecs))
		} else {
			gemspecRef := parsed.Gemspecs[0]
			if gemspecRef.Path != "gems" {
				t.Errorf("Expected path 'gems', got %s", gemspecRef.Path)
			}
			if gemspecRef.Name != "custom_gem" {
				t.Errorf("Expected name 'custom_gem', got %s", gemspecRef.Name)
			}
			if gemspecRef.DevelopmentGroup != testGroup {
				t.Errorf("Expected development_group '%s', got %s", testGroup, gemspecRef.DevelopmentGroup)
			}
		}

		// Check that dependencies were loaded correctly
		var foundMinitest bool
		for _, dep := range parsed.Dependencies {
			if dep.Name == "minitest" {
				foundMinitest = true
				// Should be in 'test' group as specified by development_group option
				if len(dep.Groups) != 1 || dep.Groups[0] != testGroup {
					t.Errorf("Expected minitest to be in '%s' group, got %v", testGroup, dep.Groups)
				}
			}
		}
		if !foundMinitest {
			t.Error("Expected to find minitest in dependencies")
		}
	})
}

func TestWriteGemfileWithGemspec(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "Generated_Gemfile")

	// Create a ParsedGemfile with gemspec
	parsed := &ParsedGemfile{
		Sources: []Source{
			{Type: "rubygems", URL: "https://rubygems.org"},
		},
		RubyVersion: "3.2.0",
		Gemspecs: []GemspecReference{
			{
				Path:             ".",
				DevelopmentGroup: "development",
				Glob:             "{,*,*/*}.gemspec",
			},
		},
		Dependencies: []GemDependency{
			{
				Name:        "rails",
				Constraints: []string{"~> 7.1"},
				Groups:      []string{"default"},
			},
			{
				Name:   "pry",
				Groups: []string{"development"},
			},
		},
	}

	// Write the Gemfile
	err := WriteGemfile(outputPath, parsed)
	if err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Read and verify the written file
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read generated Gemfile: %v", err)
	}

	contentStr := string(content)

	// Check for gemspec directive
	if !containsLine(contentStr, "gemspec") {
		t.Error("Generated Gemfile should contain 'gemspec' directive")
	}

	// Check for other elements
	if !containsLine(contentStr, "source 'https://rubygems.org'") {
		t.Error("Generated Gemfile should contain source declaration")
	}

	if !containsLine(contentStr, "ruby '3.2.0'") {
		t.Error("Generated Gemfile should contain ruby version")
	}

	if !containsLine(contentStr, "gem 'rails', '~> 7.1'") {
		t.Error("Generated Gemfile should contain rails gem")
	}
}

func TestWriteGemfileWithGemspecOptions(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "Gemfile_with_options")

	// Create a ParsedGemfile with gemspec that has options
	parsed := &ParsedGemfile{
		Sources: []Source{
			{Type: "rubygems", URL: "https://rubygems.org"},
		},
		Gemspecs: []GemspecReference{
			{
				Path:             "components/payment",
				Name:             "payment_gem",
				DevelopmentGroup: "ci",
				Glob:             "*.gemspec",
			},
		},
	}

	// Write the Gemfile
	err := WriteGemfile(outputPath, parsed)
	if err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Read and verify the written file
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read generated Gemfile: %v", err)
	}

	contentStr := string(content)

	// Check for gemspec directive with options
	if !containsSubstring(contentStr, "gemspec") {
		t.Error("Generated Gemfile should contain 'gemspec' directive")
	}

	// Should contain the options
	expectedPatterns := []string{
		"path: 'components/payment'",
		"name: 'payment_gem'",
		"development_group: :ci",
		"glob: '*.gemspec'",
	}

	for _, pattern := range expectedPatterns {
		if !containsSubstring(contentStr, pattern) {
			t.Errorf("Generated Gemfile should contain '%s'", pattern)
		}
	}
}

// Helper function to check if content contains a line
func containsLine(content, line string) bool {
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(line) {
			return true
		}
	}
	return false
}

// Helper function to check if content contains a substring
func containsSubstring(content, substr string) bool {
	return strings.Contains(content, substr)
}
