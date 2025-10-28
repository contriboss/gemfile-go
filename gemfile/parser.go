// Package gemfile provides a parser for Ruby's Gemfile format.
// It parses the Bundler DSL without evaluating Ruby code.
//
// Ruby equivalent: Bundler::Definition
package gemfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GemfileParser parses Gemfile syntax into structured data.
// Ruby equivalent: Bundler::Dsl
type GemfileParser struct {
	filepath string
	content  string
}

// ParsedGemfile represents the parsed Gemfile content.
type ParsedGemfile struct {
	Dependencies []GemDependency    // Declared gems
	Sources      []Source           // Gem sources
	RubyVersion  string             // Ruby version requirement
	GitSources   map[string]string  // Gem name to git URL mapping
	Gemspecs     []GemspecReference // Gemspec references
}

// GemDependency represents a gem dependency.
// Ruby equivalent: gem "name", "version", options
type GemDependency struct {
	Name        string   // Gem name
	Constraints []string // Version constraints (e.g., "~> 2.0" means >= 2.0.0 and < 3.0.0)
	Source      *Source  // Git, path, source block URL, or nil for default source
	Groups      []string // Groups (empty means :default)
	Require     *string  // Require behavior (nil = normal, "false" = no auto-require)
	Platforms   []string // Platform restrictions (e.g., [:jruby, :windows_31])
	Comment     string   // Inline comment if present
}

// Source represents a gem source (RubyGems, Git, Path)
type Source struct {
	Type   string // "rubygems", "git", "path"
	URL    string
	Branch string // for git sources
	Tag    string // for git sources
	Ref    string // for git sources
}

// GemspecReference represents a gemspec directive in the Gemfile.
// Ruby equivalent: gemspec path: "path", name: "name", development_group: :group
type GemspecReference struct {
	Path             string // Path to search for gemspec files (defaults to ".")
	Name             string // Specific gemspec name to load (optional)
	DevelopmentGroup string // Group for development dependencies (defaults to "development")
	Glob             string // Glob pattern for finding gemspec files (defaults to "{,*,*/*}.gemspec")
}

// GemspecFile represents a parsed .gemspec file
type GemspecFile struct {
	Name                    string            // Gem name from spec.name
	Version                 string            // Gem version from spec.version
	Summary                 string            // Gem summary
	Description             string            // Gem description
	Authors                 []string          // Gem authors
	Email                   []string          // Contact emails
	Homepage                string            // Project homepage
	License                 string            // License identifier
	RuntimeDependencies     []GemDependency   // Runtime dependencies from add_runtime_dependency
	DevelopmentDependencies []GemDependency   // Development dependencies from add_development_dependency
	RequiredRubyVersion     string            // Required Ruby version
	Files                   []string          // Files included in the gem
	Metadata                map[string]string // Additional metadata
}

// NewGemfileParser creates a new parser for the given Gemfile path
func NewGemfileParser(filePath string) *GemfileParser {
	return &GemfileParser{filepath: filePath}
}

// Parse parses the Gemfile and returns structured data
// It tries tree-sitter first (most robust), then falls back to regex parsing
func (p *GemfileParser) Parse() (*ParsedGemfile, error) {
	content, err := os.ReadFile(p.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gemfile: %w", err)
	}

	p.content = string(content)

	// Try tree-sitter first (handles complex Ruby constructs like nested blocks)
	// Note: Currently experimental - falls back to regex for edge cases
	tsParser := NewTreeSitterGemfileParser([]byte(p.content))
	gemfile, err := tsParser.ParseWithTreeSitter()

	// Use tree-sitter result if it found content AND no gemspec directives
	// (gemspec integration needs more work)
	useTreeSitter := err == nil &&
		(len(gemfile.Dependencies) > 0 || gemfile.RubyVersion != "") &&
		len(gemfile.Gemspecs) == 0

	if useTreeSitter {
		return gemfile, nil
	}

	// Fall back to regex parsing (more battle-tested for edge cases)
	return p.parseContent()
}

// parseContent parses the Gemfile content using regex patterns
func (p *GemfileParser) parseContent() (*ParsedGemfile, error) {
	result := &ParsedGemfile{
		Dependencies: []GemDependency{},
		Sources:      []Source{},
		GitSources:   make(map[string]string),
	}

	scanner := bufio.NewScanner(strings.NewReader(p.content))
	lineNum := 0
	currentGroups := []string{"default"} // Default group
	variables := make(map[string]string) // Track variables
	var currentSource *Source            // Track current source block
	blockDepth := 0                      // Track nesting depth for source blocks

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse variable assignments first
		if varName, varValue := p.parseVariable(line); varName != "" {
			variables[varName] = varValue
			continue
		}

		// Expand variables in the line
		expandedLine := p.expandVariables(line, variables)

		// Parse different types of lines
		if err := p.parseLine(expandedLine, &currentGroups, &currentSource, &blockDepth, result); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

	return result, nil
}

// parseLine parses a single line of the Gemfile
func (p *GemfileParser) parseLine(
	line string,
	currentGroups *[]string,
	currentSource **Source,
	blockDepth *int,
	result *ParsedGemfile,
) error {
	line = strings.TrimSpace(line)

	// Parse source declarations
	if strings.HasPrefix(line, "source ") {
		source, isBlock, err := p.parseSource(line)
		if err == nil {
			result.Sources = append(result.Sources, source)
			// If this is a source block (has 'do'), set it as current source
			if isBlock {
				*currentSource = &source
				*blockDepth = 1 // Start tracking block depth
			}
		}
		return nil
	}

	// Parse git_source declarations
	if strings.HasPrefix(line, "git_source(") {
		// git_source(:github) { |repo| "https://github.com/#{repo}.git" }
		// Store for later use - simplified parsing for now
		return nil
	}

	// Parse group blocks
	if strings.HasPrefix(line, "group ") {
		*currentGroups = p.parseGroups(line)
		// Increment block depth if this is a group block
		if strings.Contains(line, " do") {
			*blockDepth++
		}
		return nil
	}

	// Parse end statements
	if line == endKeyword {
		*blockDepth--
		// Reset current source when we exit a source block (depth returns to 0)
		if *blockDepth == 0 {
			*currentSource = nil
		}
		// Always reset groups when exiting any block
		*currentGroups = []string{"default"}
		return nil
	}

	// Parse gemspec directive
	if strings.HasPrefix(line, "gemspec") {
		return p.handleGemspecDirective(line, result)
	}

	// Parse gem declarations
	if strings.HasPrefix(line, "gem ") {
		dep, err := p.parseGemLine(line, *currentGroups, *currentSource)
		if err != nil {
			return err
		}
		if dep != nil {
			result.Dependencies = append(result.Dependencies, *dep)
		}
		return nil
	}

	// Parse ruby version
	if strings.HasPrefix(line, "ruby ") {
		result.RubyVersion = p.parseRubyVersion(line)
		return nil
	}

	// Skip other lines (variables, etc.)
	return nil
}

// parseSource parses source declarations
// Examples:
//
//	source 'https://rubygems.org'
//	source 'https://gem.coop' do
//
// Returns the Source, a boolean indicating if it's a block (has 'do'), and an error
func (p *GemfileParser) parseSource(line string) (Source, bool, error) {
	re := regexp.MustCompile(`source\s+['"]([^'"]+)['"]`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return Source{}, false, fmt.Errorf("invalid source line: %s", line)
	}

	source := Source{
		Type: "rubygems",
		URL:  matches[1],
	}

	// Check if this is a source block (has 'do' keyword)
	isBlock := strings.Contains(line, " do")

	return source, isBlock, nil
}

// parseGroups parses group declarations
// Examples: group :development, :test do
func (p *GemfileParser) parseGroups(line string) []string {
	// Extract group names using regex
	re := regexp.MustCompile(`:(\w+)`)
	matches := re.FindAllStringSubmatch(line, -1)

	groups := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			groups = append(groups, match[1])
		}
	}

	if len(groups) == 0 {
		return []string{"default"}
	}

	return groups
}

// parseGemLine parses gem declarations
// Examples:
//
//	gem 'rails', '~> 7.0'
//	gem 'devise', '>= 4.8', groups: [:default, :production]
//	gem 'capybara', require: false
//	gem 'state_machines', github: 'state-machines/state_machines', branch: 'master'
//	gem 'commonshare_cms', path: 'components/cms'
func (p *GemfileParser) parseGemLine(line string, currentGroups []string, currentSource *Source) (*GemDependency, error) {
	// Basic gem pattern: gem 'name'
	nameRe := regexp.MustCompile(`gem\s+['"]([^'"]+)['"]`)
	nameMatches := nameRe.FindStringSubmatch(line)
	if len(nameMatches) < 2 {
		return nil, fmt.Errorf("invalid gem line: %s", line)
	}

	dep := &GemDependency{
		Name:   nameMatches[1],
		Groups: make([]string, len(currentGroups)),
	}
	copy(dep.Groups, currentGroups)

	// Extract version constraints
	dep.Constraints = p.extractVersionConstraints(line)

	// Extract special options (git, path, etc.)
	dep.Source = p.extractSource(line)

	// If no explicit source was found but we're inside a source block, use currentSource
	if dep.Source == nil && currentSource != nil {
		// Create a copy of the current source for this gem
		sourceCopy := *currentSource
		dep.Source = &sourceCopy
	}

	dep.Require = p.extractRequire(line)

	// Extract group overrides
	if groups := p.extractGroupOverrides(line); len(groups) > 0 {
		dep.Groups = groups
	}

	// Extract platform restrictions
	dep.Platforms = p.extractPlatforms(line)

	return dep, nil
}

// extractVersionConstraints extracts version constraints from gem line
func (p *GemfileParser) extractVersionConstraints(line string) []string {
	// First, remove the gem name to avoid matching it
	nameRe := regexp.MustCompile(`gem\s+['"][^'"]+['"],?\s*`)
	remaining := nameRe.ReplaceAllString(line, "")

	// Pattern to match version strings (not including options like require:, github:, etc.)
	// Stop at first option keyword
	optionsStart := strings.Index(remaining, "require:")
	if optionsStart == -1 {
		optionsStart = strings.Index(remaining, "github:")
	}
	if optionsStart == -1 {
		optionsStart = strings.Index(remaining, "path:")
	}
	if optionsStart == -1 {
		optionsStart = strings.Index(remaining, "groups:")
	}
	if optionsStart == -1 {
		optionsStart = strings.Index(remaining, "platforms:")
	}

	versionPart := remaining
	if optionsStart != -1 {
		versionPart = remaining[:optionsStart]
	}

	// Extract all quoted strings from the version part
	re := regexp.MustCompile(`['"]([^'"]+)['"]`)
	matches := re.FindAllStringSubmatch(versionPart, -1)

	constraints := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			constraints = append(constraints, match[1])
		}
	}

	return constraints
}

// extractSource extracts git/path source information
func (p *GemfileParser) extractSource(line string) *Source {
	// Check for github source: github: 'user/repo'
	if githubRe := regexp.MustCompile(`github:\s*['"]([^'"]+)['"]`); githubRe.MatchString(line) {
		matches := githubRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			source := &Source{
				Type: "git",
				URL:  fmt.Sprintf("https://github.com/%s.git", matches[1]),
			}

			// Extract branch/tag/ref
			if branchRe := regexp.MustCompile(`branch:\s*['"]([^'"]+)['"]`); branchRe.MatchString(line) {
				branchMatches := branchRe.FindStringSubmatch(line)
				if len(branchMatches) > 1 {
					source.Branch = branchMatches[1]
				}
			}

			return source
		}
	}

	// Check for git source: git: 'https://...'
	if gitRe := regexp.MustCompile(`git:\s*['"]([^'"]+)['"]`); gitRe.MatchString(line) {
		matches := gitRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return &Source{
				Type: "git",
				URL:  matches[1],
			}
		}
	}

	// Check for path source: path: 'local/path'
	if pathRe := regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`); pathRe.MatchString(line) {
		matches := pathRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return &Source{
				Type: "path",
				URL:  matches[1],
			}
		}
	}

	return nil
}

// extractRequire extracts require option
func (p *GemfileParser) extractRequire(line string) *string {
	// require: false
	if requireRe := regexp.MustCompile(`require:\s*(false|['"][^'"]*['"])`); requireRe.MatchString(line) {
		matches := requireRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			require := matches[1]
			if require == falseValue {
				require = ""
			} else {
				// Remove quotes
				require = strings.Trim(require, `'"`)
			}
			return &require
		}
	}

	return nil
}

// extractGroupOverrides extracts group overrides from gem line
func (p *GemfileParser) extractGroupOverrides(line string) []string {
	// groups: [:development, :test]
	if groupsRe := regexp.MustCompile(`groups?:\s*\[([^\]]+)\]`); groupsRe.MatchString(line) {
		matches := groupsRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			groupStr := matches[1]
			groupRe := regexp.MustCompile(`:(\w+)`)
			groupMatches := groupRe.FindAllStringSubmatch(groupStr, -1)

			groups := make([]string, 0, len(groupMatches))
			for _, match := range groupMatches {
				if len(match) > 1 {
					groups = append(groups, match[1])
				}
			}
			return groups
		}
	}

	return nil
}

// extractPlatforms extracts platform restrictions from gem line
func (p *GemfileParser) extractPlatforms(line string) []string {
	// platforms: [:windows_31, :jruby]
	if platformsRe := regexp.MustCompile(`platforms?:\s*\[([^\]]+)\]`); platformsRe.MatchString(line) {
		matches := platformsRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			platformStr := matches[1]
			platformRe := regexp.MustCompile(`:(\w+)`)
			platformMatches := platformRe.FindAllStringSubmatch(platformStr, -1)

			platforms := make([]string, 0, len(platformMatches))
			for _, match := range platformMatches {
				if len(match) > 1 {
					platforms = append(platforms, match[1])
				}
			}
			return platforms
		}
	}

	// platforms: :jruby (single platform)
	if platformRe := regexp.MustCompile(`platforms?:\s*:(\w+)`); platformRe.MatchString(line) {
		matches := platformRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return []string{matches[1]}
		}
	}

	return nil
}

// parseRubyVersion extracts Ruby version requirement
func (p *GemfileParser) parseRubyVersion(line string) string {
	re := regexp.MustCompile(`ruby\s+['"]([^'"]+)['"]`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// parseGemspecDirective parses gemspec directive
// Examples:
//
//	gemspec
//	gemspec path: "components/payment"
//	gemspec name: "payment_core"
//	gemspec development_group: :ci
//	gemspec path: ".", name: "my_gem", development_group: :test
func (p *GemfileParser) parseGemspecDirective(line string) *GemspecReference {
	gemspecRef := &GemspecReference{
		Path:             ".",
		DevelopmentGroup: "development",
		Glob:             "{,*,*/*}.gemspec",
	}

	// If it's just "gemspec" with no options, return defaults
	if strings.TrimSpace(line) == gemspecDirective {
		return gemspecRef
	}

	// Parse path option
	if pathRe := regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`); pathRe.MatchString(line) {
		matches := pathRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			gemspecRef.Path = matches[1]
		}
	}

	// Parse name option
	if nameRe := regexp.MustCompile(`name:\s*['"]([^'"]+)['"]`); nameRe.MatchString(line) {
		matches := nameRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			gemspecRef.Name = matches[1]
		}
	}

	// Parse development_group option
	if devGroupRe := regexp.MustCompile(`development_group:\s*:(\w+)`); devGroupRe.MatchString(line) {
		matches := devGroupRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			gemspecRef.DevelopmentGroup = matches[1]
		}
	}

	// Parse glob option
	if globRe := regexp.MustCompile(`glob:\s*['"]([^'"]+)['"]`); globRe.MatchString(line) {
		matches := globRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			gemspecRef.Glob = matches[1]
		}
	}

	return gemspecRef
}

// parseVariable parses variable assignments like: rails_version = '~> 8.0.1'
func (p *GemfileParser) parseVariable(line string) (varName, varValue string) {
	re := regexp.MustCompile(`^(\w+)\s*=\s*['"]([^'"]+)['"]`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}
	return "", ""
}

// expandVariables replaces variable references with their values
func (p *GemfileParser) expandVariables(line string, variables map[string]string) string {
	// Replace variable references
	for varName, varValue := range variables {
		// Match variable name as a standalone word (not part of a string)
		pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(varName))
		re := regexp.MustCompile(pattern)

		// Only replace if not inside quotes
		if !p.isInsideQuotes(line, varName) {
			line = re.ReplaceAllString(line, fmt.Sprintf("'%s'", varValue))
		}
	}
	return line
}

// isInsideQuotes checks if a variable name appears inside quoted strings
func (p *GemfileParser) isInsideQuotes(line, varName string) bool {
	// Simple check: if the variable appears between quotes, don't replace
	index := strings.Index(line, varName)
	if index == -1 {
		return false
	}

	// Count quotes before the variable
	beforeVar := line[:index]
	singleQuotes := strings.Count(beforeVar, "'")
	doubleQuotes := strings.Count(beforeVar, "\"")

	// If odd number of quotes, we're inside a quoted string
	return (singleQuotes%2 == 1) || (doubleQuotes%2 == 1)
}

// handleGemspecDirective handles gemspec directive parsing and loading
func (p *GemfileParser) handleGemspecDirective(line string, result *ParsedGemfile) error {
	gemspecRef := p.parseGemspecDirective(line)
	if gemspecRef != nil {
		result.Gemspecs = append(result.Gemspecs, *gemspecRef)
		// Load dependencies from the gemspec
		deps, err := LoadGemspecDependencies(*gemspecRef, filepath.Dir(p.filepath))
		if err != nil {
			// Log warning but don't fail - gemspec might not exist yet during development
			return nil
		}
		result.Dependencies = append(result.Dependencies, deps...)
	}
	return nil
}
