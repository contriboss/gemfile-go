package gemfile

import (
	"fmt"
	"os"
	"regexp"
	"strings"
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
func (w *GemfileWriter) AddGem(dep GemDependency) error {
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

// formatGemLine formats a gem dependency into a Gemfile line
func (w *GemfileWriter) formatGemLine(dep GemDependency) string {
	parts := []string{fmt.Sprintf("gem '%s'", dep.Name)}
	
	// Add version constraints
	for _, constraint := range dep.Constraints {
		parts = append(parts, fmt.Sprintf("'%s'", constraint))
	}
	
	// Add source information
	if dep.Source != nil {
		switch dep.Source.Type {
		case "git":
			if strings.Contains(dep.Source.URL, "github.com") {
				// Use github shorthand if possible
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
		case "path":
			parts = append(parts, fmt.Sprintf("path: '%s'", dep.Source.URL))
		case "rubygems":
			if dep.Source.URL != "https://rubygems.org" {
				parts = append(parts, fmt.Sprintf("source: '%s'", dep.Source.URL))
			}
		}
	}
	
	// Add groups (if not default)
	if len(dep.Groups) > 0 && !isDefaultGroup(dep.Groups) {
		if len(dep.Groups) == 1 {
			parts = append(parts, fmt.Sprintf("group: :%s", dep.Groups[0]))
		} else {
			groupsStr := make([]string, len(dep.Groups))
			for i, group := range dep.Groups {
				groupsStr[i] = ":" + group
			}
			parts = append(parts, fmt.Sprintf("groups: [%s]", strings.Join(groupsStr, ", ")))
		}
	}
	
	// Add require option
	if dep.Require != nil {
		if *dep.Require == "" || *dep.Require == "false" {
			parts = append(parts, "require: false")
		} else {
			parts = append(parts, fmt.Sprintf("require: '%s'", *dep.Require))
		}
	}
	
	return strings.Join(parts, ", ")
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
	return len(groups) == 1 && groups[0] == "default"
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
			
			if trimmed == "end" {
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
	return os.WriteFile(w.filepath, []byte(content), 0644)
}

// AddGemToFile is a convenience function to add a gem to a Gemfile
func AddGemToFile(filepath string, dep GemDependency) error {
	writer := NewGemfileWriter(filepath)
	return writer.AddGem(dep)
}

// RemoveGemFromFile is a convenience function to remove a gem from a Gemfile
func RemoveGemFromFile(filepath, gemName string) error {
	writer := NewGemfileWriter(filepath)
	return writer.RemoveGem(gemName)
}