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
		if newSection := checkSectionHeaders(line); newSection != "" {
			savePendingGems(lockfile, &currentGem, &currentGitGem, &currentPathGem)
			currentSection = newSection
			continue
		}

		// Handle special lines
		if handleSpecialLines(line, currentSection, &currentGitGem, &currentPathGem) {
			continue
		}

		// Process content based on current section
		processSection(line, currentSection, lockfile, &currentGem, &currentGitGem, &currentPathGem)
	}

	// Finalize parsing
	finalizeGems(lockfile, currentGem, currentGitGem, currentPathGem)

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("❌ Error reading lockfile\n   💡 File may be corrupted - try regenerating with 'bundle lock'")
	}

	return lockfile, nil
}

// checkSectionHeaders checks if a line is a section header and returns the section name
func checkSectionHeaders(line string) string {
	switch line {
	case sectionGEM:
		return sectionGEM
	case sectionGIT:
		return sectionGIT
	case sectionPATH:
		return sectionPATH
	case sectionPLATFORMS:
		return sectionPLATFORMS
	case sectionDEPENDENCIES:
		return sectionDEPENDENCIES
	}

	if strings.HasPrefix(line, "BUNDLED WITH") {
		return sectionBUNDLED_WITH
	}

	return ""
}

// handleSpecialLines handles special lines like remote, revision, branch, tag, and specs
func handleSpecialLines(line, currentSection string, currentGitGem **GitGemSpec, currentPathGem **PathGemSpec) bool {
	if strings.HasPrefix(line, "  remote:") {
		handleRemoteLine(line, currentSection, currentGitGem, currentPathGem)
		return true
	}

	if strings.HasPrefix(line, "  revision:") && currentSection == sectionGIT {
		handleRevisionLine(line, currentGitGem)
		return true
	}

	if strings.HasPrefix(line, "  branch:") && currentSection == sectionGIT {
		handleBranchLine(line, currentGitGem)
		return true
	}

	if strings.HasPrefix(line, "  tag:") && currentSection == sectionGIT {
		handleTagLine(line, currentGitGem)
		return true
	}

	if strings.HasPrefix(line, "  specs:") {
		return true // Skip specs header
	}

	return false
}

// handleRemoteLine processes remote lines for GIT and PATH sections
func handleRemoteLine(line, currentSection string, currentGitGem **GitGemSpec, currentPathGem **PathGemSpec) {
	remote := strings.TrimSpace(strings.TrimPrefix(line, "  remote:"))

	switch currentSection {
	case sectionGIT:
		if *currentGitGem == nil {
			*currentGitGem = &GitGemSpec{}
		}
		(*currentGitGem).Remote = remote
	case sectionPATH:
		if *currentPathGem == nil {
			*currentPathGem = &PathGemSpec{}
		}
		(*currentPathGem).Remote = remote
	}
}

// handleRevisionLine processes revision lines for GIT sections
func handleRevisionLine(line string, currentGitGem **GitGemSpec) {
	revision := strings.TrimSpace(strings.TrimPrefix(line, "  revision:"))
	if *currentGitGem == nil {
		*currentGitGem = &GitGemSpec{}
	}
	(*currentGitGem).Revision = revision
}

// handleBranchLine processes branch lines for GIT sections
func handleBranchLine(line string, currentGitGem **GitGemSpec) {
	branch := strings.TrimSpace(strings.TrimPrefix(line, "  branch:"))
	if *currentGitGem == nil {
		*currentGitGem = &GitGemSpec{}
	}
	(*currentGitGem).Branch = branch
}

// handleTagLine processes tag lines for GIT sections
func handleTagLine(line string, currentGitGem **GitGemSpec) {
	tag := strings.TrimSpace(strings.TrimPrefix(line, "  tag:"))
	if *currentGitGem == nil {
		*currentGitGem = &GitGemSpec{}
	}
	(*currentGitGem).Tag = tag
}

// processSection processes content lines based on the current section
func processSection(line, currentSection string, lockfile *Lockfile,
	currentGem **GemSpec, currentGitGem **GitGemSpec, currentPathGem **PathGemSpec) {
	switch currentSection {
	case sectionGEM:
		processGemSection(line, lockfile, currentGem, gemSpecRegex, depRegex)
	case sectionGIT:
		processGitPathSection(line, currentGitGem, currentPathGem, true, gemSpecRegex, depRegex)
	case sectionPATH:
		processGitPathSection(line, currentGitGem, currentPathGem, false, gemSpecRegex, depRegex)
	case sectionPLATFORMS:
		processPlatformsSection(line, lockfile)
	case sectionDEPENDENCIES:
		processDependenciesSection(line, lockfile)
	case "BUNDLED_WITH":
		processBundledWithSection(line, lockfile)
	}
}

// processPlatformsSection processes lines in the PLATFORMS section
func processPlatformsSection(line string, lockfile *Lockfile) {
	if strings.HasPrefix(line, "  ") {
		platform := strings.TrimSpace(line)
		lockfile.Platforms = append(lockfile.Platforms, platform)
	}
}

// processDependenciesSection processes lines in the DEPENDENCIES section
func processDependenciesSection(line string, lockfile *Lockfile) {
	if !strings.HasPrefix(line, "  ") {
		return
	}

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

// processBundledWithSection processes lines in the BUNDLED_WITH section
func processBundledWithSection(line string, lockfile *Lockfile) {
	if strings.HasPrefix(line, "   ") {
		lockfile.BundledWith = strings.TrimSpace(line)
	}
}

// finalizeGems adds any remaining gems to the lockfile
func finalizeGems(lockfile *Lockfile, currentGem *GemSpec, currentGitGem *GitGemSpec, currentPathGem *PathGemSpec) {
	if currentGem != nil {
		lockfile.GemSpecs = append(lockfile.GemSpecs, *currentGem)
	}
	if currentGitGem != nil {
		lockfile.GitSpecs = append(lockfile.GitSpecs, *currentGitGem)
	}
	if currentPathGem != nil {
		lockfile.PathSpecs = append(lockfile.PathSpecs, *currentPathGem)
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

// ParseResult represents the result of parsing a gem spec section line
type ParseResult struct {
	IsGemSpec      bool
	GemName        string
	GemVersion     string
	IsDep          bool
	DepName        string
	DepConstraints string
}

// parseGemSpecSection handles parsing gem specs within GIT or PATH sections
func parseGemSpecSection(line string, gemSpecRegex, depRegex *regexp.Regexp) ParseResult {
	if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
		return ParseResult{
			IsGemSpec:  true,
			GemName:    matches[1],
			GemVersion: matches[2],
		}
	} else if matches := depRegex.FindStringSubmatch(line); matches != nil {
		constraints := ""
		if len(matches) > 2 && matches[2] != "" {
			constraints = matches[2]
		}
		return ParseResult{
			IsDep:          true,
			DepName:        matches[1],
			DepConstraints: constraints,
		}
	}
	return ParseResult{}
}

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

// processGemSection processes lines in the GEM section
func processGemSection(line string, lockfile *Lockfile, currentGem **GemSpec, gemSpecRegex, depRegex *regexp.Regexp) {
	if matches := gemSpecRegex.FindStringSubmatch(line); matches != nil {
		// Save current gem before starting new one
		if *currentGem != nil {
			lockfile.GemSpecs = append(lockfile.GemSpecs, **currentGem)
		}

		// Parse gem name and version
		name := matches[1]
		versionAndPlatform := matches[2]
		version := versionAndPlatform
		platform := ""

		// Check if version contains platform info (e.g., "1.13.8-x86_64-darwin")
		parts := strings.Split(versionAndPlatform, "-")
		hasPlatformInfo := strings.Contains(versionAndPlatform, "x86") ||
			strings.Contains(versionAndPlatform, "darwin") ||
			strings.Contains(versionAndPlatform, "linux") ||
			strings.Contains(versionAndPlatform, "java")
		if len(parts) >= 3 && hasPlatformInfo {
			// Assume version is the first part, platform is the rest
			version = parts[0]
			platform = strings.Join(parts[1:], "-")
		}

		// Start new gem
		*currentGem = &GemSpec{
			Name:     name,
			Version:  version,
			Platform: platform,
		}
	} else if matches := depRegex.FindStringSubmatch(line); matches != nil && *currentGem != nil {
		// Add dependency to current gem
		dep := Dependency{
			Name: matches[1],
		}
		if len(matches) > 2 && matches[2] != "" {
			dep.Constraints = parseConstraints(matches[2])
		}
		(*currentGem).Dependencies = append((*currentGem).Dependencies, dep)
	}
}

// processGitPathSection processes lines in GIT or PATH sections
func processGitPathSection(
	line string, currentGitGem **GitGemSpec, currentPathGem **PathGemSpec,
	isGitSection bool, gemSpecRegex, depRegex *regexp.Regexp) {
	result := parseGemSpecSection(line, gemSpecRegex, depRegex)
	if isGitSection {
		if result.IsGemSpec {
			if *currentGitGem == nil {
				*currentGitGem = &GitGemSpec{}
			}
			(*currentGitGem).Name = result.GemName
			(*currentGitGem).Version = result.GemVersion
		} else if result.IsDep && *currentGitGem != nil {
			dep := Dependency{Name: result.DepName}
			if result.DepConstraints != "" {
				dep.Constraints = parseConstraints(result.DepConstraints)
			}
			(*currentGitGem).Dependencies = append((*currentGitGem).Dependencies, dep)
		}
	} else {
		if result.IsGemSpec {
			if *currentPathGem == nil {
				*currentPathGem = &PathGemSpec{}
			}
			(*currentPathGem).Name = result.GemName
			(*currentPathGem).Version = result.GemVersion
		} else if result.IsDep && *currentPathGem != nil {
			dep := Dependency{Name: result.DepName}
			if result.DepConstraints != "" {
				dep.Constraints = parseConstraints(result.DepConstraints)
			}
			(*currentPathGem).Dependencies = append((*currentPathGem).Dependencies, dep)
		}
	}
}

// FilterGemsByGroups filters gems based on included/excluded groups
func FilterGemsByGroups(gems []GemSpec, includeGroups, excludeGroups []string) []GemSpec {
	if len(includeGroups) == 0 && len(excludeGroups) == 0 {
		return gems // No filtering needed
	}

	var filtered []GemSpec
	for i := range gems {
		gem := &gems[i]
		gemGroups := getGemGroups(gem)

		if isGemExcluded(gemGroups, excludeGroups) {
			continue
		}

		if !isGemIncluded(gemGroups, includeGroups) {
			continue
		}

		filtered = append(filtered, *gem)
	}

	return filtered
}

// getGemGroups returns the groups for a gem, defaulting to "default" if none specified
func getGemGroups(gem *GemSpec) []string {
	if len(gem.Groups) == 0 {
		return []string{"default"}
	}
	return gem.Groups
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
	if len(includeGroups) == 0 {
		return true // No include filter means include all
	}

	for _, includeGroup := range includeGroups {
		for _, gemGroup := range gemGroups {
			if gemGroup == includeGroup || gemGroup == "default" {
				return true
			}
		}
	}
	return false
}
