// Package lockfile provides a pure Go parser for Ruby's Gemfile.lock format.
// It parses the Bundler lockfile format without requiring Ruby.
//
// Ruby equivalent: Bundler.locked_gems
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
type Lockfile struct {
	GemSpecs     []GemSpec           // Gems from the GEM section
	GitSpecs     []GitGemSpec        // Gems from git repositories
	PathSpecs    []PathGemSpec       // Gems from local paths
	Platforms    []string            // Supported platforms (e.g., "ruby", "x86_64-linux")
	Dependencies []Dependency        // Top-level dependencies from Gemfile
	BundledWith  string              // Bundler version used
	Groups       map[string][]string // Group name to gem names mapping
}

// FindGem searches for a gem by name in the lockfile.
// Ruby equivalent: Bundler.locked_gems.specs.find {|s| s.name == name}
func (l *Lockfile) FindGem(name string) *GemSpec {
	for i := range l.GemSpecs {
		if l.GemSpecs[i].Name == name {
			return &l.GemSpecs[i]
		}
	}
	return nil
}

// GemSpec represents a single gem in the lockfile.
// Ruby equivalent: Bundler::LazySpecification
type GemSpec struct {
	Name         string       // Gem name
	Version      string       // Exact version locked
	Platform     string       // Platform restriction (empty for pure Ruby)
	Dependencies []Dependency // Runtime dependencies
	Groups       []string     // Groups this gem belongs to
	Checksum     string       // SHA256 for integrity verification
	// Security and metadata
	SourceURL               string            `json:"source_url,omitempty"`
	PostInstallMessage      string            `json:"post_install_message,omitempty"`
	Extensions              []string          `json:"extensions,omitempty"`
	RequiredRubyVersion     string            `json:"required_ruby_version,omitempty"`
	RequiredRubygemsVersion string            `json:"required_rubygems_version,omitempty"`
	Metadata                map[string]string `json:"metadata,omitempty"`
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
	PostInstallMessage  string            `json:"post_install_message,omitempty"`
	Extensions          []string          `json:"extensions,omitempty"`
	RequiredRubyVersion string            `json:"required_ruby_version,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	// Git-specific metadata
	CommitMessage string `json:"commit_message,omitempty"`
	AuthorEmail   string `json:"author_email,omitempty"`
	CommitDate    string `json:"commit_date,omitempty"`
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
	AbsolutePath   string `json:"absolute_path,omitempty"`
	LastModified   string `json:"last_modified,omitempty"`
	DevelopmentGem bool   `json:"development_gem,omitempty"`
}

type Dependency struct {
	Name        string
	Constraints []string
	// Additional dependency metadata
	Type        string `json:"type,omitempty"`        // "runtime", "development", "test"
	Scope       string `json:"scope,omitempty"`       // "direct", "transitive"
	Optional    bool   `json:"optional,omitempty"`    // Whether dependency is optional
	Platform    string `json:"platform,omitempty"`    // Platform restriction
	Environment string `json:"environment,omitempty"` // Environment restriction
}

const (
	sectionGEM          = "GEM"
	sectionGIT          = "GIT"
	sectionPATH         = "PATH"
	sectionPLATFORMS    = "PLATFORMS"
	sectionDEPENDENCIES = "DEPENDENCIES"
	sectionBUNDLED_WITH = "BUNDLED_WITH"
)

var (
	gemSpecRegex = regexp.MustCompile(`^    ([a-zA-Z0-9\-_]+) \(([^)]+)\)$`)
	depRegex     = regexp.MustCompile(`^      ([a-zA-Z0-9\-_]+)(?: \(([^)]+)\))?$`)
)

// ParseFile parses a Gemfile.lock from a file path.
func ParseFile(path string) (*Lockfile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open lockfile: %w", err)
	}
	defer file.Close()

	return Parse(file)
}

// Parse reads and parses a Gemfile.lock from an io.Reader.
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
		case sectionGEM:
			// Save any pending Git gem before switching sections
			if currentGitGem != nil {
				lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
				currentGitGem = nil
			}
			if currentPathGem != nil {
				lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
				currentPathGem = nil
			}
			currentSection = sectionGEM
			continue
		case sectionGIT:
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
			currentSection = sectionGIT
			continue
		case sectionPATH:
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
			currentSection = sectionPATH
			continue
		case sectionPLATFORMS:
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
			currentSection = sectionPLATFORMS
			continue
		case sectionDEPENDENCIES:
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
			currentSection = sectionDEPENDENCIES
			continue
		}

		if strings.HasPrefix(line, "BUNDLED WITH") {
			currentSection = sectionBUNDLED_WITH
			continue
		}

		if strings.HasPrefix(line, "  remote:") {
			if currentSection == sectionGIT {
				// Extract Git remote URL
				remote := strings.TrimSpace(strings.TrimPrefix(line, "  remote:"))
				if currentGitGem == nil {
					currentGitGem = &GitGemSpec{}
				}
				currentGitGem.Remote = remote
			} else if currentSection == sectionPATH {
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

		if strings.HasPrefix(line, "  revision:") && currentSection == sectionGIT {
			revision := strings.TrimSpace(strings.TrimPrefix(line, "  revision:"))
			if currentGitGem == nil {
				currentGitGem = &GitGemSpec{}
			}
			currentGitGem.Revision = revision
			continue
		}

		if strings.HasPrefix(line, "  branch:") && currentSection == sectionGIT {
			branch := strings.TrimSpace(strings.TrimPrefix(line, "  branch:"))
			if currentGitGem == nil {
				currentGitGem = &GitGemSpec{}
			}
			currentGitGem.Branch = branch
			continue
		}

		if strings.HasPrefix(line, "  tag:") && currentSection == sectionGIT {
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
		case sectionGEM:
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

		case sectionGIT:
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

		case sectionPATH:
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

		case sectionPLATFORMS:
			if strings.HasPrefix(line, "  ") {
				platform := strings.TrimSpace(line)
				lockfile.Platforms = append(lockfile.Platforms, platform)
			}

		case sectionDEPENDENCIES:
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
