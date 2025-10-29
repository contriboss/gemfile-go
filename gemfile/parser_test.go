package gemfile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const rubyChinaURL = "https://gems.ruby-china.com"

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
	err := os.WriteFile(gemfilePath, []byte(testGemfile), 0600)
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
		if source.Type != rubygemsSource {
			t.Errorf("Expected source type 'rubygems', got %s", source.Type)
		}
		if source.URL != rubygemsURL {
			t.Errorf("Expected source URL '%s', got %s", rubygemsURL, source.URL)
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
		platforms   []string
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
		checkGemDependency(t, &dep, expectedGems)
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

func TestInlineSourceOption(t *testing.T) {
	gemfileContent := fmt.Sprintf("gem 'webmock', '~> 3.19', source: '%s'", rubyChinaURL)

	check := func(t *testing.T, parsed *ParsedGemfile) {
		t.Helper()
		if len(parsed.Dependencies) != 1 {
			t.Fatalf("expected 1 dependency, got %d", len(parsed.Dependencies))
		}

		dep := parsed.Dependencies[0]
		if len(dep.Constraints) != 1 || dep.Constraints[0] != "~> 3.19" {
			t.Errorf("expected constraint '~> 3.19', got %v", dep.Constraints)
		}
		if dep.Source == nil {
			t.Fatalf("expected inline source to set source, got nil")
		}
		if dep.Source.Type != rubygemsSource {
			t.Errorf("expected source type 'rubygems', got %s", dep.Source.Type)
		}
		if dep.Source.URL != rubyChinaURL {
			t.Errorf("expected source URL %q, got %s", rubyChinaURL, dep.Source.URL)
		}
	}

	t.Run("regex parser", func(t *testing.T) {
		parser := &GemfileParser{content: gemfileContent}
		parsed, err := parser.parseContent()
		if err != nil {
			t.Fatalf("parseContent failed: %v", err)
		}
		check(t, parsed)
	})

	t.Run("tree-sitter parser", func(t *testing.T) {
		parser := NewTreeSitterGemfileParser([]byte(gemfileContent))
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}
		check(t, parsed)
	})
}

func TestInlineSourceOverridesBlock(t *testing.T) {
	gemfileContent := fmt.Sprintf(`source 'https://gem.coop' do
  gem 'inside_block'
  gem 'inline_override', source: '%s'
end

gem 'outside_block'
`, rubyChinaURL)

	assertSources := func(t *testing.T, parsed *ParsedGemfile) {
		t.Helper()

		inside := findGem(parsed.Dependencies, "inside_block")
		if inside == nil || inside.Source == nil {
			t.Fatalf("expected inside_block to inherit block source")
		}
		if inside.Source.URL != "https://gem.coop" {
			t.Errorf("inside_block expected source https://gem.coop, got %s", inside.Source.URL)
		}

		override := findGem(parsed.Dependencies, "inline_override")
		if override == nil || override.Source == nil {
			t.Fatalf("expected inline_override to have inline source")
		}
		if override.Source.Type != rubygemsSource {
			t.Errorf("inline_override expected source type rubygems, got %s", override.Source.Type)
		}
		if override.Source.URL != rubyChinaURL {
			t.Errorf("inline_override expected source %s, got %s", rubyChinaURL, override.Source.URL)
		}

		outside := findGem(parsed.Dependencies, "outside_block")
		if outside == nil {
			t.Fatalf("expected outside_block gem to be parsed")
		}
		if outside.Source != nil {
			t.Errorf("outside_block expected no source, got %+v", outside.Source)
		}
	}

	t.Run("regex parser", func(t *testing.T) {
		parser := &GemfileParser{content: gemfileContent}
		parsed, err := parser.parseContent()
		if err != nil {
			t.Fatalf("parseContent failed: %v", err)
		}
		assertSources(t, parsed)
	})

	t.Run("tree-sitter parser", func(t *testing.T) {
		parser := NewTreeSitterGemfileParser([]byte(gemfileContent))
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}
		assertSources(t, parsed)
	})
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

func checkGemDependency(t *testing.T, dep *GemDependency, expectedGems map[string]struct {
	constraints []string
	groups      []string
	sourceType  string
	requireVal  *string
	platforms   []string
}) {
	expected, exists := expectedGems[dep.Name]
	if !exists {
		t.Errorf("Unexpected gem: %s", dep.Name)
		return
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

	// Check platforms
	if len(expected.platforms) > 0 {
		if len(dep.Platforms) != len(expected.platforms) {
			t.Errorf("Gem %s: expected %d platforms, got %d",
				dep.Name, len(expected.platforms), len(dep.Platforms))
		} else {
			for i, platform := range expected.platforms {
				if dep.Platforms[i] != platform {
					t.Errorf("Gem %s: expected platform %s, got %s",
						dep.Name, platform, dep.Platforms[i])
				}
			}
		}
	}
}

func TestSourceBlocks(t *testing.T) {
	// Create a test Gemfile with source blocks
	testGemfile := fmt.Sprintf(`# Test Gemfile with source blocks
source 'https://rubygems.org'

ruby '3.2.0'

gem 'rake'
gem 'rails', '~> 7.0'

source 'https://gem.coop' do
  gem 'minitest'
  gem 'rspec', '~> 3.0'
end

gem 'rack'
gem 'puma', '>= 5.0'

source '%s' do
  gem 'private_gem'
  gem 'another_private', require: false
end

group :development do
  gem 'rubocop'
end

# Gem with explicit git source inside a source block should use git source
source 'https://gem.coop' do
  gem 'custom_gem'
  gem 'git_gem', github: 'user/repo'
end
`, rubyChinaURL)

	// Write to temp file
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err := os.WriteFile(gemfilePath, []byte(testGemfile), 0600)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	// Parse the Gemfile
	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Test source parsing - should have 4 sources (rubygems.org + 2x gem.coop + gems.ruby-china.com)
	expectedSourceCount := 4
	if len(parsed.Sources) != expectedSourceCount {
		t.Errorf("Expected %d sources, got %d", expectedSourceCount, len(parsed.Sources))
	}

	// Define expected gem sources
	expectedGemSources := map[string]struct {
		hasSource  bool
		sourceURL  string
		sourceType string
	}{
		"rake":            {hasSource: false}, // No source block, should be nil
		"rails":           {hasSource: false}, // No source block, should be nil
		"minitest":        {hasSource: true, sourceURL: "https://gem.coop", sourceType: rubygemsSource},
		"rspec":           {hasSource: true, sourceURL: "https://gem.coop", sourceType: rubygemsSource},
		"rack":            {hasSource: false}, // Outside source block, should be nil
		"puma":            {hasSource: false}, // Outside source block, should be nil
		"private_gem":     {hasSource: true, sourceURL: rubyChinaURL, sourceType: rubygemsSource},
		"another_private": {hasSource: true, sourceURL: rubyChinaURL, sourceType: rubygemsSource},
		"rubocop":         {hasSource: false}, // In group block, not source block
		"custom_gem":      {hasSource: true, sourceURL: "https://gem.coop", sourceType: rubygemsSource},
		"git_gem":         {hasSource: true, sourceURL: "https://github.com/user/repo.git", sourceType: "git"}, // Explicit git source overrides
	}

	// Check each gem's source
	for _, dep := range parsed.Dependencies {
		expected, exists := expectedGemSources[dep.Name]
		if !exists {
			t.Errorf("Unexpected gem found: %s", dep.Name)
			continue
		}

		if expected.hasSource {
			if dep.Source == nil {
				t.Errorf("Gem %s: expected source but got nil", dep.Name)
			} else {
				if dep.Source.URL != expected.sourceURL {
					t.Errorf("Gem %s: expected source URL %s, got %s",
						dep.Name, expected.sourceURL, dep.Source.URL)
				}
				if dep.Source.Type != expected.sourceType {
					t.Errorf("Gem %s: expected source type %s, got %s",
						dep.Name, expected.sourceType, dep.Source.Type)
				}
			}
		} else {
			if dep.Source != nil {
				t.Errorf("Gem %s: expected no source but got %s (%s)",
					dep.Name, dep.Source.URL, dep.Source.Type)
			}
		}
	}

	// Verify all expected gems were found
	if len(parsed.Dependencies) != len(expectedGemSources) {
		t.Errorf("Expected %d gems, got %d", len(expectedGemSources), len(parsed.Dependencies))
	}
}

func TestGemfileParserPlatforms(t *testing.T) {
	// Create a test Gemfile with platform restrictions
	testGemfile := `source 'https://rubygems.org'

# Single platform
gem "weakling", platforms: :jruby
gem "ruby-debug", platforms: :mri_31

# Multiple platforms
gem "nokogiri", platforms: [:windows_31, :jruby]
gem "thin", "~> 1.7", platforms: [:ruby, :mswin]

# Platform with version constraints and require
gem "sqlite3", "~> 1.4", require: false, platforms: :ruby

# Platform with groups
group :development do
  gem "pry-byebug", platforms: :mri
end
`

	// Write to temp file
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err := os.WriteFile(gemfilePath, []byte(testGemfile), 0600)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	// Parse the Gemfile
	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Test platform parsing
	expectedGems := map[string]struct {
		constraints []string
		groups      []string
		sourceType  string
		requireVal  *string
		platforms   []string
	}{
		"weakling": {
			constraints: []string{},
			groups:      []string{"default"},
			platforms:   []string{"jruby"},
		},
		"ruby-debug": {
			constraints: []string{},
			groups:      []string{"default"},
			platforms:   []string{"mri_31"},
		},
		"nokogiri": {
			constraints: []string{},
			groups:      []string{"default"},
			platforms:   []string{"windows_31", "jruby"},
		},
		"thin": {
			constraints: []string{"~> 1.7"},
			groups:      []string{"default"},
			platforms:   []string{"ruby", "mswin"},
		},
		"sqlite3": {
			constraints: []string{"~> 1.4"},
			groups:      []string{"default"},
			requireVal:  stringPtr(""),
			platforms:   []string{"ruby"},
		},
		"pry-byebug": {
			constraints: []string{},
			groups:      []string{"development"},
			platforms:   []string{"mri"},
		},
	}

	if len(parsed.Dependencies) != len(expectedGems) {
		t.Errorf("Expected %d gems, got %d", len(expectedGems), len(parsed.Dependencies))
	}

	for _, dep := range parsed.Dependencies {
		checkGemDependency(t, &dep, expectedGems)
	}
}
