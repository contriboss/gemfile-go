package gemfile

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
		parser := NewTreeSitterGemfileParser([]byte(gemfileContent), "")
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
		parser := NewTreeSitterGemfileParser([]byte(gemfileContent), "")
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}
		assertSources(t, parsed)
	})
}

// Helper functions
func TestHashRocketSyntax(t *testing.T) {
	testGemfile := `# Gemfile with hash rocket syntax
source 'https://gem.coop'

gemspec :path => './', :name => 'my-gem', :development_group => :dev, :glob => '*.gemspec'

gem 'rails', :require => 'rails/all'
gem 'rspec', :groups => [:development, :test]
gem 'nokogiri', :platforms => [:mri, :jruby]
gem 'pg', :git => 'https://github.com/postgres/postgres.git', :branch => 'master'
gem 'mysql2', :github => 'brianmario/mysql2', :branch => 'master'
gem 'sqlite3', :path => 'vendor/sqlite3'
gem 'redis', :source => 'https://gems.example.com'
`
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile_hashrocket")
	err := os.WriteFile(gemfilePath, []byte(testGemfile), 0600)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Test gemspec with hash rocket
	if len(parsed.Gemspecs) == 0 {
		t.Fatal("Expected gemspec to be parsed")
	}
	gs := parsed.Gemspecs[0]
	// Path should be the raw path from the Gemfile, not resolved
	if gs.Path != "./" {
		t.Errorf("Expected gemspec path './', got %s", gs.Path)
	}
	if gs.Name != "my-gem" {
		t.Errorf("Expected gemspec name 'my-gem', got %s", gs.Name)
	}
	if gs.DevelopmentGroup != "dev" {
		t.Errorf("Expected gemspec development_group 'dev', got %s", gs.DevelopmentGroup)
	}
	if gs.Glob != "*.gemspec" {
		t.Errorf("Expected gemspec glob '*.gemspec', got %s", gs.Glob)
	}

	// Test gems with hash rocket
	expectedGems := map[string]struct {
		require    string
		groups     []string
		platforms  []string
		sourceType string
		sourceURL  string
		branch     string
	}{
		"rails": {
			require: "rails/all",
		},
		"rspec": {
			groups: []string{"development", "test"},
		},
		"nokogiri": {
			platforms: []string{"mri", "jruby"},
		},
		"pg": {
			sourceType: "git",
			sourceURL:  "https://github.com/postgres/postgres.git",
			branch:     "master",
		},
		"mysql2": {
			sourceType: "git",
			sourceURL:  "https://github.com/brianmario/mysql2.git",
			branch:     "master",
		},
		"sqlite3": {
			sourceType: pathSource,
			sourceURL:  filepath.Clean(filepath.Join(tmpDir, "vendor", "sqlite3")),
		},
		"redis": {
			sourceType: "rubygems",
			sourceURL:  "https://gems.example.com",
		},
	}

	for name, expected := range expectedGems {
		var found *GemDependency
		for _, g := range parsed.Dependencies {
			if g.Name == name {
				found = &g
				break
			}
		}

		if found == nil {
			t.Errorf("Gem %s not found", name)
			continue
		}

		// Verify constraints are empty for these gems (as they are options)
		if len(found.Constraints) > 0 {
			t.Errorf("Gem %s: expected 0 constraints, got %v", name, found.Constraints)
		}

		if expected.require != "" {
			if found.Require == nil || *found.Require != expected.require {
				val := "nil"
				if found.Require != nil {
					val = *found.Require
				}
				t.Errorf("Gem %s: expected require %s, got %s", name, expected.require, val)
			}
		}

		if len(expected.groups) > 0 {
			if !reflect.DeepEqual(found.Groups, expected.groups) {
				t.Errorf("Gem %s: expected groups %v, got %v", name, expected.groups, found.Groups)
			}
		}

		if len(expected.platforms) > 0 {
			if !reflect.DeepEqual(found.Platforms, expected.platforms) {
				t.Errorf("Gem %s: expected platforms %v, got %v", name, expected.platforms, found.Platforms)
			}
		}

		if expected.sourceType != "" {
			if found.Source == nil {
				t.Errorf("Gem %s: expected source, got nil", name)
			} else {
				if found.Source.Type != expected.sourceType {
					t.Errorf("Gem %s: expected source type %s, got %s", name, expected.sourceType, found.Source.Type)
				}
				if found.Source.URL != expected.sourceURL {
					t.Errorf("Gem %s: expected source URL %s, got %s", name, expected.sourceURL, found.Source.URL)
				}
				if expected.branch != "" && found.Source.Branch != expected.branch {
					t.Errorf("Gem %s: expected branch %s, got %s", name, expected.branch, found.Source.Branch)
				}
			}
		}
	}
}

func TestSingleGroupAndPlatform(t *testing.T) {
	testGemfile := `# Gemfile with single group and platform values
source 'https://rubygems.org'

gem 'rack', :group => :test
gem 'thor', groups: :development
gem 'json', :platform => :mri
gem 'rake', platforms: 'ruby'
`
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile_single_values")
	err := os.WriteFile(gemfilePath, []byte(testGemfile), 0600)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	expectedGems := map[string]struct {
		groups    []string
		platforms []string
	}{
		"rack": {
			groups: []string{"test"},
		},
		"thor": {
			groups: []string{"development"},
		},
		"json": {
			platforms: []string{"mri"},
		},
		"rake": {
			platforms: []string{"ruby"},
		},
	}

	for name, expected := range expectedGems {
		found := findGem(parsed.Dependencies, name)
		if found == nil {
			t.Errorf("Gem %s not found", name)
			continue
		}

		if len(expected.groups) > 0 {
			if !reflect.DeepEqual(found.Groups, expected.groups) {
				t.Errorf("Gem %s: expected groups %v, got %v", name, expected.groups, found.Groups)
			}
		}

		if len(expected.platforms) > 0 {
			if !reflect.DeepEqual(found.Platforms, expected.platforms) {
				t.Errorf("Gem %s: expected platforms %v, got %v", name, expected.platforms, found.Platforms)
			}
		}
	}
}

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

func TestParseGroupsImproved(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "standard group",
			line:     "group :development, :test do",
			expected: []string{"development", "test"},
		},
		{
			name:     "group with comment",
			line:     "group :development do # comment",
			expected: []string{"development"},
		},
		{
			name:     "group with trailing spaces",
			line:     "group :development do  ",
			expected: []string{"development"},
		},
		{
			name:     "group with no do",
			line:     "group :development, :test",
			expected: []string{"development", "test"},
		},
		{
			name:     "group with string names",
			line:     "group 'development', \"test\" do",
			expected: []string{"development", "test"},
		},
		{
			name:     "complex line with comment",
			line:     "group :development, :test do # some note",
			expected: []string{"development", "test"},
		},
	}

	parser := &GemfileParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.parseGroups(tt.line)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseGroups(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
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

func TestEnvFetchSupport(t *testing.T) {
	// Test ENV.fetch with default value
	testGemfile := `source 'https://rubygems.org'

gem 'rails'
gem 'activesupport', ENV.fetch("RAILS_VERSION", "~> 8.1.0")
gem 'railties', ENV.fetch("RAILS_VERSION", "~> 8.1.0")
`

	t.Run("ENV.fetch with default (env not set)", func(t *testing.T) {
		// Make sure env var is not set
		os.Unsetenv("RAILS_VERSION")

		parser := NewTreeSitterGemfileParser([]byte(testGemfile), "")
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}

		// Find activesupport gem
		as := findGem(parsed.Dependencies, "activesupport")
		if as == nil {
			t.Fatal("expected activesupport gem to be parsed")
		}

		// Should use default value "~> 8.1.0"
		if len(as.Constraints) != 1 || as.Constraints[0] != "~> 8.1.0" {
			t.Errorf("expected constraint '~> 8.1.0', got %v", as.Constraints)
		}
	})

	t.Run("ENV.fetch with env var set", func(t *testing.T) {
		os.Setenv("RAILS_VERSION", "~> 7.1.0")
		defer os.Unsetenv("RAILS_VERSION")

		parser := NewTreeSitterGemfileParser([]byte(testGemfile), "")
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}

		// Find activesupport gem
		as := findGem(parsed.Dependencies, "activesupport")
		if as == nil {
			t.Fatal("expected activesupport gem to be parsed")
		}

		// Should use env var value "~> 7.1.0"
		if len(as.Constraints) != 1 || as.Constraints[0] != "~> 7.1.0" {
			t.Errorf("expected constraint '~> 7.1.0', got %v", as.Constraints)
		}
	})

	t.Run("ENV.fetch with empty string env var", func(t *testing.T) {
		// Set env var to empty string
		os.Setenv("RAILS_VERSION", "")
		defer os.Unsetenv("RAILS_VERSION")

		parser := NewTreeSitterGemfileParser([]byte(testGemfile))
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}

		// Find activesupport gem
		as := findGem(parsed.Dependencies, "activesupport")
		if as == nil {
			t.Fatal("expected activesupport gem to be parsed")
		}

		// Should use empty string from env var, NOT the default value
		// Ruby: ENV.fetch("X", "default") returns "" when ENV["X"] == ""
		if len(as.Constraints) != 0 {
			t.Errorf("expected no constraints (empty string), got %v", as.Constraints)
		}
	})
}

func TestEnvElementReferenceSupport(t *testing.T) {
	// Test ENV["VAR"] syntax
	testGemfile := `source 'https://rubygems.org'

gem 'my_gem', ENV["MY_GEM_VERSION"]
`

	t.Run("ENV[] with env var set", func(t *testing.T) {
		os.Setenv("MY_GEM_VERSION", "~> 2.0")
		defer os.Unsetenv("MY_GEM_VERSION")

		parser := NewTreeSitterGemfileParser([]byte(testGemfile), "")
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}

		gem := findGem(parsed.Dependencies, "my_gem")
		if gem == nil {
			t.Fatal("expected my_gem to be parsed")
		}

		if len(gem.Constraints) != 1 || gem.Constraints[0] != "~> 2.0" {
			t.Errorf("expected constraint '~> 2.0', got %v", gem.Constraints)
		}
	})

	t.Run("ENV[] with env var not set", func(t *testing.T) {
		os.Unsetenv("MY_GEM_VERSION")

		parser := NewTreeSitterGemfileParser([]byte(testGemfile), "")
		parsed, err := parser.ParseWithTreeSitter()
		if err != nil {
			t.Fatalf("ParseWithTreeSitter failed: %v", err)
		}

		gem := findGem(parsed.Dependencies, "my_gem")
		if gem == nil {
			t.Fatal("expected my_gem to be parsed")
		}

		// Should have no constraints when env var is not set
		if len(gem.Constraints) != 0 {
			t.Errorf("expected no constraints when env not set, got %v", gem.Constraints)
		}
	})
}

// TestPathNormalizationConsistency verifies that both tree-sitter and regex parsers
// normalize relative path sources consistently
func TestPathNormalizationConsistency(t *testing.T) {
	gemfileContent := `source 'https://rubygems.org'

gem 'local_gem', path: '../vendor/local_gem'
gem 'another_gem', path: './relative/path'
`

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	// Parse with the main parser (will use tree-sitter if conditions are met)
	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Also parse with tree-sitter explicitly
	tsParser := NewTreeSitterGemfileParser([]byte(gemfileContent), gemfilePath)
	tsParsed, err := tsParser.ParseWithTreeSitter()
	if err != nil {
		t.Fatalf("Tree-sitter parse failed: %v", err)
	}

	// Verify we have the expected gems
	if len(parsed.Dependencies) != 2 {
		t.Fatalf("Expected 2 dependencies, got %d", len(parsed.Dependencies))
	}

	if len(tsParsed.Dependencies) != 2 {
		t.Fatalf("Tree-sitter: Expected 2 dependencies, got %d", len(tsParsed.Dependencies))
	}

	// Check that paths are normalized (absolute)
	for _, dep := range parsed.Dependencies {
		if dep.Source == nil {
			t.Errorf("Expected source for gem %s", dep.Name)
			continue
		}
		if dep.Source.Type != "path" {
			t.Errorf("Expected path source for gem %s, got %s", dep.Name, dep.Source.Type)
			continue
		}
		if !filepath.IsAbs(dep.Source.URL) {
			t.Errorf("Expected absolute path for gem %s, got %s", dep.Name, dep.Source.URL)
		}
	}

	// Check tree-sitter results
	for _, dep := range tsParsed.Dependencies {
		if dep.Source == nil {
			t.Errorf("Tree-sitter: Expected source for gem %s", dep.Name)
			continue
		}
		if dep.Source.Type != "path" {
			t.Errorf("Tree-sitter: Expected path source for gem %s, got %s", dep.Name, dep.Source.Type)
			continue
		}
		if !filepath.IsAbs(dep.Source.URL) {
			t.Errorf("Tree-sitter: Expected absolute path for gem %s, got %s", dep.Name, dep.Source.URL)
		}
	}

	// Verify both parsers produce the same paths (order-independent)
	regexDepsByName := make(map[string]string, len(parsed.Dependencies))
	for _, dep := range parsed.Dependencies {
		if dep.Source == nil {
			t.Errorf("Missing source for gem %s in regex parser results", dep.Name)
			continue
		}
		regexDepsByName[dep.Name] = dep.Source.URL
	}

	tsDepsByName := make(map[string]string, len(tsParsed.Dependencies))
	for _, dep := range tsParsed.Dependencies {
		if dep.Source == nil {
			t.Errorf("Missing source for gem %s in tree-sitter results", dep.Name)
			continue
		}
		tsDepsByName[dep.Name] = dep.Source.URL
	}

	for name, regexURL := range regexDepsByName {
		tsURL, ok := tsDepsByName[name]
		if !ok {
			t.Errorf("Tree-sitter: missing gem %s present in regex parser results", name)
			continue
		}
		if regexURL != tsURL {
			t.Errorf("Path mismatch for gem %s: regex=%s, tree-sitter=%s",
				name, regexURL, tsURL)
		}
	}

	for name := range tsDepsByName {
		if _, ok := regexDepsByName[name]; !ok {
			t.Errorf("Regex parser: missing gem %s present in tree-sitter results", name)
		}
	}
}
