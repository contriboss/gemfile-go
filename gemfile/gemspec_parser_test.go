package gemfile

import (
	"path/filepath"
	"reflect"
	"testing"
)

// Test constants
const (
	testGemName     = "test_gem"
	testGemHomepage = "https://github.com/example/test_gem"
	testGemRack     = "rack"
	testDevelopment = "development"
)

func TestGemspecParser(t *testing.T) {
	// Test parsing a comprehensive gemspec file
	gemspecPath := filepath.Join("..", "testdata", "test_gem.gemspec")
	parser := NewGemspecParser(gemspecPath)
	gemspec, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse gemspec: %v", err)
	}

	// Test basic metadata
	if gemspec.Name != testGemName {
		t.Errorf("Expected name '%s', got %s", testGemName, gemspec.Name)
	}

	if gemspec.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", gemspec.Version)
	}

	if gemspec.Summary != "A test gem for gemspec parsing" {
		t.Errorf("Expected summary 'A test gem for gemspec parsing', got %s", gemspec.Summary)
	}

	if gemspec.Homepage != testGemHomepage {
		t.Errorf("Expected homepage '%s', got %s", testGemHomepage, gemspec.Homepage)
	}

	if gemspec.License != "MIT" {
		t.Errorf("Expected license 'MIT', got %s", gemspec.License)
	}

	// Test authors
	expectedAuthors := []string{"Test Author", "Another Author"}
	if !reflect.DeepEqual(gemspec.Authors, expectedAuthors) {
		t.Errorf("Expected authors %v, got %v", expectedAuthors, gemspec.Authors)
	}

	// Test emails
	expectedEmails := []string{"test@example.com", "another@example.com"}
	if !reflect.DeepEqual(gemspec.Email, expectedEmails) {
		t.Errorf("Expected emails %v, got %v", expectedEmails, gemspec.Email)
	}

	// Test runtime dependencies
	if len(gemspec.RuntimeDependencies) != 3 {
		t.Errorf("Expected 3 runtime dependencies, got %d", len(gemspec.RuntimeDependencies))
	} else {
		// Check first runtime dependency
		dep := gemspec.RuntimeDependencies[0]
		if dep.Name != testGemRack {
			t.Errorf("Expected first runtime dep to be '%s', got %s", testGemRack, dep.Name)
		}
		if len(dep.Constraints) != 1 || dep.Constraints[0] != "~> 2.0" {
			t.Errorf("Expected rack constraint '~> 2.0', got %v", dep.Constraints)
		}

		// Check second runtime dependency with multiple constraints
		dep = gemspec.RuntimeDependencies[1]
		if dep.Name != "thor" {
			t.Errorf("Expected second runtime dep to be 'thor', got %s", dep.Name)
		}
		if len(dep.Constraints) != 2 {
			t.Errorf("Expected thor to have 2 constraints, got %d", len(dep.Constraints))
		}

		// Check third runtime dependency (using add_dependency)
		dep = gemspec.RuntimeDependencies[2]
		if dep.Name != "json" {
			t.Errorf("Expected third runtime dep to be 'json', got %s", dep.Name)
		}
	}

	// Test development dependencies
	if len(gemspec.DevelopmentDependencies) != 4 {
		t.Errorf("Expected 4 development dependencies, got %d", len(gemspec.DevelopmentDependencies))
	} else {
		// Check rubocop with multiple constraints
		var rubocop *GemDependency
		for _, dep := range gemspec.DevelopmentDependencies {
			if dep.Name == "rubocop" {
				rubocop = &dep
				break
			}
		}
		if rubocop == nil {
			t.Error("Expected to find rubocop in development dependencies")
		} else if len(rubocop.Constraints) != 2 {
			t.Errorf("Expected rubocop to have 2 constraints, got %d", len(rubocop.Constraints))
		}
	}

	// Test required ruby version
	if gemspec.RequiredRubyVersion != ">= 2.6.0" {
		t.Errorf("Expected required ruby version '>= 2.6.0', got %s", gemspec.RequiredRubyVersion)
	}

	// Test metadata
	if len(gemspec.Metadata) != 3 {
		t.Errorf("Expected 3 metadata entries, got %d", len(gemspec.Metadata))
	}
	if gemspec.Metadata["source_code_uri"] != "https://github.com/example/test_gem" {
		t.Errorf("Expected source_code_uri metadata, got %s", gemspec.Metadata["source_code_uri"])
	}
}

func TestParseGemspecDirective(t *testing.T) {
	parser := NewGemfileParser("test.gemfile")

	tests := []struct {
		name     string
		line     string
		expected GemspecReference
	}{
		{
			name: "simple gemspec",
			line: "gemspec",
			expected: GemspecReference{
				Path:             ".",
				DevelopmentGroup: "development",
				Glob:             "{,*,*/*}.gemspec",
			},
		},
		{
			name: "gemspec with path",
			line: `gemspec path: "components/payment"`,
			expected: GemspecReference{
				Path:             "components/payment",
				DevelopmentGroup: "development",
				Glob:             "{,*,*/*}.gemspec",
			},
		},
		{
			name: "gemspec with name",
			line: `gemspec name: "payment_core"`,
			expected: GemspecReference{
				Path:             ".",
				Name:             "payment_core",
				DevelopmentGroup: "development",
				Glob:             "{,*,*/*}.gemspec",
			},
		},
		{
			name: "gemspec with development group",
			line: `gemspec development_group: :ci`,
			expected: GemspecReference{
				Path:             ".",
				DevelopmentGroup: "ci",
				Glob:             "{,*,*/*}.gemspec",
			},
		},
		{
			name: "gemspec with glob",
			line: `gemspec glob: "*.gemspec"`,
			expected: GemspecReference{
				Path:             ".",
				DevelopmentGroup: "development",
				Glob:             "*.gemspec",
			},
		},
		{
			name: "gemspec with multiple options",
			line: `gemspec path: "gems", name: "my_gem", development_group: :test`,
			expected: GemspecReference{
				Path:             "gems",
				Name:             "my_gem",
				DevelopmentGroup: "test",
				Glob:             "{,*,*/*}.gemspec",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseGemspecDirective(tt.line)

			if result.Path != tt.expected.Path {
				t.Errorf("Expected path %s, got %s", tt.expected.Path, result.Path)
			}
			if result.Name != tt.expected.Name {
				t.Errorf("Expected name %s, got %s", tt.expected.Name, result.Name)
			}
			if result.DevelopmentGroup != tt.expected.DevelopmentGroup {
				t.Errorf("Expected development_group %s, got %s", tt.expected.DevelopmentGroup, result.DevelopmentGroup)
			}
			if result.Glob != tt.expected.Glob {
				t.Errorf("Expected glob %s, got %s", tt.expected.Glob, result.Glob)
			}
		})
	}
}

func TestFindGemspecs(t *testing.T) {
	testDataPath := filepath.Join("..", "testdata")

	tests := []struct {
		name          string
		basePath      string
		glob          string
		nameFilter    string
		expectedCount int
		shouldError   bool
	}{
		{
			name:          "find all gemspecs with default glob",
			basePath:      testDataPath,
			glob:          "",
			nameFilter:    "",
			expectedCount: 3, // test_gem.gemspec, another_gem.gemspec, exotic.gemspec
			shouldError:   false,
		},
		{
			name:          "find specific gemspec by name",
			basePath:      testDataPath,
			glob:          "",
			nameFilter:    "test_gem",
			expectedCount: 1,
			shouldError:   false,
		},
		{
			name:          "find with custom glob",
			basePath:      testDataPath,
			glob:          "test*.gemspec",
			nameFilter:    "",
			expectedCount: 1,
			shouldError:   false,
		},
		{
			name:          "no gemspecs found",
			basePath:      testDataPath,
			glob:          "nonexistent*.gemspec",
			nameFilter:    "",
			expectedCount: 0,
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gemspecs, err := FindGemspecs(tt.basePath, tt.glob, tt.nameFilter)

			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(gemspecs) != tt.expectedCount {
				t.Errorf("Expected %d gemspecs, got %d: %v", tt.expectedCount, len(gemspecs), gemspecs)
			}
		})
	}
}

func TestExpandGlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		glob     string
		expected []string
	}{
		{
			name:     "bundler default pattern",
			basePath: "/test",
			glob:     "{,*,*/*}.gemspec",
			expected: []string{
				"/test/.gemspec",
				"/test/*.gemspec",
				"/test/*/*.gemspec",
			},
		},
		{
			name:     "simple pattern",
			basePath: "/test",
			glob:     "*.gemspec",
			expected: []string{
				"/test/*.gemspec",
			},
		},
		{
			name:     "custom brace expansion",
			basePath: "/gems",
			glob:     "{foo,bar}.gemspec",
			expected: []string{
				"/gems/foo.gemspec",
				"/gems/bar.gemspec",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandGlobPattern(tt.basePath, tt.glob)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLoadGemspecDependencies(t *testing.T) {
	testDataPath := filepath.Join("..", "testdata")

	gemspecRef := GemspecReference{
		Path:             testDataPath,
		Name:             "test_gem",
		DevelopmentGroup: "development",
		Glob:             "",
	}

	deps, err := LoadGemspecDependencies(gemspecRef, ".")
	if err != nil {
		t.Fatalf("Failed to load gemspec dependencies: %v", err)
	}

	// Should include the gem itself as a path dependency
	if len(deps) == 0 {
		t.Error("Expected at least one dependency (the gem itself)")
	}

	// Check that the first dependency is the gem itself
	if deps[0].Name != "test_gem" {
		t.Errorf("Expected first dependency to be 'test_gem', got %s", deps[0].Name)
	}
	if deps[0].Source == nil || deps[0].Source.Type != "path" {
		t.Error("Expected first dependency to have a path source")
	}

	// Check that runtime dependencies are included
	var foundRack bool
	for _, dep := range deps {
		if dep.Name == "rack" {
			foundRack = true
			if len(dep.Groups) != 1 || dep.Groups[0] != "default" {
				t.Errorf("Expected rack to be in default group, got %v", dep.Groups)
			}
		}
	}
	if !foundRack {
		t.Error("Expected to find 'rack' in dependencies")
	}

	// Check that development dependencies are included in the right group
	var foundRspec bool
	for _, dep := range deps {
		if dep.Name == "rspec" {
			foundRspec = true
			if len(dep.Groups) != 1 || dep.Groups[0] != testDevelopment {
				t.Errorf("Expected rspec to be in development group, got %v", dep.Groups)
			}
		}
	}
	if !foundRspec {
		t.Error("Expected to find 'rspec' in dependencies")
	}
}

func TestGemfileWithGemspecDirective(t *testing.T) {
	// Test parsing a Gemfile that contains a gemspec directive
	gemfilePath := filepath.Join("..", "testdata", "gemspec_test_gemfile")
	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile with gemspec: %v", err)
	}

	// Check that gemspec was parsed
	if len(parsed.Gemspecs) != 1 {
		t.Errorf("Expected 1 gemspec reference, got %d", len(parsed.Gemspecs))
	}

	// The gemspec directive should have default values
	if parsed.Gemspecs[0].Path != "." {
		t.Errorf("Expected default path '.', got %s", parsed.Gemspecs[0].Path)
	}
	if parsed.Gemspecs[0].DevelopmentGroup != "development" {
		t.Errorf("Expected default development group 'development', got %s", parsed.Gemspecs[0].DevelopmentGroup)
	}

	// Check that other gems are also parsed
	var foundPuma bool
	for _, dep := range parsed.Dependencies {
		if dep.Name == "puma" {
			foundPuma = true
			if len(dep.Constraints) != 1 || dep.Constraints[0] != "~> 5.6" {
				t.Errorf("Expected puma version '~> 5.6', got %v", dep.Constraints)
			}
		}
	}
	if !foundPuma {
		t.Error("Expected to find 'puma' in dependencies")
	}

	// Check ruby version
	if parsed.RubyVersion != "3.0.0" {
		t.Errorf("Expected ruby version '3.0.0', got %s", parsed.RubyVersion)
	}
}
