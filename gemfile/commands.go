package gemfile

import (
	"fmt"
	"os"
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
)

const (
	defaultGemfileName = "Gemfile"
)

// AddOptions represents options for the add command
type AddOptions struct {
	Name        string
	Version     string
	Groups      []string
	Source      string
	Git         string
	Github      string
	Branch      string
	Tag         string
	Ref         string
	Path        string
	Require     *string
	SkipInstall bool
	Strict      bool
	Optimistic  bool
}

// RemoveOptions represents options for the remove command
type RemoveOptions struct {
	GemNames []string
	Install  bool
}

// AddGemCommand handles the ore add command
func AddGemCommand(gemfilePath string, opts *AddOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("gem name is required")
	}

	resolvedGemfilePath, err := resolveGemfilePath(gemfilePath)
	if err != nil {
		return err
	}

	if err := ensureLockfile(resolvedGemfilePath); err != nil {
		return err
	}

	dep := buildDependency(opts)

	if err := AddGemToFile(resolvedGemfilePath, &dep); err != nil {
		return fmt.Errorf("failed to add gem to Gemfile: %w", err)
	}

	return nil
}

func resolveGemfilePath(gemfilePath string) (string, error) {
	if gemfilePath == "" {
		gemfilePath = findGemfile()
	}

	if _, err := os.Stat(gemfilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("gemfile not found, use 'ore init' to create one")
	}

	return gemfilePath, nil
}

func buildDependency(opts *AddOptions) GemDependency {
	dep := GemDependency{
		Name:    opts.Name,
		Groups:  opts.Groups,
		Require: opts.Require,
	}

	applyVersionConstraints(opts, &dep)
	applySourceOptions(opts, &dep)

	if len(dep.Groups) == 0 {
		dep.Groups = []string{"default"}
	}

	return dep
}

func applyVersionConstraints(opts *AddOptions, dep *GemDependency) {
	if opts.Version == "" {
		return
	}

	if opts.Strict {
		dep.Constraints = []string{"= " + opts.Version}
		return
	}

	if opts.Optimistic {
		dep.Constraints = []string{">= " + opts.Version}
		return
	}

	dep.Constraints = []string{opts.Version}
}

func applySourceOptions(opts *AddOptions, dep *GemDependency) {
	switch {
	case opts.Git != "":
		dep.Source = &Source{
			Type:   "git",
			URL:    opts.Git,
			Branch: opts.Branch,
			Tag:    opts.Tag,
			Ref:    opts.Ref,
		}
	case opts.Github != "":
		dep.Source = &Source{
			Type:   "git",
			URL:    fmt.Sprintf("https://github.com/%s.git", opts.Github),
			Branch: opts.Branch,
			Tag:    opts.Tag,
			Ref:    opts.Ref,
		}
	case opts.Path != "":
		dep.Source = &Source{
			Type: "path",
			URL:  opts.Path,
		}
	case opts.Source != "":
		dep.Source = &Source{
			Type: "rubygems",
			URL:  opts.Source,
		}
	}
}

func ensureLockfile(gemfilePath string) error {
	lockfilePath := lockfile.DetermineLockfilePath(gemfilePath)
	if _, err := lockfile.ParseFile(lockfilePath); err != nil {
		return err
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
		return fmt.Errorf("gemfile not found")
	}

	if err := ensureLockfile(gemfilePath); err != nil {
		return err
	}

	// Remove each gem
	for _, gemName := range opts.GemNames {
		if err := RemoveGemFromFile(gemfilePath, gemName); err != nil {
			return fmt.Errorf("failed to remove gem %q: %w", gemName, err)
		}
	}

	if opts.Install {
		return fmt.Errorf(
			"bundle install is not supported by gemfile-go: this library only edits an existing " +
				"Gemfile and lockfile. Ensure your lockfile exists and is valid, then run " +
				"`bundle install` separately with Bundler",
		)
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

	return defaultGemfileName // default
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

	if requireStr == falseValue {
		empty := ""
		return &empty
	}

	return &requireStr
}
