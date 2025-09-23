package lockfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if second.Name != StateMachinesGem || second.Branch != "master" {
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
		{Name: RailsGem, Groups: []string{"default", "production"}},
		{Name: "rubocop", Groups: []string{"development"}},
		{Name: "rspec", Groups: []string{"test"}},
	}

	filtered := FilterGemsByGroups(gems, []string{"production"}, nil)
	if len(filtered) != 1 || filtered[0].Name != RailsGem {
		t.Errorf("--only production failed: %v", filtered)
	}

	filtered = FilterGemsByGroups(gems, nil, []string{"test"})
	if len(filtered) != 2 {
		t.Errorf("--without test failed: %v", filtered)
	}

	filtered = FilterGemsByGroups(gems, []string{"production"}, []string{"development"})
	if len(filtered) != 1 || filtered[0].Name != RailsGem {
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
	lockfileContent := `GIT
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

	lockfile, err := Parse(strings.NewReader(lockfileContent))
	if err != nil {
		t.Fatalf("Failed to parse lockfile: %v", err)
	}

	// Test PATH gems parsed correctly
	if len(lockfile.PathSpecs) != 3 {
		t.Errorf("Expected 3 PATH gems, got %d", len(lockfile.PathSpecs))
	}

	// Test individual PATH gems
	testFirstPathGem(t, &lockfile.PathSpecs[0])
	testSecondPathGem(t, &lockfile.PathSpecs[1])
	testThirdPathGem(t, &lockfile.PathSpecs[2])

	// Test that Git gems are still parsed correctly alongside PATH gems
	testGitGemsStillWork(t, lockfile)

	// Test semantic versioning
	testSemVerParsing(t, lockfile.PathSpecs)
}

// testFirstPathGem tests the first PATH gem (commonshare_cms)
func testFirstPathGem(t *testing.T, cms *PathGemSpec) {
	if cms.Name != "commonshare_cms" {
		t.Errorf("Expected first PATH gem name 'commonshare_cms', got '%s'", cms.Name)
	}
	if cms.Version != "0.6.1" {
		t.Errorf("Expected first PATH gem version '0.6.1', got '%s'", cms.Version)
	}
	if cms.Remote != "components/cms" {
		t.Errorf("Expected first PATH gem remote 'components/cms', got '%s'", cms.Remote)
	}
	if len(cms.Dependencies) != 7 {
		t.Errorf("Expected 7 dependencies for commonshare_cms, got %d", len(cms.Dependencies))
	}
}

// testSecondPathGem tests the second PATH gem (common_insight)
func testSecondPathGem(t *testing.T, insight *PathGemSpec) {
	if insight.Name != "common_insight" {
		t.Errorf("Expected second PATH gem name 'common_insight', got '%s'", insight.Name)
	}
	if insight.Version != VersionStr {
		t.Errorf("Expected second PATH gem version '0.1.0', got '%s'", insight.Version)
	}
	if insight.Remote != "components/common_insight" {
		t.Errorf("Expected second PATH gem remote 'components/common_insight', got '%s'", insight.Remote)
	}
	if len(insight.Dependencies) != 4 {
		t.Errorf("Expected 4 dependencies for common_insight, got %d", len(insight.Dependencies))
	}
}

// testThirdPathGem tests the third PATH gem (frontend_link)
func testThirdPathGem(t *testing.T, frontend *PathGemSpec) {
	if frontend.Name != "frontend_link" {
		t.Errorf("Expected third PATH gem name 'frontend_link', got '%s'", frontend.Name)
	}
	if frontend.Version != VersionStr {
		t.Errorf("Expected third PATH gem version '0.1.0', got '%s'", frontend.Version)
	}
	if frontend.Remote != "components/frontend_link" {
		t.Errorf("Expected third PATH gem remote 'components/frontend_link', got '%s'", frontend.Remote)
	}
	if len(frontend.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies for frontend_link, got %d", len(frontend.Dependencies))
	}
}

// testGitGemsStillWork tests that Git gems work alongside PATH gems
func testGitGemsStillWork(t *testing.T, lockfile *Lockfile) {
	if len(lockfile.GitSpecs) != 1 {
		t.Errorf("Expected 1 Git gem, got %d", len(lockfile.GitSpecs))
		return
	}

	git := lockfile.GitSpecs[0]
	if git.Name != StateMachinesGem {
		t.Errorf("Expected Git gem name 'state_machines', got '%s'", git.Name)
	}

	// Test that regular gems are still parsed correctly
	if len(lockfile.GemSpecs) != 2 {
		t.Errorf("Expected 2 regular gems, got %d", len(lockfile.GemSpecs))
	}
}

// testSemVerParsing tests semantic version parsing for PATH gems
func testSemVerParsing(t *testing.T, pathSpecs []PathGemSpec) {
	if len(pathSpecs) == 0 {
		return
	}

	cms := pathSpecs[0]
	if cms.FullName() != "commonshare_cms-0.6.1" {
		t.Errorf("Expected FullName 'commonshare_cms-0.6.1', got '%s'", cms.FullName())
	}

	_, err := cms.SemVer()
	if err != nil {
		t.Errorf("PATH gem SemVer parsing failed: %v", err)
	}
}
