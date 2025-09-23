// Package lockfile provides a pure Go parser for Ruby's Gemfile.lock format.
//
// This package is designed for Ruby developers learning Go or Go developers
// working with Ruby projects. It parses the Bundler lockfile format to extract
// dependency information without requiring Ruby to be installed.
//
// Fun fact: Did you know that Gemfile.lock uses a custom format that's almost
// YAML but not quite? It's like YAML's quirky cousin who shows up at family
// gatherings with their own unique style! 🎭
//
// Example usage:
//   lockfile, err := ParseFile("Gemfile.lock")
//   if err != nil {
//       log.Fatal(err)
//   }
//   fmt.Printf("Found %d gems\n", len(lockfile.GemSpecs))
package lockfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Lockfile represents a parsed Gemfile.lock file.
// In Ruby world, this is your project's "frozen" dependency snapshot.
// Think of it as a time capsule of exact gem versions that worked together! 💎
type Lockfile struct {
	// GemSpecs contains all gems from the GEM section
	// These are your regular gems from rubygems.org
	GemSpecs     []GemSpec

	// GitSpecs contains gems installed from git repositories
	// For when you're living on the edge with unreleased features!
	GitSpecs     []GitGemSpec

	// PathSpecs contains gems installed from local paths
	// Perfect for developing multiple gems simultaneously
	PathSpecs    []PathGemSpec

	// Platforms lists all platforms this lockfile supports
	// e.g., "ruby", "x86_64-linux", "arm64-darwin"
	Platforms    []string

	// Dependencies are the top-level gems your app directly depends on
	// These come from your Gemfile
	Dependencies []Dependency

	// BundledWith shows which Bundler version created this lockfile
	// Important for compatibility!
	BundledWith  string

	// Groups maps group names to gem names (e.g., "development" -> ["rspec", "pry"])
	Groups       map[string][]string
}

// FindGem searches for a gem by name in the lockfile.
// Returns nil if not found. This is like `bundle show gemname`.
func (l *Lockfile) FindGem(name string) *GemSpec {
	for i := range l.GemSpecs {
		if l.GemSpecs[i].Name == name {
			return &l.GemSpecs[i]
		}
	}
	return nil
}

// GemSpec represents a single gem in the lockfile.
// For Ruby devs: This is like `gem.specification` but flattened and simplified.
// For Go devs: This is similar to a go.mod module entry with extra metadata.
type GemSpec struct {
	// Name is the gem name (e.g., "rails", "puma", "sidekiq")
	Name         string

	// Version is the exact version locked (e.g., "7.0.4")
	// In Ruby: Gem::Version.new("7.0.4")
	Version      string

	// Platform specifies architecture if not "ruby" (e.g., "java", "x86_64-linux")
	// Empty string means it works on all platforms (pure Ruby)
	Platform     string

	// Dependencies this gem needs to work (runtime dependencies)
	Dependencies []Dependency

	// Groups this gem belongs to (e.g., ["default", "development"])
	Groups       []string

	// Checksum is the SHA256 hash for security verification
	// Prevents sneaky gem swapping! 🔒
	Checksum     string
	// Security and metadata
	SourceURL        string            `json:"source_url,omitempty"`
	PostInstallMessage string          `json:"post_install_message,omitempty"`
	Extensions       []string          `json:"extensions,omitempty"`
	RequiredRubyVersion string         `json:"required_ruby_version,omitempty"`
	RequiredRubygemsVersion string     `json:"required_rubygems_version,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	// Installation tracking
	InstallationState string `json:"installation_state,omitempty"`
	InstallationError string `json:"installation_error,omitempty"`
}

type GitGemSpec struct {
	Name         string
	Version      string
	Remote       string
	Revision     string
	Branch       string
	Tag          string
	Dependencies []Dependency
	Groups       []string
	// Additional metadata for Git gems
	PostInstallMessage string            `json:"post_install_message,omitempty"`
	Extensions         []string          `json:"extensions,omitempty"`
	RequiredRubyVersion string           `json:"required_ruby_version,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	// Git-specific metadata
	CommitMessage      string `json:"commit_message,omitempty"`
	AuthorEmail        string `json:"author_email,omitempty"`
	CommitDate         string `json:"commit_date,omitempty"`
}

type PathGemSpec struct {
	Name         string
	Version      string
	Remote       string // local path
	Dependencies []Dependency
	Groups       []string
	// Additional metadata for PATH gems
	PostInstallMessage  string            `json:"post_install_message,omitempty"`
	Extensions          []string          `json:"extensions,omitempty"`
	RequiredRubyVersion string            `json:"required_ruby_version,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	// Path-specific metadata
	AbsolutePath        string `json:"absolute_path,omitempty"`
	LastModified        string `json:"last_modified,omitempty"`
	DevelopmentGem      bool   `json:"development_gem,omitempty"`
}

type Dependency struct {
	Name        string
	Constraints []string
	// Additional dependency metadata
	Type        string `json:"type,omitempty"`        // "runtime", "development", "test"
	Scope       string `json:"scope,omitempty"`       // "direct", "transitive"
	Optional    bool   `json:"optional,omitempty"`    // Whether dependency is optional
	Platform    string `json:"platform,omitempty"`   // Platform restriction
	Environment string `json:"environment,omitempty"` // Environment restriction
}

var (
	gemSpecRegex = regexp.MustCompile(`^    ([a-zA-Z0-9\-_]+) \(([^)]+)\)$`)
	depRegex     = regexp.MustCompile(`^      ([a-zA-Z0-9\-_]+)(?: \(([^)]+)\))?$`)
)

// ParseFile is a convenience function to parse a Gemfile.lock from a file path.
// This is what most Ruby developers will use:
//   lock, err := lockfile.ParseFile("Gemfile.lock")
func ParseFile(path string) (*Lockfile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open lockfile: %w", err)
	}
	defer file.Close()

	return Parse(file)
}

// Parse reads and parses a Gemfile.lock from an io.Reader.
// For advanced users who want to parse from memory or network streams.
func Parse(reader io.Reader) (*Lockfile, error) {
	lockfile := &Lockfile{
		Groups: make(map[string][]string),
	}
	scanner := bufio.NewScanner(reader)

	var currentSection string
	var currentGem *GemSpec
	var currentGitGem *GitGemSpec
	var currentPathGem *PathGemSpec

	for scanner.Scan() {
		line := scanner.Text()

		// Check for section headers
		switch line {
		case "GEM":
			// Save any pending Git gem before switching sections
			if currentGitGem != nil {
				lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
				currentGitGem = nil
			}
			if currentPathGem != nil {
				lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
				currentPathGem = nil
			}
			currentSection = "GEM"
			continue
		case "GIT":
			// Save any pending gems before starting new GIT section
			if currentGem != nil {
				lockfile.GemSpecs = append(lockfile.GemSpecs, *currentGem)
				currentGem = nil
			}
			if currentGitGem != nil {
				lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
				currentGitGem = nil
			}
			if currentPathGem != nil {
				lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
				currentPathGem = nil
			}
			currentSection = "GIT"
			continue
		case "PATH":
			// Save any pending gems before starting new PATH section
			if currentGem != nil {
				lockfile.GemSpecs = append(lockfile.GemSpecs, *currentGem)
				currentGem = nil
			}
			if currentGitGem != nil {
				lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
				currentGitGem = nil
			}
			if currentPathGem != nil {
				lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
				currentPathGem = nil
			}
			currentSection = "PATH"
			continue
		case "PLATFORMS":
			// Save any pending gems before switching sections
			if currentGem != nil {
				lockfile.GemSpecs = append(lockfile.GemSpecs, *currentGem)
				currentGem = nil
			}
			if currentGitGem != nil {
				lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
				currentGitGem = nil
			}
			if currentPathGem != nil {
				lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
				currentPathGem = nil
			}
			currentSection = "PLATFORMS"
			continue
		case "DEPENDENCIES":
			// Save any pending gems before switching sections
			if currentGem != nil {
				lockfile.GemSpecs = append(lockfile.GemSpecs, *currentGem)
				currentGem = nil
			}
			if currentGitGem != nil {
				lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
				currentGitGem = nil
			}
			if currentPathGem != nil {
				lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
				currentPathGem = nil
			}
			currentSection = "DEPENDENCIES"
			continue
		}

		if strings.HasPrefix(line, "BUNDLED WITH") {
			currentSection = "BUNDLED_WITH"
			continue
		}

		if strings.HasPrefix(line, "  remote:") {
			if currentSection == "GIT" {
				// Extract Git remote URL
				remote := strings.TrimSpace(strings.TrimPrefix(line, "  remote:"))
				if currentGitGem == nil {
					currentGitGem = &GitGemSpec{}
				}
				currentGitGem.Remote = remote
			} else if currentSection == "PATH" {
				// Extract PATH remote (local directory)
				remote := strings.TrimSpace(strings.TrimPrefix(line, "  remote:"))
				if currentPathGem == nil {
					currentPathGem = &PathGemSpec{}
				}
				currentPathGem.Remote = remote
			}
			// For GEM section, skip remote URLs as before
			continue
		}

		if strings.HasPrefix(line, "  revision:") && currentSection == "GIT" {
			revision := strings.TrimSpace(strings.TrimPrefix(line, "  revision:"))
			if currentGitGem == nil {
				currentGitGem = &GitGemSpec{}
			}
			currentGitGem.Revision = revision
			continue
		}

		if strings.HasPrefix(line, "  branch:") && currentSection == "GIT" {
			branch := strings.TrimSpace(strings.TrimPrefix(line, "  branch:"))
			if currentGitGem == nil {
				currentGitGem = &GitGemSpec{}
			}
			currentGitGem.Branch = branch
			continue
		}

		if strings.HasPrefix(line, "  tag:") && currentSection == "GIT" {
			tag := strings.TrimSpace(strings.TrimPrefix(line, "  tag:"))
			if currentGitGem == nil {
				currentGitGem = &GitGemSpec{}
			}
			currentGitGem.Tag = tag
			continue
		}

		if strings.HasPrefix(line, "  specs:") {
			// Skip specs header
			continue
		}

		// Process lines based on current section
		switch currentSection {
		case "GEM":
			if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
				// Save previous gem
				if currentGem != nil {
					lockfile.GemSpecs = append(lockfile.GemSpecs, *currentGem)
				}

				// Parse version and platform from version string
				versionAndPlatform := matches[2]
				version := versionAndPlatform
				platform := ""

				// Check if version contains platform info (e.g., "1.13.8-x86_64-darwin")
				parts := strings.Split(versionAndPlatform, "-")
				if len(parts) >= 3 && (strings.Contains(versionAndPlatform, "x86") || strings.Contains(versionAndPlatform, "darwin") || strings.Contains(versionAndPlatform, "linux") || strings.Contains(versionAndPlatform, "java")) {
					// Assume version is the first part, platform is the rest
					version = parts[0]
					platform = strings.Join(parts[1:], "-")
				}

				// Start new gem
				currentGem = &GemSpec{
					Name:     matches[1],
					Version:  version,
					Platform: platform,
				}
			} else if matches := depRegex.FindStringSubmatch(line); matches != nil && currentGem != nil {
				// Add dependency to current gem
				dep := Dependency{
					Name: matches[1],
				}
				if len(matches) > 2 && matches[2] != "" {
					dep.Constraints = parseConstraints(matches[2])
				}
				currentGem.Dependencies = append(currentGem.Dependencies, dep)
			}

		case "GIT":
			if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
				// This is a gem spec inside a Git section
				if currentGitGem == nil {
					currentGitGem = &GitGemSpec{}
				}

				// Parse version from the gem spec line
				versionAndPlatform := matches[2]
				version := versionAndPlatform

				// For Git gems, usually no platform info in version
				currentGitGem.Name = matches[1]
				currentGitGem.Version = version
			} else if matches := depRegex.FindStringSubmatch(line); matches != nil && currentGitGem != nil {
				// Add dependency to current Git gem
				dep := Dependency{
					Name: matches[1],
				}
				if len(matches) > 2 && matches[2] != "" {
					dep.Constraints = parseConstraints(matches[2])
				}
				currentGitGem.Dependencies = append(currentGitGem.Dependencies, dep)
			}

		case "PATH":
			if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
				// This is a gem spec inside a PATH section
				if currentPathGem == nil {
					currentPathGem = &PathGemSpec{}
				}

				// Parse version from the gem spec line
				versionAndPlatform := matches[2]
				version := versionAndPlatform

				// For PATH gems, usually no platform info in version
				currentPathGem.Name = matches[1]
				currentPathGem.Version = version
			} else if matches := depRegex.FindStringSubmatch(line); matches != nil && currentPathGem != nil {
				// Add dependency to current PATH gem
				dep := Dependency{
					Name: matches[1],
				}
				if len(matches) > 2 && matches[2] != "" {
					dep.Constraints = parseConstraints(matches[2])
				}
				currentPathGem.Dependencies = append(currentPathGem.Dependencies, dep)
			}

		case "PLATFORMS":
			if strings.HasPrefix(line, "  ") {
				platform := strings.TrimSpace(line)
				lockfile.Platforms = append(lockfile.Platforms, platform)
			}

		case "DEPENDENCIES":
			if strings.HasPrefix(line, "  ") {
				depLine := strings.TrimSpace(line)
				if matches := regexp.MustCompile(`^([a-zA-Z0-9\-_]+) \(([^)]+)\)$`).FindStringSubmatch(depLine); matches != nil {
					dep := Dependency{
						Name:        matches[1],
						Constraints: parseConstraints(matches[2]),
					}
					lockfile.Dependencies = append(lockfile.Dependencies, dep)
				} else if parts := strings.Fields(depLine); len(parts) > 0 {
					dep := Dependency{
						Name: parts[0],
					}
					lockfile.Dependencies = append(lockfile.Dependencies, dep)
				}
			}

		case "BUNDLED_WITH":
			if strings.HasPrefix(line, "   ") {
				lockfile.BundledWith = strings.TrimSpace(line)
			}
		}
	}

	// Add the last gem if exists
	if currentGem != nil {
		lockfile.GemSpecs = append(lockfile.GemSpecs, *currentGem)
	}

	// Add the last Git gem if exists
	if currentGitGem != nil {
		lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
	}

	// Add the last PATH gem if exists
	if currentPathGem != nil {
		lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("❌ Error reading lockfile\n   💡 File may be corrupted - try regenerating with 'bundle lock'")
	}

	return lockfile, nil
}

func parseConstraints(constraintStr string) []string {
	constraints := strings.Split(constraintStr, ",")
	result := make([]string, 0, len(constraints))

	for _, constraint := range constraints {
		constraint = strings.TrimSpace(constraint)
		if constraint != "" {
			result = append(result, constraint)
		}
	}

	return result
}

func (gs *GemSpec) FullName() string {
	if gs.Platform != "" {
		return fmt.Sprintf("%s-%s-%s", gs.Name, gs.Version, gs.Platform)
	}
	return fmt.Sprintf("%s-%s", gs.Name, gs.Version)
}

func (gs *GemSpec) SemVer() (*semver.Version, error) {
	return semver.NewVersion(gs.Version)
}

func (gits *GitGemSpec) FullName() string {
	return fmt.Sprintf("%s-%s", gits.Name, gits.Version)
}

func (gits *GitGemSpec) SemVer() (*semver.Version, error) {
	return semver.NewVersion(gits.Version)
}

func (paths *PathGemSpec) FullName() string {
	return fmt.Sprintf("%s-%s", paths.Name, paths.Version)
}

func (paths *PathGemSpec) SemVer() (*semver.Version, error) {
	return semver.NewVersion(paths.Version)
}

// FilterGemsByGroups filters gems based on included/excluded groups
func FilterGemsByGroups(gems []GemSpec, includeGroups, excludeGroups []string) []GemSpec {
	if len(includeGroups) == 0 && len(excludeGroups) == 0 {
		return gems // No filtering needed
	}

	var filtered []GemSpec
	for _, gem := range gems {
		// Default to production group if no groups specified
		gemGroups := gem.Groups
		if len(gemGroups) == 0 {
			gemGroups = []string{"default"}
		}

		// Check exclusions first
		excluded := false
		for _, excludeGroup := range excludeGroups {
			for _, gemGroup := range gemGroups {
				if gemGroup == excludeGroup {
					excluded = true
					break
				}
			}
			if excluded {
				break
			}
		}

		if excluded {
			continue
		}

		// Check inclusions
		if len(includeGroups) > 0 {
			included := false
			for _, includeGroup := range includeGroups {
				for _, gemGroup := range gemGroups {
					if gemGroup == includeGroup || gemGroup == "default" {
						included = true
						break
					}
				}
				if included {
					break
				}
			}
			if !included {
				continue
			}
		}

		filtered = append(filtered, gem)
	}

	return filtered
}
