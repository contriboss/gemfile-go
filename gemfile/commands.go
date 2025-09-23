package gemfile

import (
	"fmt"
	"os"
	"strings"
)

// AddOptions represents options for the add command
type AddOptions struct {
	Name         string
	Version      string
	Groups       []string
	Source       string
	Git          string
	Github       string
	Branch       string
	Tag          string
	Ref          string
	Path         string
	Require      *string
	SkipInstall  bool
	Strict       bool
	Optimistic   bool
}

// RemoveOptions represents options for the remove command
type RemoveOptions struct {
	GemNames []string
	Install  bool
}

// AddGemCommand handles the ore add command
func AddGemCommand(gemfilePath string, opts AddOptions) error {
	// Validate gem name
	if opts.Name == "" {
		return fmt.Errorf("gem name is required")
	}
	
	// Find Gemfile
	if gemfilePath == "" {
		gemfilePath = findGemfile()
	}
	
	if _, err := os.Stat(gemfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Gemfile not found. Use 'ore init' to create one")
	}
	
	// Build dependency
	dep := GemDependency{
		Name:    opts.Name,
		Groups:  opts.Groups,
		Require: opts.Require,
	}
	
	// Handle version constraints
	if opts.Version != "" {
		if opts.Strict {
			dep.Constraints = []string{"= " + opts.Version}
		} else if opts.Optimistic {
			dep.Constraints = []string{">= " + opts.Version}
		} else {
			dep.Constraints = []string{opts.Version}
		}
	}
	
	// Handle source options
	if opts.Git != "" {
		dep.Source = &Source{
			Type:   "git",
			URL:    opts.Git,
			Branch: opts.Branch,
			Tag:    opts.Tag,
			Ref:    opts.Ref,
		}
	} else if opts.Github != "" {
		dep.Source = &Source{
			Type:   "git",
			URL:    fmt.Sprintf("https://github.com/%s.git", opts.Github),
			Branch: opts.Branch,
			Tag:    opts.Tag,
			Ref:    opts.Ref,
		}
	} else if opts.Path != "" {
		dep.Source = &Source{
			Type: "path",
			URL:  opts.Path,
		}
	} else if opts.Source != "" {
		dep.Source = &Source{
			Type: "rubygems",
			URL:  opts.Source,
		}
	}
	
	// Set default groups if none specified
	if len(dep.Groups) == 0 {
		dep.Groups = []string{"default"}
	}
	
	// Add gem to Gemfile
	if err := AddGemToFile(gemfilePath, dep); err != nil {
		return fmt.Errorf("failed to add gem to Gemfile: %w", err)
	}
	
	return nil
}

// RemoveGemCommand handles the ore remove command
func RemoveGemCommand(gemfilePath string, opts RemoveOptions) error {
	// Validate gem names
	if len(opts.GemNames) == 0 {
		return fmt.Errorf("at least one gem name is required")
	}
	
	// Find Gemfile
	if gemfilePath == "" {
		gemfilePath = findGemfile()
	}
	
	if _, err := os.Stat(gemfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Gemfile not found")
	}
	
	// Remove each gem
	for _, gemName := range opts.GemNames {
		if err := RemoveGemFromFile(gemfilePath, gemName); err != nil {
			return fmt.Errorf("failed to remove gem %q: %w", gemName, err)
		}
	}
	
	return nil
}

// findGemfile finds the Gemfile in the current directory
func findGemfile() string {
	candidates := []string{"Gemfile", "gems.rb"}
	
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	
	return "Gemfile" // default
}

// ParseGroups parses a comma-separated group string
func ParseGroups(groupStr string) []string {
	if groupStr == "" {
		return []string{"default"}
	}
	
	groups := strings.Split(groupStr, ",")
	for i, group := range groups {
		groups[i] = strings.TrimSpace(group)
	}
	
	return groups
}

// ParseRequire parses require option
func ParseRequire(requireStr string) *string {
	if requireStr == "" {
		return nil
	}
	
	if requireStr == "false" {
		empty := ""
		return &empty
	}
	
	return &requireStr
}