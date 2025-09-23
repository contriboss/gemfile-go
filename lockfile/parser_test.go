package lockfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// Test-specific constants
	testRailsGem = "rails"
	testVersion  = "0.1.0"
)

func TestParse(t *testing.T) {
	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    actionpack (7.0.4)
      actionview (= 7.0.4)
      activesupport (= 7.0.4)
      rack (~> 2.0, >= 2.2.0)
    actionview (7.0.4)
      activesupport (= 7.0.4)
      builder (~> 3.1)
    activesupport (7.0.4)
      concurrent-ruby (~> 1.0, >= 1.0.2)
      i18n (>= 1.6, < 2)
    builder (3.2.4)
    nokogiri (1.13.8-x86_64-darwin)
      racc (~> 1.4)

PLATFORMS
  x86_64-darwin-21

DEPENDENCIES
  actionpack (~> 7.0)

BUNDLED WITH
   2.3.26`

	lockfile, err := Parse(strings.NewReader(lockfileContent))
	if err != nil {
		t.Fatalf("Failed to parse lockfile: %v", err)
	}

	// Test gem specs
	if len(lockfile.GemSpecs) != 5 {
		t.Errorf("Expected 5 gem specs, got %d", len(lockfile.GemSpecs))
	}

	// Test specific gem
	actionpack := findGem(lockfile.GemSpecs, "actionpack")
	if actionpack == nil {
		t.Error("actionpack gem not found")
	} else {
		if actionpack.Version != "7.0.4" {
			t.Errorf("Expected actionpack version 7.0.4, got %s", actionpack.Version)
		}
		if len(actionpack.Dependencies) != 3 {
			t.Errorf("Expected 3 dependencies for actionpack, got %d", len(actionpack.Dependencies))
		}
	}

	// Test platform-specific gem
	nokogiri := findGem(lockfile.GemSpecs, "nokogiri")
	if nokogiri == nil {
		t.Error("nokogiri gem not found")
	} else if nokogiri.Platform != "x86_64-darwin" {
		t.Errorf("Expected nokogiri platform x86_64-darwin, got %s", nokogiri.Platform)
	}

	// Test platforms
	if len(lockfile.Platforms) != 1 || lockfile.Platforms[0] != "x86_64-darwin-21" {
		t.Errorf("Expected platform x86_64-darwin-21, got %v", lockfile.Platforms)
	}

	// Test dependencies
	if len(lockfile.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(lockfile.Dependencies))
	}

	// Test bundled with
	if lockfile.BundledWith != "2.3.26" {
		t.Errorf("Expected bundled with 2.3.26, got %s", lockfile.BundledWith)
	}
}

func findGem(gems []GemSpec, name string) *GemSpec {
	for i := range gems {
		if gems[i].Name == name {
			return &gems[i]
		}
	}
	return nil
}
func TestParseGitLockfile(t *testing.T) {
	data, err := os.ReadFile("../testdata/git.lock")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	lockfile, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to parse lockfile: %v", err)
	}

	if len(lockfile.GitSpecs) != 2 {
		t.Fatalf("expected 2 git gems, got %d", len(lockfile.GitSpecs))
	}

	first := lockfile.GitSpecs[0]
	if first.Name != "no_fly_list" || first.Tag != "v0.6.0" {
		t.Errorf("unexpected first git gem: %+v", first)
	}

	second := lockfile.GitSpecs[1]
	if second.Name != stateMachinesGem || second.Branch != "master" {
		t.Errorf("unexpected second git gem: %+v", second)
	}

	if len(lockfile.Platforms) != 1 || lockfile.Platforms[0] != "x86_64-linux" {
		t.Errorf("platforms parsed incorrectly: %v", lockfile.Platforms)
	}
}

func TestParsePlatformsLockfile(t *testing.T) {
	data, err := os.ReadFile("../testdata/platforms.lock")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	lockfile, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to parse lockfile: %v", err)
	}

	if len(lockfile.Platforms) != 3 {
		t.Fatalf("expected 3 platforms, got %d", len(lockfile.Platforms))
	}

	nDarwin := findGem(lockfile.GemSpecs, "nokogiri")
	if nDarwin == nil {
		t.Fatalf("nokogiri not parsed")
	}

	platforms := map[string]bool{}
	for _, gem := range lockfile.GemSpecs {
		if gem.Name == "nokogiri" {
			platforms[gem.Platform] = true
		}
	}
	if !platforms["arm64-darwin"] || !platforms["x86_64-linux"] {
		t.Errorf("expected both darwin and linux variants, got %v", platforms)
	}
}

func TestFilterGemsByGroups(t *testing.T) {
	gems := []GemSpec{
		{Name: "rails", Groups: []string{"default", "production"}},
		{Name: "rubocop", Groups: []string{"development"}},
		{Name: "rspec", Groups: []string{"test"}},
	}

	filtered := FilterGemsByGroups(gems, []string{"production"}, nil)
	if len(filtered) != 1 || filtered[0].Name != testRailsGem {
		t.Errorf("--only production failed: %v", filtered)
	}

	filtered = FilterGemsByGroups(gems, nil, []string{"test"})
	if len(filtered) != 2 {
		t.Errorf("--without test failed: %v", filtered)
	}

	filtered = FilterGemsByGroups(gems, []string{"production"}, []string{"development"})
	if len(filtered) != 1 || filtered[0].Name != testRailsGem {
		t.Errorf("combined only/without failed: %v", filtered)
	}
}

func TestParseBundler1File(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "testdata", "bundler1.lock"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	lf, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if lf.BundledWith != "1.17.3" {
		t.Errorf("expected bundler 1.17.3, got %s", lf.BundledWith)
	}
	if len(lf.GemSpecs) == 0 {
		t.Errorf("expected gems parsed")
	}
}

func TestParseBundler2File(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "testdata", "bundler2.lock"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	lf, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if lf.BundledWith != "2.4.13" {
		t.Errorf("expected bundler 2.4.13, got %s", lf.BundledWith)
	}
	if len(lf.GemSpecs) == 0 {
		t.Errorf("expected gems parsed")
	}
}

func TestParsePathGems(t *testing.T) {
	lockfileContent := getPathGemsTestData()

	lockfile, err := Parse(strings.NewReader(lockfileContent))
	if err != nil {
		t.Fatalf("Failed to parse lockfile: %v", err)
	}

	validatePathGemsCount(t, lockfile)
	validatePathGems(t, lockfile)
	validateGitGems(t, lockfile)
	validateRegularGems(t, lockfile)
	validatePathGemMethods(t, lockfile)
}

// getPathGemsTestData returns the test lockfile content
func getPathGemsTestData() string {
	return `GIT
  remote: https://github.com/state-machines/state_machines.git
  revision: e9d8375b5a94ee859e52496f36a9120411ec08e5
  branch: master
  specs:
    state_machines (0.10.0)

PATH
  remote: components/cms
  specs:
    commonshare_cms (0.6.1)
      actionpack (>= 7.2)
      activejob (>= 7.2)
      activerecord (>= 7.2)
      rails_app_version
      railties (>= 7.2)
      redcarpet
      words_counted

PATH
  remote: components/common_insight
  specs:
    common_insight (0.1.0)
      actionpack (>= 7.2)
      activejob (>= 7.2)
      activerecord (>= 7.2)
      rails_app_version

PATH
  remote: components/frontend_link
  specs:
    frontend_link (0.1.0)
      actionpack (>= 7.1)
      railties (>= 7.1)

GEM
  remote: https://rubygems.org/
  specs:
    actionpack (8.0.2)
      actionview (= 8.0.2)
      activesupport (= 8.0.2)
      nokogiri (>= 1.8.5)
    rails_app_version (1.3.2)
      railties (>= 7.0, < 8.1)

PLATFORMS
  arm64-darwin
  x86_64-darwin

DEPENDENCIES
  commonshare_cms!
  common_insight!
  frontend_link!
  state_machines!

BUNDLED WITH
   2.6.9`
}

// validatePathGemsCount validates the count of PATH gems
func validatePathGemsCount(t *testing.T, lockfile *Lockfile) {
	if len(lockfile.PathSpecs) != 3 {
		t.Errorf("Expected 3 PATH gems, got %d", len(lockfile.PathSpecs))
	}
}

// validatePathGems validates individual PATH gems
func validatePathGems(t *testing.T, lockfile *Lockfile) {
	pathGemTests := []struct {
		index       int
		name        string
		version     string
		remote      string
		depCount    int
		description string
	}{
		{0, "commonshare_cms", "0.6.1", "components/cms", 7, "first"},
		{1, "common_insight", testVersion, "components/common_insight", 4, "second"},
		{2, "frontend_link", testVersion, "components/frontend_link", 2, "third"},
	}

	for _, test := range pathGemTests {
		if test.index >= len(lockfile.PathSpecs) {
			continue
		}
		gem := lockfile.PathSpecs[test.index]
		validatePathGem(t, &gem, test.name, test.version, test.remote, test.depCount, test.description)
	}
}

// validatePathGem validates a single PATH gem
func validatePathGem(t *testing.T, gem *PathGemSpec, expectedName, expectedVersion,
	expectedRemote string, expectedDepCount int, description string) {
	if gem.Name != expectedName {
		t.Errorf("Expected %s PATH gem name '%s', got '%s'", description, expectedName, gem.Name)
	}
	if gem.Version != expectedVersion {
		t.Errorf("Expected %s PATH gem version '%s', got '%s'", description, expectedVersion, gem.Version)
	}
	if gem.Remote != expectedRemote {
		t.Errorf("Expected %s PATH gem remote '%s', got '%s'", description, expectedRemote, gem.Remote)
	}
	if len(gem.Dependencies) != expectedDepCount {
		t.Errorf("Expected %d dependencies for %s, got %d", expectedDepCount, expectedName, len(gem.Dependencies))
	}
}

// validateGitGems validates Git gems parsing
func validateGitGems(t *testing.T, lockfile *Lockfile) {
	if len(lockfile.GitSpecs) != 1 {
		t.Errorf("Expected 1 Git gem, got %d", len(lockfile.GitSpecs))
		return
	}

	git := lockfile.GitSpecs[0]
	if git.Name != "state_machines" {
		t.Errorf("Expected Git gem name 'state_machines', got '%s'", git.Name)
	}
}

// validateRegularGems validates regular gems parsing
func validateRegularGems(t *testing.T, lockfile *Lockfile) {
	if len(lockfile.GemSpecs) != 2 {
		t.Errorf("Expected 2 regular gems, got %d", len(lockfile.GemSpecs))
	}
}

// validatePathGemMethods validates PATH gem methods
func validatePathGemMethods(t *testing.T, lockfile *Lockfile) {
	if len(lockfile.PathSpecs) == 0 {
		return
	}

	cms := lockfile.PathSpecs[0]
	if cms.FullName() != "commonshare_cms-0.6.1" {
		t.Errorf("Expected FullName 'commonshare_cms-0.6.1', got '%s'", cms.FullName())
	}

	_, err := cms.SemVer()
	if err != nil {
		t.Errorf("PATH gem SemVer parsing failed: %v", err)
	}
}
