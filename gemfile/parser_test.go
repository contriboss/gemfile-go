package gemfile

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGemfileParserSources tests source parsing
func TestGemfileParserSources(t *testing.T) {
	parsed := parseTestGemfile(t)
	testSourceParsing(t, parsed)
}

// TestGemfileParserRuby tests Ruby version parsing
func TestGemfileParserRuby(t *testing.T) {
	parsed := parseTestGemfile(t)
	testRubyVersionParsing(t, parsed)
}

// TestGemfileParserGems tests gem parsing
func TestGemfileParserGems(t *testing.T) {
	parsed := parseTestGemfile(t)
	testGemParsing(t, parsed)
}

// TestGemfileParserGroups tests group parsing
func TestGemfileParserGroups(t *testing.T) {
	parsed := parseTestGemfile(t)
	testGroupParsing(t, parsed)
}

// TestGemfileParserGitGems tests Git gem parsing
func TestGemfileParserGitGems(t *testing.T) {
	parsed := parseTestGemfile(t)
	testGitGemParsing(t, parsed)
}

// TestGemfileParserPathGems tests Path gem parsing
func TestGemfileParserPathGems(t *testing.T) {
	parsed := parseTestGemfile(t)
	testPathGemParsing(t, parsed)
}

// parseTestGemfile creates and parses a test Gemfile
func parseTestGemfile(t *testing.T) *ParsedGemfile {
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

	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err := os.WriteFile(gemfilePath, []byte(testGemfile), 0600)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}
	return parsed
}

// testSourceParsing tests source parsing
func testSourceParsing(t *testing.T, parsed *ParsedGemfile) {
	if len(parsed.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(parsed.Sources))
		return
	}

	source := parsed.Sources[0]
	if source.Type != "rubygems" {
		t.Errorf("Expected source type 'rubygems', got %s", source.Type)
	}
	if source.URL != RubygemsURL {
		t.Errorf("Expected source URL 'https://rubygems.org', got %s", source.URL)
	}
}

// testRubyVersionParsing tests Ruby version parsing
func testRubyVersionParsing(t *testing.T, parsed *ParsedGemfile) {
	if parsed.RubyVersion != "3.2.0" {
		t.Errorf("Expected ruby version '3.2.0', got %s", parsed.RubyVersion)
	}
}

// testGemParsing tests basic gem parsing
func testGemParsing(t *testing.T, parsed *ParsedGemfile) {
	if len(parsed.Dependencies) < 4 {
		t.Errorf("Expected at least 4 dependencies, got %d", len(parsed.Dependencies))
	}
}

// testGroupParsing tests group parsing
func testGroupParsing(t *testing.T, parsed *ParsedGemfile) {
	// Simple test - just check we have some gems with groups
	hasGroupedGems := false
	for _, dep := range parsed.Dependencies {
		if len(dep.Groups) > 0 && dep.Groups[0] != DefaultGroup {
			hasGroupedGems = true
			break
		}
	}
	if !hasGroupedGems {
		t.Error("Expected to find gems with non-default groups")
	}
}

// testGitGemParsing tests Git gem parsing
func testGitGemParsing(t *testing.T, parsed *ParsedGemfile) {
	hasGitGem := false
	for _, dep := range parsed.Dependencies {
		if dep.Source != nil && dep.Source.Type == GitSource {
			hasGitGem = true
			break
		}
	}
	if !hasGitGem {
		t.Error("Expected to find at least one Git gem")
	}
}

// testPathGemParsing tests Path gem parsing
func testPathGemParsing(t *testing.T, parsed *ParsedGemfile) {
	hasPathGem := false
	for _, dep := range parsed.Dependencies {
		if dep.Source != nil && dep.Source.Type == PathStr {
			hasPathGem = true
			break
		}
	}
	if !hasPathGem {
		t.Error("Expected to find at least one Path gem")
	}
}

func TestGemfileParserSimple(t *testing.T) {
	simpleGemfile := `gem 'rails'
gem 'puma', '~> 5.0'`

	// Write to temp file
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err := os.WriteFile(gemfilePath, []byte(simpleGemfile), 0600)
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

func findGem(deps []GemDependency, name string) *GemDependency {
	for _, dep := range deps {
		if dep.Name == name {
			return &dep
		}
	}
	return nil
}
