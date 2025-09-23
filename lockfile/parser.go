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

var (
	gemSpecRegex = regexp.MustCompile(`^ {4}([a-zA-Z0-9\-_]+) \(([^)]+)\)$`)
	depRegex     = regexp.MustCompile(`^ {6}([a-zA-Z0-9\-_]+)(?: \(([^)]+)\))?$`)
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
// savePendingGems saves any pending gems to the lockfile
func savePendingGems(lockfile *Lockfile, currentGem **GemSpec, currentGitGem **GitGemSpec, currentPathGem **PathGemSpec) {
	if *currentGem != nil {
		lockfile.GemSpecs = append(lockfile.GemSpecs, **currentGem)
		*currentGem = nil
	}
	if *currentGitGem != nil {
		lockfile.GitSpecs = append(lockfile.GitSpecs, **currentGitGem)
		*currentGitGem = nil
	}
	if *currentPathGem != nil {
		lockfile.PathSpecs = append(lockfile.PathSpecs, **currentPathGem)
		*currentPathGem = nil
	}
}

// parseVersionAndPlatform separates version and platform from version string
func parseVersionAndPlatform(versionAndPlatform string) (version, platform string) {
	version = versionAndPlatform
	platform = ""

	// Check if version contains platform info (e.g., "1.13.8-x86_64-darwin")
	parts := strings.Split(versionAndPlatform, "-")
	hasPlatformInfo := strings.Contains(versionAndPlatform, "x86") || strings.Contains(versionAndPlatform, "darwin") ||
		strings.Contains(versionAndPlatform, "linux") || strings.Contains(versionAndPlatform, "java")
	if len(parts) >= 3 && hasPlatformInfo {
		// Assume version is the first part, platform is the rest
		version = parts[0]
		platform = strings.Join(parts[1:], "-")
	}
	return version, platform
}

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
		if newSection := detectSection(line); newSection != "" {
			savePendingGems(lockfile, &currentGem, &currentGitGem, &currentPathGem)
			currentSection = newSection
			continue
		}

		// Skip specs header
		if strings.HasPrefix(line, "  specs:") {
			continue
		}

		// Process line based on current section
		switch currentSection {
		case GemSection:
			processGemSection(line, &currentGem, lockfile)
		case GitSection:
			if processGitMetadata(line, &currentGitGem) {
				continue
			}
			parseGemSpecForGit(line, &currentGitGem)
			parseDependencyForGit(line, currentGitGem)
		case PathSection:
			if processPathMetadata(line, &currentPathGem) {
				continue
			}
			parseGemSpecForPath(line, &currentPathGem)
			parseDependencyForPath(line, currentPathGem)
		case PlatformsSection:
			processPlatformsSection(line, lockfile)
		case DependenciesSection:
			processDependenciesSection(line, lockfile)
		case BundledWithSection:
			processBundledWithSection(line, lockfile)
		}
	}

	// Save any remaining pending gems
	savePendingGems(lockfile, &currentGem, &currentGitGem, &currentPathGem)

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("❌ Error reading lockfile\n   💡 File may be corrupted - try regenerating with 'bundle lock'")
	}

	return lockfile, nil
}

// detectSection checks if the line is a section header and returns the section name.
func detectSection(line string) string {
	switch line {
	case GemSection, GitSection, PathSection, PlatformsSection, DependenciesSection:
		return line
	}
	if strings.HasPrefix(line, "BUNDLED WITH") {
		return BundledWithSection
	}
	return ""
}

// processGemSection handles lines in the GEM section.
func processGemSection(line string, currentGem **GemSpec, lockfile *Lockfile) {
	if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
		// Save previous gem
		if *currentGem != nil {
			lockfile.GemSpecs = append(lockfile.GemSpecs, **currentGem)
		}
		// Parse and create new gem
		version, platform := parseVersionAndPlatform(matches[2])
		*currentGem = &GemSpec{
			Name:     matches[1],
			Version:  version,
			Platform: platform,
		}
	} else if matches := depRegex.FindStringSubmatch(line); matches != nil && *currentGem != nil {
		dep := Dependency{Name: matches[1]}
		if len(matches) > 2 && matches[2] != "" {
			dep.Constraints = parseConstraints(matches[2])
		}
		(*currentGem).Dependencies = append((*currentGem).Dependencies, dep)
	}
}

// processGitMetadata handles metadata lines in the GIT section (remote, revision, branch, tag).
func processGitMetadata(line string, currentGitGem **GitGemSpec) bool {
	if strings.HasPrefix(line, "  remote:") {
		remote := strings.TrimSpace(strings.TrimPrefix(line, "  remote:"))
		if *currentGitGem == nil {
			*currentGitGem = &GitGemSpec{}
		}
		(*currentGitGem).Remote = remote
		return true
	}
	if strings.HasPrefix(line, "  revision:") {
		revision := strings.TrimSpace(strings.TrimPrefix(line, "  revision:"))
		if *currentGitGem == nil {
			*currentGitGem = &GitGemSpec{}
		}
		(*currentGitGem).Revision = revision
		return true
	}
	if strings.HasPrefix(line, "  branch:") {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "  branch:"))
		if *currentGitGem == nil {
			*currentGitGem = &GitGemSpec{}
		}
		(*currentGitGem).Branch = branch
		return true
	}
	if strings.HasPrefix(line, "  tag:") {
		tag := strings.TrimSpace(strings.TrimPrefix(line, "  tag:"))
		if *currentGitGem == nil {
			*currentGitGem = &GitGemSpec{}
		}
		(*currentGitGem).Tag = tag
		return true
	}
	return false
}

// processPathMetadata handles metadata lines in the PATH section (remote).
func processPathMetadata(line string, currentPathGem **PathGemSpec) bool {
	if strings.HasPrefix(line, "  remote:") {
		remote := strings.TrimSpace(strings.TrimPrefix(line, "  remote:"))
		if *currentPathGem == nil {
			*currentPathGem = &PathGemSpec{}
		}
		(*currentPathGem).Remote = remote
		return true
	}
	return false
}

// processPlatformsSection handles lines in the PLATFORMS section.
func processPlatformsSection(line string, lockfile *Lockfile) {
	if strings.HasPrefix(line, "  ") {
		platform := strings.TrimSpace(line)
		lockfile.Platforms = append(lockfile.Platforms, platform)
	}
}

// processDependenciesSection handles lines in the DEPENDENCIES section.
func processDependenciesSection(line string, lockfile *Lockfile) {
	if strings.HasPrefix(line, "  ") {
		depLine := strings.TrimSpace(line)
		if matches := regexp.MustCompile(`^([a-zA-Z0-9\-_]+) \(([^)]+)\)$`).FindStringSubmatch(depLine); matches != nil {
			dep := Dependency{
				Name:        matches[1],
				Constraints: parseConstraints(matches[2]),
			}
			lockfile.Dependencies = append(lockfile.Dependencies, dep)
		} else if parts := strings.Fields(depLine); len(parts) > 0 {
			dep := Dependency{Name: parts[0]}
			lockfile.Dependencies = append(lockfile.Dependencies, dep)
		}
	}
}

// processBundledWithSection handles lines in the BUNDLED_WITH section.
func processBundledWithSection(line string, lockfile *Lockfile) {
	if strings.HasPrefix(line, "   ") {
		lockfile.BundledWith = strings.TrimSpace(line)
	}
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
	for i := range gems {
		gem := &gems[i]
		// Default to production group if no groups specified
		gemGroups := gem.Groups
		if len(gemGroups) == 0 {
			gemGroups = []string{"default"}
		}

		// Check exclusions first
		if isGemExcluded(gemGroups, excludeGroups) {
			continue
		}

		// Check inclusions
		if len(includeGroups) > 0 && !isGemIncluded(gemGroups, includeGroups) {
			continue
		}

		filtered = append(filtered, *gem)
	}

	return filtered
}

// parseGemSpecForGit handles gem spec parsing for Git gems
func parseGemSpecForGit(line string, currentGitGem **GitGemSpec) {
	if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
		// This is a gem spec inside a Git section
		if *currentGitGem == nil {
			*currentGitGem = &GitGemSpec{}
		}

		// Parse version from the gem spec line
		versionAndPlatform := matches[2]
		version := versionAndPlatform

		// For Git gems, usually no platform info in version
		(*currentGitGem).Name = matches[1]
		(*currentGitGem).Version = version
	}
}

// parseDependencyForGit handles dependency parsing for Git gems
func parseDependencyForGit(line string, currentGitGem *GitGemSpec) {
	if matches := depRegex.FindStringSubmatch(line); matches != nil && currentGitGem != nil {
		// Add dependency to current Git gem
		dep := Dependency{
			Name: matches[1],
		}
		if len(matches) > 2 && matches[2] != "" {
			dep.Constraints = parseConstraints(matches[2])
		}
		currentGitGem.Dependencies = append(currentGitGem.Dependencies, dep)
	}
}

// parseGemSpecForPath handles gem spec parsing for Path gems
func parseGemSpecForPath(line string, currentPathGem **PathGemSpec) {
	if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
		// This is a gem spec inside a PATH section
		if *currentPathGem == nil {
			*currentPathGem = &PathGemSpec{}
		}

		// Parse version from the gem spec line
		versionAndPlatform := matches[2]
		version := versionAndPlatform

		// For PATH gems, usually no platform info in version
		(*currentPathGem).Name = matches[1]
		(*currentPathGem).Version = version
	}
}

// parseDependencyForPath handles dependency parsing for Path gems
func parseDependencyForPath(line string, currentPathGem *PathGemSpec) {
	if matches := depRegex.FindStringSubmatch(line); matches != nil && currentPathGem != nil {
		// Add dependency to current PATH gem
		dep := Dependency{
			Name: matches[1],
		}
		if len(matches) > 2 && matches[2] != "" {
			dep.Constraints = parseConstraints(matches[2])
		}
		currentPathGem.Dependencies = append(currentPathGem.Dependencies, dep)
	}
}

// isGemExcluded checks if a gem should be excluded based on its groups
func isGemExcluded(gemGroups, excludeGroups []string) bool {
	for _, excludeGroup := range excludeGroups {
		for _, gemGroup := range gemGroups {
			if gemGroup == excludeGroup {
				return true
			}
		}
	}
	return false
}

// isGemIncluded checks if a gem should be included based on its groups
func isGemIncluded(gemGroups, includeGroups []string) bool {
	for _, includeGroup := range includeGroups {
		for _, gemGroup := range gemGroups {
			if gemGroup == includeGroup || gemGroup == "default" {
				return true
			}
		}
	}
	return false
}
