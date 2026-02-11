package gemfile

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	gemRspec = `gem "rspec"`
	rspec    = "rspec"
)

func TestParseEvalGemfile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create main Gemfile
	mainGemfileContent := `
source "https://rubygems.org"
gem "rails"
eval_gemfile "modular/rspec.gemfile"
`
	mainGemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(mainGemfilePath, []byte(mainGemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write main Gemfile: %v", err)
	}

	// Create modular directory
	modularDir := filepath.Join(tmpDir, "modular")
	if err := os.Mkdir(modularDir, 0700); err != nil {
		t.Fatalf("Failed to create modular directory: %v", err)
	}

	// Create rspec.gemfile
	rspecGemfileContent := "\n" + gemRspec + `
eval_gemfile "nested.gemfile"
`
	rspecGemfilePath := filepath.Join(modularDir, "rspec.gemfile")
	if err := os.WriteFile(rspecGemfilePath, []byte(rspecGemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write rspec Gemfile: %v", err)
	}

	// Create nested.gemfile
	nestedGemfileContent := `
gem "rspec-core"
`
	nestedGemfilePath := filepath.Join(modularDir, "nested.gemfile")
	if err := os.WriteFile(nestedGemfilePath, []byte(nestedGemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write nested Gemfile: %v", err)
	}

	parser := NewGemfileParser(mainGemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	expectedGems := []string{"rails", rspec, "rspec-core"}
	for _, expected := range expectedGems {
		found := false
		for _, gem := range parsed.Dependencies {
			if gem.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected gem %s not found", expected)
		}
	}

	if len(parsed.Dependencies) != 3 {
		t.Errorf("Expected 3 gems, got %d", len(parsed.Dependencies))
	}
}

func TestParseEvalGemfileAbsolute(t *testing.T) {
	tmpDir := t.TempDir()

	// Create modular directory
	modularDir := filepath.Join(tmpDir, "modular")
	if err := os.Mkdir(modularDir, 0700); err != nil {
		t.Fatalf("Failed to create modular directory: %v", err)
	}

	// Create rspec.gemfile
	rspecGemfileContent := gemRspec
	rspecGemfilePath := filepath.Join(modularDir, "rspec.gemfile")
	if err := os.WriteFile(rspecGemfilePath, []byte(rspecGemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write rspec Gemfile: %v", err)
	}

	// Create main Gemfile with absolute path
	mainGemfileContent := `eval_gemfile "` + rspecGemfilePath + `"`
	mainGemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(mainGemfilePath, []byte(mainGemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write main Gemfile: %v", err)
	}

	parser := NewGemfileParser(mainGemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	if len(parsed.Dependencies) != 1 || parsed.Dependencies[0].Name != rspec {
		t.Errorf("Expected gem rspec, got %v", parsed.Dependencies)
	}
}

func TestParseEvalGemfileWithGroups(t *testing.T) {
	tmpDir := t.TempDir()

	// Create modular.gemfile
	modularContent := gemRspec
	modularPath := filepath.Join(tmpDir, "modular.gemfile")
	if err := os.WriteFile(modularPath, []byte(modularContent), 0600); err != nil {
		t.Fatalf("Failed to write modular Gemfile: %v", err)
	}

	// Create main Gemfile with groups
	mainGemfileContent := `
group :test do
  eval_gemfile "modular.gemfile"
end
`
	mainGemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(mainGemfilePath, []byte(mainGemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write main Gemfile: %v", err)
	}

	parser := NewGemfileParser(mainGemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	if len(parsed.Dependencies) != 1 {
		t.Fatalf("Expected 1 gem, got %d", len(parsed.Dependencies))
	}

	gem := parsed.Dependencies[0]
	if gem.Name != rspec {
		t.Errorf("Expected gem rspec, got %s", gem.Name)
	}

	foundTestGroup := false
	for _, g := range gem.Groups {
		if g == "test" {
			foundTestGroup = true
			break
		}
	}
	if !foundTestGroup {
		t.Errorf("Expected gem to be in 'test' group, got %v", gem.Groups)
	}
}
