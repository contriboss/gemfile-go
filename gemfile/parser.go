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
	PostInstallMessage      string            // Post-install message
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
// conditionalState tracks if/elsif/else conditional branching
type conditionalState struct {
	inConditional bool // Inside an if/elsif/else block
	branchActive  bool // Current branch should be processed
	conditionMet  bool // A true branch was already found (for elsif/else)
	depth         int  // Nesting depth for nested conditionals
}

// conditionalHandler manages conditional block processing
type conditionalHandler struct {
	parser *GemfileParser
	stack  []conditionalState
}

func newConditionalHandler(p *GemfileParser) *conditionalHandler {
	return &conditionalHandler{parser: p, stack: []conditionalState{}}
}

// shouldProcess returns true if the current line should be processed
func (h *conditionalHandler) shouldProcess() bool {
	for _, cs := range h.stack {
		if !cs.branchActive {
			return false
		}
	}
	return true
}

// handleLine processes conditional keywords, returns true if the line was a conditional keyword
func (h *conditionalHandler) handleLine(line string) (handled, skipLine bool) {
	if strings.HasPrefix(line, "if ") {
		condition := strings.TrimPrefix(line, "if ")
		isTrue := h.parser.evaluateCondition(condition)
		h.stack = append(h.stack, conditionalState{
			inConditional: true,
			branchActive:  isTrue,
			conditionMet:  isTrue,
			depth:         1,
		})
		return true, true
	}
	if strings.HasPrefix(line, "elsif ") && len(h.stack) > 0 {
		cs := &h.stack[len(h.stack)-1]
		if cs.conditionMet {
			cs.branchActive = false
		} else {
			condition := strings.TrimPrefix(line, "elsif ")
			isTrue := h.parser.evaluateCondition(condition)
			cs.branchActive = isTrue
			if isTrue {
				cs.conditionMet = true
			}
		}
		return true, true
	}
	if line == "else" && len(h.stack) > 0 {
		cs := &h.stack[len(h.stack)-1]
		cs.branchActive = !cs.conditionMet
		return true, true
	}
	if line == endKeyword && len(h.stack) > 0 {
		cs := &h.stack[len(h.stack)-1]
		if cs.depth > 0 {
			cs.depth--
			if cs.depth == 0 {
				h.stack = h.stack[:len(h.stack)-1]
				return true, true
			}
		}
	}
	// Track nested blocks within conditionals
	if len(h.stack) > 0 && (strings.HasSuffix(line, " do") || strings.HasSuffix(line, " do |")) {
		h.stack[len(h.stack)-1].depth++
	}
	return false, false
}

// handleInactiveLine handles end keywords in inactive branches
func (h *conditionalHandler) handleInactiveLine(line string) {
	if line == endKeyword && len(h.stack) > 0 {
		cs := &h.stack[len(h.stack)-1]
		cs.depth--
		if cs.depth == 0 {
			h.stack = h.stack[:len(h.stack)-1]
		}
	}
}

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
	condHandler := newConditionalHandler(p)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle if/elsif/else/end
		if handled, skip := condHandler.handleLine(line); handled && skip {
			continue
		}

		// Skip lines in inactive conditional branches
		if !condHandler.shouldProcess() {
			condHandler.handleInactiveLine(line)
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

// evaluateCondition evaluates simple Ruby conditions (ENV comparisons)
func (p *GemfileParser) evaluateCondition(condition string) bool {
	condition = strings.TrimSpace(condition)

	// Handle ENV["VAR"] == "value"
	envEqRe := regexp.MustCompile(`ENV\s*\[\s*["']([^"']+)["']\s*\]\s*==\s*["']([^"']*)["']`)
	if matches := envEqRe.FindStringSubmatch(condition); len(matches) == 3 {
		envVal := os.Getenv(matches[1])
		return envVal == matches[2]
	}

	// Handle ENV["VAR"] != "value"
	envNeqRe := regexp.MustCompile(`ENV\s*\[\s*["']([^"']+)["']\s*\]\s*!=\s*["']([^"']*)["']`)
	if matches := envNeqRe.FindStringSubmatch(condition); len(matches) == 3 {
		envVal := os.Getenv(matches[1])
		return envVal != matches[2]
	}

	// Handle ENV["VAR"] (truthy check)
	envTruthyRe := regexp.MustCompile(`^ENV\s*\[\s*["']([^"']+)["']\s*\]$`)
	if matches := envTruthyRe.FindStringSubmatch(condition); len(matches) == 2 {
		return os.Getenv(matches[1]) != ""
	}

	// Unknown condition - default to true (include all gems)
	return true
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
	// Strip inline comments first
	if idx := strings.Index(line, "#"); idx != -1 {
		line = line[:idx]
	}

	// Remove the 'group' keyword
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "group ")

	// Trim everything after the 'do' token (not just suffix)
	if idx := strings.Index(line, " do"); idx != -1 {
		line = line[:idx]
	} else if idx := strings.Index(line, "do"); idx != -1 {
		// Handle cases like 'group(:dev)do' - although rare in Gemfiles
		// Check if 'do' is a separate word or at least not part of a symbol/string
		// For simplicity, we can check if it's preceded by space or closing paren/quote
		line = line[:idx]
	}

	// Extract group names using a more precise regex
	// Supports symbols like :test or strings like "test"
	// match :(\w+) and ['"](\w+)['"]
	re := regexp.MustCompile(`:(\w+)|['"](\w+)['"]`)
	matches := re.FindAllStringSubmatch(line, -1)

	groups := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			groups = append(groups, match[1])
		} else if len(match) > 2 && match[2] != "" {
			groups = append(groups, match[2])
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

	// Stop at first option keyword or hash rocket
	// Supports both symbolized hashes and hash rockets
	optionRe := regexp.MustCompile(`(?::\w+\s*=>|[\w:]+:)`)
	if loc := optionRe.FindStringIndex(remaining); loc != nil {
		remaining = remaining[:loc[0]]
	}

	// Extract all quoted strings from the version part
	re := regexp.MustCompile(`['"]([^'"]+)['"]`)
	matches := re.FindAllStringSubmatch(remaining, -1)

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
	// Check for github source: github: 'user/repo' or :github => 'user/repo'
	if githubRe := regexp.MustCompile(`(?::github\s*=>|github:)\s*['"]([^'"]+)['"]`); githubRe.MatchString(line) {
		matches := githubRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			source := &Source{
				Type: "git",
				URL:  fmt.Sprintf("https://github.com/%s.git", matches[1]),
			}

			// Extract branch/tag/ref
			if branchRe := regexp.MustCompile(`(?::branch\s*=>|branch:)\s*['"]([^'"]+)['"]`); branchRe.MatchString(line) {
				branchMatches := branchRe.FindStringSubmatch(line)
				if len(branchMatches) > 1 {
					source.Branch = branchMatches[1]
				}
			}

			return source
		}
	}

	// Check for git source: git: 'https://...' or :git => 'https://...'
	if gitRe := regexp.MustCompile(`(?::git\s*=>|git:)\s*['"]([^'"]+)['"]`); gitRe.MatchString(line) {
		matches := gitRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			source := &Source{
				Type: "git",
				URL:  matches[1],
			}

			// Extract branch/tag/ref for generic git source too
			if branchRe := regexp.MustCompile(`(?::branch\s*=>|branch:)\s*['"]([^'"]+)['"]`); branchRe.MatchString(line) {
				branchMatches := branchRe.FindStringSubmatch(line)
				if len(branchMatches) > 1 {
					source.Branch = branchMatches[1]
				}
			}

			return source
		}
	}

	// Check for path source: path: 'local/path' or :path => 'local/path'
	if pathRe := regexp.MustCompile(`(?::path\s*=>|path:)\s*['"]([^'"]+)['"]`); pathRe.MatchString(line) {
		matches := pathRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return &Source{
				Type: "path",
				URL:  matches[1],
			}
		}
	}

	// Check for inline rubygems source: source: 'https://...' or :source => 'https://...'
	if sourceRe := regexp.MustCompile(`(?::source\s*=>|source:)\s*['"]([^'"]+)['"]`); sourceRe.MatchString(line) {
		matches := sourceRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			return &Source{
				Type: "rubygems",
				URL:  matches[1],
			}
		}
	}

	return nil
}

// extractRequire extracts require option
func (p *GemfileParser) extractRequire(line string) *string {
	// require: false or :require => false or require: 'foo' or :require => 'foo'
	if requireRe := regexp.MustCompile(`(?::require\s*=>|require:)\s*(false|['"][^'"]*['"])`); requireRe.MatchString(line) {
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
	// groups: [:development, :test] or :groups => [:development, :test]
	pattern := `(?::groups?\s*=>|groups?:\s*)\s*\[([^\]]+)\]`
	return p.extractArrayFromOption(line, pattern)
}

// extractPlatforms extracts platform restrictions from gem line
func (p *GemfileParser) extractPlatforms(line string) []string {
	// platforms: [:windows_31, :jruby] or :platforms => [:windows_31, :jruby]
	pattern := `(?::platforms?\s*=>|platforms?:\s*)\s*\[([^\]]+)\]`
	return p.extractArrayFromOption(line, pattern)
}

// extractArrayFromOption extracts an array of symbols/strings from a line using the given pattern
func (p *GemfileParser) extractArrayFromOption(line, optionPattern string) []string {
	re := regexp.MustCompile(optionPattern)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		content := matches[1]
		// Match :symbol or "string" or 'string'
		itemRe := regexp.MustCompile(`:(\w+)|['"](\w+)['"]`)
		itemMatches := itemRe.FindAllStringSubmatch(content, -1)

		items := make([]string, 0, len(itemMatches))
		for _, match := range itemMatches {
			if len(match) > 1 && match[1] != "" {
				items = append(items, match[1])
			} else if len(match) > 2 && match[2] != "" {
				items = append(items, match[2])
			}
		}
		return items
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

	// Parse various options
	p.parseGemspecOption(line, "path", &gemspecRef.Path)
	p.parseGemspecOption(line, "name", &gemspecRef.Name)
	p.parseGemspecOption(line, "development_group", &gemspecRef.DevelopmentGroup)
	p.parseGemspecOption(line, "glob", &gemspecRef.Glob)

	return gemspecRef
}

// parseGemspecOption parses a single option from a gemspec directive
func (p *GemfileParser) parseGemspecOption(line, optionName string, target *string) {
	// Pattern for both symbol and keyword syntax:
	// :option => "value", :option => :value, option: "value", option: :value
	pattern := fmt.Sprintf(`(?::%s\s*=>|%s:)\s*(?:['"]([^'"]+)['"]|:?(\w+))`, optionName, optionName)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		if matches[1] != "" {
			*target = matches[1]
		} else if len(matches) > 2 && matches[2] != "" {
			*target = matches[2]
		}
	}
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
	// First, expand ENV.fetch("VAR", "default") calls
	line = p.expandEnvFetch(line)

	// Then, expand ENV["VAR"] calls
	line = p.expandEnvBracket(line)

	// Finally, replace variable references
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

// expandEnvFetch expands ENV.fetch("VAR", "default") calls
func (p *GemfileParser) expandEnvFetch(line string) string {
	// Match ENV.fetch("VAR_NAME", "default_value") or ENV.fetch("VAR_NAME")
	re := regexp.MustCompile(`ENV\.fetch\s*\(\s*["']([^"']+)["']\s*(?:,\s*["']([^"']*)["'])?\s*\)`)

	return re.ReplaceAllStringFunc(line, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		envVarName := submatches[1]
		defaultValue := ""
		if len(submatches) > 2 {
			defaultValue = submatches[2]
		}

		// Look up environment variable
		if value := os.Getenv(envVarName); value != "" {
			return fmt.Sprintf("'%s'", value)
		}

		// Return default value if provided
		if defaultValue != "" {
			return fmt.Sprintf("'%s'", defaultValue)
		}

		return match // Return original if no value found
	})
}

// expandEnvBracket expands ENV["VAR"] calls
func (p *GemfileParser) expandEnvBracket(line string) string {
	// Match ENV["VAR_NAME"]
	re := regexp.MustCompile(`ENV\s*\[\s*["']([^"']+)["']\s*\]`)

	return re.ReplaceAllStringFunc(line, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		envVarName := submatches[1]

		// Look up environment variable
		if value := os.Getenv(envVarName); value != "" {
			return fmt.Sprintf("'%s'", value)
		}

		return "" // Return empty string if env var not set
	})
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
