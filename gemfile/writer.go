package gemfile

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	falseValue     = "false"
	endKeyword     = "end"
	rubygemsSource = "rubygems"
	rubygemsURL    = "https://rubygems.org"
	pathSource     = "path"
	defaultGroup   = "default"
)

// GemfileWriter handles writing and modifying Gemfiles
type GemfileWriter struct {
	filepath string
	content  []string
}

// NewGemfileWriter creates a new writer for the given Gemfile path
func NewGemfileWriter(filepath string) *GemfileWriter {
	return &GemfileWriter{filepath: filepath}
}

// Load reads the current Gemfile content
func (w *GemfileWriter) Load() error {
	content, err := os.ReadFile(w.filepath)
	if err != nil {
		return fmt.Errorf("failed to read Gemfile: %w", err)
	}

	w.content = strings.Split(string(content), "\n")
	return nil
}

// AddGem adds a gem to the Gemfile
func (w *GemfileWriter) AddGem(dep *GemDependency) error {
	if err := w.Load(); err != nil {
		return err
	}

	// Check if gem already exists
	if w.hasGem(dep.Name) {
		return fmt.Errorf("gem %q already exists in Gemfile", dep.Name)
	}

	gemLine := w.formatGemLine(dep)

	// Find the best place to insert the gem
	insertIndex := w.findInsertionPoint(dep.Groups)

	// Insert the gem line
	w.content = append(w.content[:insertIndex], append([]string{gemLine}, w.content[insertIndex:]...)...)

	return w.save()
}

// RemoveGem removes a gem from the Gemfile
func (w *GemfileWriter) RemoveGem(gemName string) error {
	if err := w.Load(); err != nil {
		return err
	}

	found := false
	newContent := make([]string, 0, len(w.content))

	for _, line := range w.content {
		if w.isGemLine(line, gemName) {
			found = true
			// Skip this line
			continue
		}
		newContent = append(newContent, line)
	}

	if !found {
		return fmt.Errorf("gem %q not found in Gemfile", gemName)
	}

	w.content = newContent
	return w.save()
}

// hasGem checks if a gem already exists in the Gemfile
func (w *GemfileWriter) hasGem(gemName string) bool {
	for _, line := range w.content {
		if w.isGemLine(line, gemName) {
			return true
		}
	}
	return false
}

// isGemLine checks if a line declares the specified gem
func (w *GemfileWriter) isGemLine(line, gemName string) bool {
	// Match: gem 'gemname' or gem "gemname"
	pattern := fmt.Sprintf(`^\s*gem\s+['"]%s['"]`, regexp.QuoteMeta(gemName))
	matched, _ := regexp.MatchString(pattern, line)
	return matched
}

// formatGemLine formats a gem dependency into a Gemfile line string.
func (w *GemfileWriter) formatGemLine(dep *GemDependency) string {
	parts := []string{fmt.Sprintf("gem '%s'", dep.Name)}
	parts = append(parts, w.formatConstraints(dep)...)

	if source := w.formatSource(dep); source != "" {
		parts = append(parts, source)
	}

	if groups := w.formatGroups(dep); groups != "" {
		parts = append(parts, groups)
	}

	if require := w.formatRequire(dep); require != "" {
		parts = append(parts, require)
	}

	return strings.Join(parts, ", ")
}

// formatConstraints formats the version constraints for a gem.
func (w *GemfileWriter) formatConstraints(dep *GemDependency) []string {
	var parts []string
	for _, constraint := range dep.Constraints {
		parts = append(parts, fmt.Sprintf("'%s'", constraint))
	}
	return parts
}

// formatSource formats the source information for a gem.
func (w *GemfileWriter) formatSource(dep *GemDependency) string {
	if dep.Source == nil {
		return ""
	}

	var parts []string
	switch dep.Source.Type {
	case "git":
		if strings.Contains(dep.Source.URL, "github.com") {
			githubPath := extractGitHubPath(dep.Source.URL)
			if githubPath != "" {
				parts = append(parts, fmt.Sprintf("github: '%s'", githubPath))
			} else {
				parts = append(parts, fmt.Sprintf("git: '%s'", dep.Source.URL))
			}
		} else {
			parts = append(parts, fmt.Sprintf("git: '%s'", dep.Source.URL))
		}

		if dep.Source.Branch != "" {
			parts = append(parts, fmt.Sprintf("branch: '%s'", dep.Source.Branch))
		}
		if dep.Source.Tag != "" {
			parts = append(parts, fmt.Sprintf("tag: '%s'", dep.Source.Tag))
		}
		if dep.Source.Ref != "" {
			parts = append(parts, fmt.Sprintf("ref: '%s'", dep.Source.Ref))
		}
	case pathSource:
		parts = append(parts, fmt.Sprintf("path: '%s'", dep.Source.URL))
	case rubygemsSource:
		if dep.Source.URL != rubygemsURL {
			parts = append(parts, fmt.Sprintf("source: '%s'", dep.Source.URL))
		}
	}
	return strings.Join(parts, ", ")
}

// formatGroups formats the group information for a gem.
func (w *GemfileWriter) formatGroups(dep *GemDependency) string {
	if len(dep.Groups) > 0 && !isDefaultGroup(dep.Groups) {
		if len(dep.Groups) == 1 {
			return fmt.Sprintf("group: :%s", dep.Groups[0])
		}
		groupsStr := make([]string, len(dep.Groups))
		for i, group := range dep.Groups {
			groupsStr[i] = ":" + group
		}
		return fmt.Sprintf("groups: [%s]", strings.Join(groupsStr, ", "))
	}
	return ""
}

// formatRequire formats the require option for a gem.
func (w *GemfileWriter) formatRequire(dep *GemDependency) string {
	if dep.Require != nil {
		if *dep.Require == "" || *dep.Require == falseValue {
			return "require: false"
		}
		return fmt.Sprintf("require: '%s'", *dep.Require)
	}
	return ""
}

// extractGitHubPath extracts owner/repo from GitHub URLs
func extractGitHubPath(url string) string {
	// Convert https://github.com/owner/repo.git to owner/repo
	re := regexp.MustCompile(`github\.com[/:]([^/]+/[^/]+?)(?:\.git)?(?:/.*)?$`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// isDefaultGroup checks if the groups array represents default groups
func isDefaultGroup(groups []string) bool {
	return len(groups) == 1 && groups[0] == defaultGroup
}

// findInsertionPoint finds the best place to insert a new gem
func (w *GemfileWriter) findInsertionPoint(groups []string) int {
	// If no specific groups, add after other default gems
	if isDefaultGroup(groups) {
		// Find the last gem line that's not in a group block
		lastGemIndex := -1
		inGroupBlock := false

		for i, line := range w.content {
			trimmed := strings.TrimSpace(line)

			if strings.HasPrefix(trimmed, "group ") {
				inGroupBlock = true
				continue
			}

			if trimmed == endKeyword {
				inGroupBlock = false
				continue
			}

			if !inGroupBlock && strings.HasPrefix(trimmed, "gem ") {
				lastGemIndex = i
			}
		}

		if lastGemIndex >= 0 {
			return lastGemIndex + 1
		}
	}

	// Default: add at the end of the file
	return len(w.content)
}

// save writes the modified content back to the Gemfile
func (w *GemfileWriter) save() error {
	content := strings.Join(w.content, "\n")
	return os.WriteFile(w.filepath, []byte(content), 0600)
}

// AddGemToFile is a convenience function to add a gem to a Gemfile
func AddGemToFile(filepath string, dep *GemDependency) error {
	writer := NewGemfileWriter(filepath)
	return writer.AddGem(dep)
}

// RemoveGemFromFile is a convenience function to remove a gem from a Gemfile
func RemoveGemFromFile(filepath, gemName string) error {
	writer := NewGemfileWriter(filepath)
	return writer.RemoveGem(gemName)
}

// AddGemspec adds a gemspec directive to the Gemfile
func (w *GemfileWriter) AddGemspec(gemspecRef *GemspecReference) error {
	if err := w.Load(); err != nil {
		return err
	}

	// Check if gemspec already exists
	if w.hasGemspec() {
		return fmt.Errorf("gemspec directive already exists in Gemfile")
	}

	gemspecLine := w.formatGemspecDirective(gemspecRef)

	// Find the best place to insert the gemspec (after source and ruby declarations)
	insertIndex := w.findGemspecInsertionPoint()

	// Insert the gemspec line
	w.content = append(w.content[:insertIndex], append([]string{gemspecLine}, w.content[insertIndex:]...)...)

	return w.save()
}

// hasGemspec checks if a gemspec directive already exists in the Gemfile
func (w *GemfileWriter) hasGemspec() bool {
	for _, line := range w.content {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "gemspec") {
			return true
		}
	}
	return false
}

// formatGemspecDirective formats a gemspec reference into a Gemfile line string
func (w *GemfileWriter) formatGemspecDirective(gemspecRef *GemspecReference) string {
	// If all values are defaults, just return "gemspec"
	if gemspecRef.Path == "." &&
		gemspecRef.Name == "" &&
		gemspecRef.DevelopmentGroup == developmentGroup &&
		gemspecRef.Glob == defaultGlobPattern {
		return "gemspec"
	}

	var parts []string
	parts = append(parts, "gemspec")

	// Add non-default options
	if gemspecRef.Path != "." && gemspecRef.Path != "" {
		parts = append(parts, fmt.Sprintf("path: '%s'", gemspecRef.Path))
	}

	if gemspecRef.Name != "" {
		parts = append(parts, fmt.Sprintf("name: '%s'", gemspecRef.Name))
	}

	if gemspecRef.DevelopmentGroup != developmentGroup && gemspecRef.DevelopmentGroup != "" {
		parts = append(parts, fmt.Sprintf("development_group: :%s", gemspecRef.DevelopmentGroup))
	}

	if gemspecRef.Glob != defaultGlobPattern && gemspecRef.Glob != "" {
		parts = append(parts, fmt.Sprintf("glob: '%s'", gemspecRef.Glob))
	}

	return strings.Join(parts, ", ")
}

// findGemspecInsertionPoint finds the best place to insert a gemspec directive
func (w *GemfileWriter) findGemspecInsertionPoint() int {
	// Gemspec usually goes after source and ruby declarations but before gems
	lastMetaIndex := -1
	firstGemIndex := -1

	for i, line := range w.content {
		trimmed := strings.TrimSpace(line)

		// Track source and ruby lines
		if strings.HasPrefix(trimmed, "source ") || strings.HasPrefix(trimmed, "ruby ") {
			lastMetaIndex = i
		}

		// Track first gem line
		if firstGemIndex == -1 && strings.HasPrefix(trimmed, "gem ") {
			firstGemIndex = i
		}
	}

	// Insert after source/ruby declarations if they exist
	if lastMetaIndex >= 0 {
		// Skip any blank lines after the last meta declaration
		insertPoint := lastMetaIndex + 1
		for insertPoint < len(w.content) && strings.TrimSpace(w.content[insertPoint]) == "" {
			insertPoint++
		}
		return insertPoint
	}

	// Otherwise, insert before the first gem if it exists
	if firstGemIndex >= 0 {
		return firstGemIndex
	}

	// Default: add at the beginning of the file (after any comments)
	for i, line := range w.content {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return i
		}
	}

	return len(w.content)
}

// WriteGemfile writes a complete Gemfile from a ParsedGemfile structure
func WriteGemfile(filepath string, parsed *ParsedGemfile) error {
	var lines []string

	// Add header comment if needed
	lines = append(lines, "# Generated by gemfile-go", "")

	// Add sources
	for _, source := range parsed.Sources {
		if source.Type == rubygemsSource {
			lines = append(lines, fmt.Sprintf("source '%s'", source.URL))
		}
	}

	// Add Ruby version if specified
	if parsed.RubyVersion != "" {
		if len(lines) > 2 { // After header and blank line
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("ruby '%s'", parsed.RubyVersion))
	}

	// Add gemspec directives
	for _, gemspecRef := range parsed.Gemspecs {
		if len(lines) > 2 {
			lines = append(lines, "")
		}
		writer := &GemfileWriter{}
		lines = append(lines, writer.formatGemspecDirective(&gemspecRef))
	}

	// Group dependencies by their groups
	defaultGems, groupedGems := groupDependencies(parsed.Dependencies)

	// Write default gems
	if len(defaultGems) > 0 {
		if len(lines) > 2 {
			lines = append(lines, "")
		}
		writer := &GemfileWriter{}
		for _, dep := range defaultGems {
			lines = append(lines, writer.formatGemLine(&dep))
		}
	}

	// Write grouped gems
	for group, gems := range groupedGems {
		if len(lines) > 2 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("group :%s do", group))
		writer := &GemfileWriter{}
		for _, dep := range gems {
			// Clear groups for formatting since they're in a group block
			tempDep := dep
			tempDep.Groups = []string{defaultGroup}
			lines = append(lines, "  "+writer.formatGemLine(&tempDep))
		}
		lines = append(lines, endKeyword)
	}

	// Write the file
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(filepath, []byte(content), 0600)
}

// groupDependencies separates dependencies into default and grouped gems
func groupDependencies(dependencies []GemDependency) (defaultGems []GemDependency, groupedGems map[string][]GemDependency) {
	defaultGems = []GemDependency{}
	groupedGems = make(map[string][]GemDependency)

	for _, dep := range dependencies {
		if isDefaultGroup(dep.Groups) {
			defaultGems = append(defaultGems, dep)
		} else {
			for _, group := range dep.Groups {
				groupedGems[group] = append(groupedGems[group], dep)
			}
		}
	}

	return defaultGems, groupedGems
}
