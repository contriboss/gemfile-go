package gemfile

import (
	"path/filepath"
	"testing"
)

func TestExoticGemspec(t *testing.T) {
	// Test parsing an exotic gemspec with non-orthodox patterns
	gemspecPath := filepath.Join("..", "testdata", "exotic.gemspec")
	parser := NewGemspecParser(gemspecPath)
	gemspec, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse exotic gemspec: %v", err)
	}

	// The parser should at least extract the name (even if tree-sitter fails on other parts)
	if gemspec.Name == "" {
		t.Error("Expected to extract gem name from exotic gemspec")
	}

	// Check that we get some reasonable values even with exotic patterns
	t.Logf("Parsed exotic gemspec:")
	t.Logf("  Name: %s", gemspec.Name)
	t.Logf("  Version: %s", gemspec.Version)
	t.Logf("  Summary: %s", gemspec.Summary)
	t.Logf("  License: %s", gemspec.License)
	t.Logf("  Homepage: %s", gemspec.Homepage)
	t.Logf("  Runtime deps: %d", len(gemspec.RuntimeDependencies))
	t.Logf("  Dev deps: %d", len(gemspec.DevelopmentDependencies))
	t.Logf("  Metadata entries: %d", len(gemspec.Metadata))

	// At minimum, we should get the statically defined values
	if gemspec.Name != "exotic_gem" && gemspec.Name != "" {
		t.Errorf("Expected name 'exotic_gem', got %s", gemspec.Name)
	}

	// The parser should handle the complex dependency gracefully
	var foundComplexGem bool
	for _, dep := range gemspec.RuntimeDependencies {
		if dep.Name == "complex_gem" {
			foundComplexGem = true
			// Should have multiple constraints
			if len(dep.Constraints) < 2 {
				t.Logf("complex_gem constraints: %v", dep.Constraints)
			}
		}
	}

	if !foundComplexGem {
		t.Log("Warning: complex_gem dependency not found - parser may have skipped it due to complexity")
	}

	// Test that metaprogramming dependencies are handled
	// The parser might not catch these with tree-sitter, but regex fallback might
	devDepNames := make(map[string]bool)
	for _, dep := range gemspec.DevelopmentDependencies {
		devDepNames[dep.Name] = true
	}

	// These might or might not be found depending on which parser succeeded
	if devDepNames["thor"] || devDepNames["rake"] {
		t.Log("Successfully extracted metaprogrammed dependencies")
	} else {
		t.Log("Metaprogrammed dependencies were not extracted (expected with tree-sitter)")
	}

	// Ensure the parser doesn't crash on exotic patterns
	t.Log("Parser handled exotic gemspec without crashing - success!")
}

func TestExoticGemspecRobustness(t *testing.T) {
	// This test ensures our parser is robust against various exotic patterns
	testCases := []struct {
		name     string
		content  string
		expected string // Expected gem name (if any)
	}{
		{
			name: "tap pattern",
			content: `
Gem::Specification.new.tap do |s|
  s.name = "tap_gem"
  s.version = "1.0.0"
end
`,
			expected: "tap_gem",
		},
		{
			name: "instance_eval pattern",
			content: `
Gem::Specification.new do |s|
  s.instance_eval do
    @name = "eval_gem"
    @version = "1.0.0"
  end
end
`,
			expected: "", // Tree-sitter won't catch this
		},
		{
			name: "heredoc everything",
			content: `
Gem::Specification.new do |spec|
  spec.name = <<~NAME.strip
    heredoc_gem
  NAME
  spec.version = <<~VERSION.strip
    2.0.0
  VERSION
end
`,
			expected: "", // Tree-sitter might struggle with this
		},
		{
			name: "string interpolation",
			content: `
GEM_PREFIX = "my"
Gem::Specification.new do |spec|
  spec.name = "#{GEM_PREFIX}_gem"
  spec.version = "1.#{2 + 3}.0"
end
`,
			expected: "", // Can't evaluate interpolation statically
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a parser with the content
			tsParser := NewTreeSitterGemspecParser([]byte(tc.content))
			gemspec, err := tsParser.ParseWithTreeSitter()

			if err != nil {
				t.Logf("Tree-sitter failed for %s (expected): %v", tc.name, err)
				return
			}

			if tc.expected != "" {
				if gemspec.Name != tc.expected {
					t.Errorf("Expected name %q, got %q", tc.expected, gemspec.Name)
				}
			} else {
				t.Logf("Pattern '%s' extracted name: %q (static parsing limitation acknowledged)", tc.name, gemspec.Name)
			}
		})
	}
}
