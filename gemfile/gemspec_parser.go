// Package gemfile provides functionality to parse Ruby .gemspec files
package gemfile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// defaultGlobPattern is the default glob pattern for finding gemspec files
	defaultGlobPattern = "{,*,*/*}.gemspec"
	// developmentGroup is the default group name for development dependencies
	developmentGroup = "development"
)

// GemspecParser handles parsing of .gemspec files
type GemspecParser struct {
	filepath string
}

// NewGemspecParser creates a new gemspec parser for the given file path
func NewGemspecParser(filePath string) *GemspecParser {
	return &GemspecParser{filepath: filePath}
}

// gemspecJSON represents the JSON structure returned by Ruby
type gemspecJSON struct {
	Name                    string            `json:"name"`
	Version                 string            `json:"version"`
	Summary                 string            `json:"summary"`
	Description             string            `json:"description"`
	Authors                 []string          `json:"authors"`
	Email                   []string          `json:"email"`
	Homepage                string            `json:"homepage"`
	License                 string            `json:"license"`
	Licenses                []string          `json:"licenses"`
	RequiredRubyVersion     string            `json:"required_ruby_version"`
	Files                   []string          `json:"files"`
	Metadata                map[string]string `json:"metadata"`
	RuntimeDependencies     []dependencyJSON  `json:"runtime_dependencies"`
	DevelopmentDependencies []dependencyJSON  `json:"development_dependencies"`
}

type dependencyJSON struct {
	Name         string   `json:"name"`
	Requirements []string `json:"requirements"`
}

// Parse parses a .gemspec file and returns structured data
func (p *GemspecParser) Parse() (*GemspecFile, error) {
	content, err := os.ReadFile(p.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read gemspec file: %w", err)
	}

	// NOTE: Tree-sitter was not initially included in gemfile-go as it was assumed
	// to be needed only for the ore-full ecosystem. However, it proved essential for
	// reliable gemspec parsing due to the complexity of Ruby DSL syntax.
	//
	// Tree-sitter parsing may fail with non-orthodox Ruby coding patterns such as:
	// - Dynamic version loading: spec.version = File.read('VERSION').strip
	// - Conditional dependencies: if RUBY_VERSION >= "2.7" then add_dependency...
	// - Metaprogramming: deps.each { |d| spec.add_dependency d }
	// - Non-standard patterns: Gem::Specification.new.tap do |spec|...
	// - Heredocs, string interpolation, or complex Ruby expressions
	//
	// When tree-sitter fails, we fall back to Ruby execution (which can evaluate
	// dynamic code) or regex parsing (for simple static patterns).

	// Try tree-sitter first (most reliable for standard patterns, no external dependencies)
	tsParser := NewTreeSitterGemspecParser(content)
	gemspec, err := tsParser.ParseWithTreeSitter()
	if err == nil && gemspec.Name != "" {
		return gemspec, nil
	}

	// If tree-sitter fails or doesn't find data, try Ruby
	return p.parseWithRuby()
}

// parseWithRuby attempts to parse the gemspec using Ruby execution
func (p *GemspecParser) parseWithRuby() (*GemspecFile, error) {
	rubyScript := `
require 'json'
require 'rubygems'

begin
  spec_file = ARGV[0]
  spec = Gem::Specification.load(spec_file)

  if spec.nil?
    puts JSON.generate({error: "Failed to load gemspec"})
    exit 1
  end

  # Convert spec to a hash structure
  output = {
    name: spec.name,
    version: spec.version.to_s,
    summary: spec.summary || "",
    description: spec.description || "",
    authors: Array(spec.authors),
    email: Array(spec.email),
    homepage: spec.homepage || "",
    license: spec.license || (spec.licenses.first if spec.licenses && !spec.licenses.empty?) || "",
    licenses: Array(spec.licenses),
    required_ruby_version: spec.required_ruby_version ? spec.required_ruby_version.to_s : "",
    files: spec.files || [],
    metadata: spec.metadata || {},
    runtime_dependencies: spec.runtime_dependencies.map do |dep|
      {
        name: dep.name,
        requirements: dep.requirements_list
      }
    end,
    development_dependencies: spec.development_dependencies.map do |dep|
      {
        name: dep.name,
        requirements: dep.requirements_list
      }
    end
  }

  puts JSON.generate(output)
rescue => e
  puts JSON.generate({error: e.message})
  exit 1
end
`

	// Execute Ruby script with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ruby", "-e", rubyScript, p.filepath) // #nosec G204 - Ruby is required for evaluating dynamic gemspecs
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	cmd.Dir = filepath.Dir(p.filepath)

	err := cmd.Run()
	if err != nil {
		// If Ruby is not available or script failed, fall back to basic regex parsing
		return p.fallbackParse()
	}

	// Parse JSON output
	var result gemspecJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		// If JSON parsing fails, try fallback
		return p.fallbackParse()
	}

	// Check for error in JSON response
	if errorMsg, ok := result.Metadata["error"]; ok {
		return nil, fmt.Errorf("ruby parsing error: %s", errorMsg)
	}

	return p.convertJSONToGemspecFile(&result), nil
}

// convertJSONToGemspecFile converts the JSON result to our GemspecFile structure
func (p *GemspecParser) convertJSONToGemspecFile(result *gemspecJSON) *GemspecFile {
	gemspec := &GemspecFile{
		Name:                result.Name,
		Version:             result.Version,
		Summary:             result.Summary,
		Description:         result.Description,
		Authors:             result.Authors,
		Email:               result.Email,
		Homepage:            result.Homepage,
		License:             result.License,
		RequiredRubyVersion: result.RequiredRubyVersion,
		Files:               result.Files,
		Metadata:            result.Metadata,
	}

	// Convert runtime dependencies
	for _, dep := range result.RuntimeDependencies {
		gemDep := GemDependency{
			Name:        dep.Name,
			Constraints: dep.Requirements,
		}
		gemspec.RuntimeDependencies = append(gemspec.RuntimeDependencies, gemDep)
	}

	// Convert development dependencies
	for _, dep := range result.DevelopmentDependencies {
		gemDep := GemDependency{
			Name:        dep.Name,
			Constraints: dep.Requirements,
		}
		gemspec.DevelopmentDependencies = append(gemspec.DevelopmentDependencies, gemDep)
	}

	return gemspec
}

// fallbackParse provides basic regex-based parsing when Ruby is not available
func (p *GemspecParser) fallbackParse() (*GemspecFile, error) {
	content, err := os.ReadFile(p.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read gemspec file: %w", err)
	}

	gemspec := &GemspecFile{
		Authors:                 []string{},
		Email:                   []string{},
		RuntimeDependencies:     []GemDependency{},
		DevelopmentDependencies: []GemDependency{},
		Files:                   []string{},
		Metadata:                make(map[string]string),
	}

	contentStr := string(content)

	// Extract all gemspec fields
	p.extractSimpleFields(contentStr, gemspec)
	p.extractAuthors(contentStr, gemspec)
	p.extractEmail(contentStr, gemspec)
	p.extractDependencies(contentStr, gemspec)
	p.extractMetadata(contentStr, gemspec)

	return gemspec, nil
}

// extractSimpleFields extracts simple string fields from gemspec content
func (p *GemspecParser) extractSimpleFields(content string, gemspec *GemspecFile) {
	patterns := map[string]*regexp.Regexp{
		"name":                  regexp.MustCompile(`spec\.name\s*=\s*['"](.*?)['"]`),
		"version":               regexp.MustCompile(`spec\.version\s*=\s*['"](.*?)['"]`),
		"summary":               regexp.MustCompile(`spec\.summary\s*=\s*['"](.*?)['"]`),
		"description":           regexp.MustCompile(`spec\.description\s*=\s*['"](.*?)['"]`),
		"homepage":              regexp.MustCompile(`spec\.homepage\s*=\s*['"](.*?)['"]`),
		"license":               regexp.MustCompile(`spec\.licenses?\s*=\s*['"](.*?)['"]`),
		"required_ruby_version": regexp.MustCompile(`spec\.required_ruby_version\s*=\s*['"](.*?)['"]`),
	}

	if match := patterns["name"].FindStringSubmatch(content); len(match) > 1 {
		gemspec.Name = match[1]
	}
	if match := patterns["version"].FindStringSubmatch(content); len(match) > 1 {
		gemspec.Version = match[1]
	} else if match := regexp.MustCompile(`spec\.version\s*=\s*([\w:]+)`).FindStringSubmatch(content); len(match) > 1 {
		gemspec.Version = match[1]
	}
	if match := patterns["summary"].FindStringSubmatch(content); len(match) > 1 {
		gemspec.Summary = match[1]
	}
	if match := patterns["description"].FindStringSubmatch(content); len(match) > 1 {
		gemspec.Description = match[1]
	}
	if match := patterns["homepage"].FindStringSubmatch(content); len(match) > 1 {
		gemspec.Homepage = match[1]
	}
	if match := patterns["license"].FindStringSubmatch(content); len(match) > 1 {
		gemspec.License = match[1]
	}
	if match := patterns["required_ruby_version"].FindStringSubmatch(content); len(match) > 1 {
		gemspec.RequiredRubyVersion = match[1]
	}
}

// extractAuthors extracts author information from gemspec content
func (p *GemspecParser) extractAuthors(content string, gemspec *GemspecFile) {
	if match := regexp.MustCompile(`spec\.authors?\s*=\s*\[(.*?)\]`).FindStringSubmatch(content); len(match) > 1 {
		gemspec.Authors = parseQuotedArray(match[1])
	} else if match := regexp.MustCompile(`spec\.authors?\s*=\s*['"](.*?)['"]`).FindStringSubmatch(content); len(match) > 1 {
		gemspec.Authors = []string{match[1]}
	}
}

// extractEmail extracts email information from gemspec content
func (p *GemspecParser) extractEmail(content string, gemspec *GemspecFile) {
	if match := regexp.MustCompile(`spec\.email\s*=\s*\[(.*?)\]`).FindStringSubmatch(content); len(match) > 1 {
		gemspec.Email = parseQuotedArray(match[1])
	} else if match := regexp.MustCompile(`spec\.email\s*=\s*['"](.*?)['"]`).FindStringSubmatch(content); len(match) > 1 {
		gemspec.Email = []string{match[1]}
	}
}

// extractDependencies extracts runtime and development dependencies from gemspec content
func (p *GemspecParser) extractDependencies(content string, gemspec *GemspecFile) {
	depPattern := regexp.MustCompile(`spec\.add_(?:(runtime|development)_)?dependency\s*\(?\s*['"]([\w\-]+)['"]([^)]*)\)?`)
	depMatches := depPattern.FindAllStringSubmatch(content, -1)

	for _, match := range depMatches {
		if len(match) >= 3 {
			dep := GemDependency{
				Name:        match[2],
				Constraints: extractVersionConstraints(match[3]),
			}

			if match[1] == developmentGroup {
				gemspec.DevelopmentDependencies = append(gemspec.DevelopmentDependencies, dep)
			} else {
				gemspec.RuntimeDependencies = append(gemspec.RuntimeDependencies, dep)
			}
		}
	}
}

// extractMetadata extracts metadata from gemspec content
func (p *GemspecParser) extractMetadata(content string, gemspec *GemspecFile) {
	metadataPattern := regexp.MustCompile(`spec\.metadata\[['"](.*?)['"]\]\s*=\s*['"](.*?)['"]`)
	metadataMatches := metadataPattern.FindAllStringSubmatch(content, -1)
	for _, match := range metadataMatches {
		if len(match) > 2 {
			gemspec.Metadata[match[1]] = match[2]
		}
	}
}

// parseQuotedArray parses an array of quoted strings
func parseQuotedArray(arrayContent string) []string {
	var result []string
	pattern := regexp.MustCompile(`['"](.*?)['"]`)
	matches := pattern.FindAllStringSubmatch(arrayContent, -1)
	for _, match := range matches {
		if len(match) > 1 {
			result = append(result, match[1])
		}
	}
	return result
}

// extractVersionConstraints extracts version constraints from a dependency line remainder
func extractVersionConstraints(remainder string) []string {
	var constraints []string
	pattern := regexp.MustCompile(`['"](.*?)['"]`)
	matches := pattern.FindAllStringSubmatch(remainder, -1)
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			constraints = append(constraints, match[1])
		}
	}
	return constraints
}

// FindGemspecs finds all gemspec files matching the given pattern in a directory
func FindGemspecs(basePath, glob, name string) ([]string, error) {
	if glob == "" {
		// Default glob pattern as per Bundler
		glob = defaultGlobPattern
	}

	if basePath == "" {
		basePath = "."
	}

	// Expand the glob pattern
	patterns := expandGlobPattern(basePath, glob)

	var gemspecs []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
		}

		for _, match := range matches {
			// If a specific name is requested, filter by it
			if name != "" {
				base := filepath.Base(match)
				expectedFile := name + ".gemspec"
				if base != expectedFile {
					continue
				}
			}
			gemspecs = append(gemspecs, match)
		}
	}

	return gemspecs, nil
}

// expandGlobPattern expands a bundler-style glob pattern into filepath.Glob compatible patterns
func expandGlobPattern(basePath, glob string) []string {
	// Handle bundler's {,*,*/*} syntax
	if strings.Contains(glob, "{") && strings.Contains(glob, "}") {
		// Extract the content between braces
		start := strings.Index(glob, "{")
		end := strings.Index(glob, "}")

		if start < end {
			prefix := glob[:start]
			suffix := glob[end+1:]
			options := glob[start+1 : end]

			// Split options by comma
			parts := strings.Split(options, ",")
			var patterns []string

			for _, part := range parts {
				part = strings.TrimSpace(part)
				pattern := filepath.Join(basePath, prefix+part+suffix)
				patterns = append(patterns, pattern)
			}

			return patterns
		}
	}

	// If no brace expansion, return as is
	return []string{filepath.Join(basePath, glob)}
}

// LoadGemspecDependencies loads dependencies from a gemspec directive
func LoadGemspecDependencies(gemspecRef GemspecReference, gemfileDir string) ([]GemDependency, error) {
	// Find gemspec files
	searchPath := gemspecRef.Path
	if searchPath == "" {
		searchPath = gemfileDir
	} else if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(gemfileDir, searchPath)
	}

	gemspecs, err := FindGemspecs(searchPath, gemspecRef.Glob, gemspecRef.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find gemspecs: %w", err)
	}

	if len(gemspecs) == 0 {
		return nil, fmt.Errorf("no gemspec files found in %s", searchPath)
	}

	if len(gemspecs) > 1 && gemspecRef.Name == "" {
		return nil, fmt.Errorf("multiple gemspec files found, please specify a name: %v", gemspecs)
	}

	// Parse the gemspec file
	parser := NewGemspecParser(gemspecs[0])
	gemspecFile, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse gemspec %s: %w", gemspecs[0], err)
	}

	var dependencies []GemDependency

	// Add runtime dependencies (no group specification)
	for _, dep := range gemspecFile.RuntimeDependencies {
		// Runtime deps go to default group
		dep.Groups = []string{"default"}
		dependencies = append(dependencies, dep)
	}

	// Add development dependencies to the specified group
	devGroup := gemspecRef.DevelopmentGroup
	if devGroup == "" {
		devGroup = developmentGroup
	}

	for _, dep := range gemspecFile.DevelopmentDependencies {
		dep.Groups = []string{devGroup}
		dependencies = append(dependencies, dep)
	}

	// Also add the gem itself as a path dependency
	gemPath := filepath.Dir(gemspecs[0])
	selfDep := GemDependency{
		Name: gemspecFile.Name,
		Source: &Source{
			Type: "path",
			URL:  gemPath,
		},
		Groups: []string{"default"},
	}
	dependencies = append([]GemDependency{selfDep}, dependencies...)

	return dependencies, nil
}
