package lockfile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	t.Run("basic lockfile", func(t *testing.T) {
		lf := &Lockfile{
			GemSpecs: []GemSpec{
				{
					Name:    "rails",
					Version: "8.1.0.rc1",
					Dependencies: []Dependency{
						{Name: "actionpack", Constraints: []string{"= 8.1.0.rc1"}},
						{Name: "activerecord", Constraints: []string{"= 8.1.0.rc1"}},
					},
				},
				{
					Name:    "actionpack",
					Version: "8.1.0.rc1",
					Dependencies: []Dependency{
						{Name: "rack", Constraints: []string{"~> 2.0", ">= 2.2.0"}},
					},
				},
			},
			Platforms: []string{"ruby", "x86_64-linux"},
			Dependencies: []Dependency{
				{Name: "rails", Constraints: []string{"~> 8.1.0.rc1"}},
			},
			BundledWith: "2.3.26",
		}

		var buf bytes.Buffer
		writer := NewLockfileWriter()
		err := writer.Write(lf, &buf)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		output := buf.String()

		// Verify sections are present
		if !strings.Contains(output, "GEM\n") {
			t.Error("Missing GEM section")
		}
		if !strings.Contains(output, "PLATFORMS\n") {
			t.Error("Missing PLATFORMS section")
		}
		if !strings.Contains(output, "DEPENDENCIES\n") {
			t.Error("Missing DEPENDENCIES section")
		}
		if !strings.Contains(output, "BUNDLED WITH\n") {
			t.Error("Missing BUNDLED WITH section")
		}

		// Verify gems are sorted
		actionpackIdx := strings.Index(output, "actionpack (8.1.0.rc1)")
		railsIdx := strings.Index(output, "rails (8.1.0.rc1)")
		if actionpackIdx == -1 || railsIdx == -1 || actionpackIdx >= railsIdx {
			t.Error("Gems not sorted correctly (actionpack should come before rails)")
		}

		// Verify dependency constraints
		if !strings.Contains(output, "rails (~> 8.1.0.rc1)") {
			t.Error("Missing dependency constraints for rails")
		}
		if !strings.Contains(output, "rack (~> 2.0, >= 2.2.0)") {
			t.Error("Missing multiple constraints for rack")
		}
	})

	t.Run("platform-specific gems", func(t *testing.T) {
		lf := &Lockfile{
			GemSpecs: []GemSpec{
				{
					Name:     "nokogiri",
					Version:  "1.13.8",
					Platform: "x86_64-darwin",
					Dependencies: []Dependency{
						{Name: "racc", Constraints: []string{"~> 1.4"}},
					},
				},
				{
					Name:    "racc",
					Version: "1.4.0",
				},
			},
			Platforms:   []string{"x86_64-darwin-21"},
			BundledWith: "2.3.26",
		}

		var buf bytes.Buffer
		writer := NewLockfileWriter()
		err := writer.Write(lf, &buf)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		output := buf.String()

		// Verify platform suffix in version
		if !strings.Contains(output, "nokogiri (1.13.8-x86_64-darwin)") {
			t.Error("Platform suffix not appended to version")
		}
	})

	t.Run("git sources", func(t *testing.T) {
		lf := &Lockfile{
			GitSpecs: []GitGemSpec{
				{
					Name:     "state_machines",
					Version:  "0.6.0",
					Remote:   "https://github.com/seuros/state_machines.git",
					Revision: "def456abc789",
					Branch:   "master",
				},
				{
					Name:     "no_fly_list",
					Version:  "0.6.0",
					Remote:   "https://github.com/seuros/no_fly_list.git",
					Revision: "abc123def456",
					Tag:      "v0.6.0",
					Dependencies: []Dependency{
						{Name: "activerecord", Constraints: []string{">= 6.0"}},
						{Name: "activesupport", Constraints: []string{">= 6.0"}},
					},
				},
			},
			Dependencies: []Dependency{
				{Name: "no_fly_list!"},
				{Name: "state_machines!"},
			},
			BundledWith: "2.4.13",
		}

		var buf bytes.Buffer
		writer := NewLockfileWriter()
		err := writer.Write(lf, &buf)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		output := buf.String()

		// Verify GIT sections
		if strings.Count(output, "GIT\n") != 2 {
			t.Errorf("Expected 2 GIT sections, found %d", strings.Count(output, "GIT\n"))
		}

		// Verify git metadata
		if !strings.Contains(output, "remote: https://github.com/seuros/no_fly_list.git") {
			t.Error("Missing git remote for no_fly_list")
		}
		if !strings.Contains(output, "revision: abc123def456") {
			t.Error("Missing git revision for no_fly_list")
		}
		if !strings.Contains(output, "tag: v0.6.0") {
			t.Error("Missing git tag for no_fly_list")
		}
		if !strings.Contains(output, "branch: master") {
			t.Error("Missing git branch for state_machines")
		}

		// Verify ! suffix preserved in dependencies
		if !strings.Contains(output, "no_fly_list!") {
			t.Error("Missing ! suffix for no_fly_list dependency")
		}
		if !strings.Contains(output, "state_machines!") {
			t.Error("Missing ! suffix for state_machines dependency")
		}
	})

	t.Run("path sources", func(t *testing.T) {
		lf := &Lockfile{
			PathSpecs: []PathGemSpec{
				{
					Name:    "my_local_gem",
					Version: "0.1.0",
					Remote:  "../gems/my_local_gem",
					Dependencies: []Dependency{
						{Name: "rails", Constraints: []string{">= 6.0"}},
					},
				},
			},
			Dependencies: []Dependency{
				{Name: "my_local_gem!"},
			},
			BundledWith: "2.4.13",
		}

		var buf bytes.Buffer
		writer := NewLockfileWriter()
		err := writer.Write(lf, &buf)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		output := buf.String()

		// Verify PATH section
		if !strings.Contains(output, "PATH\n") {
			t.Error("Missing PATH section")
		}
		if !strings.Contains(output, "remote: ../gems/my_local_gem") {
			t.Error("Missing path remote")
		}
		if !strings.Contains(output, "my_local_gem (0.1.0)") {
			t.Error("Missing path gem spec")
		}
	})

	t.Run("empty sections omitted", func(t *testing.T) {
		lf := &Lockfile{
			GemSpecs: []GemSpec{
				{Name: "rails", Version: "8.1.0.rc1"},
			},
			BundledWith: "2.3.26",
			// No GitSpecs, PathSpecs, Platforms, or Dependencies
		}

		var buf bytes.Buffer
		writer := NewLockfileWriter()
		err := writer.Write(lf, &buf)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		output := buf.String()

		// GEM and BUNDLED WITH should be present
		if !strings.Contains(output, "GEM\n") {
			t.Error("Missing GEM section")
		}
		if !strings.Contains(output, "BUNDLED WITH\n") {
			t.Error("Missing BUNDLED WITH section")
		}

		// Other sections should be omitted
		if strings.Contains(output, "GIT\n") {
			t.Error("Empty GIT section should be omitted")
		}
		if strings.Contains(output, "PATH\n") {
			t.Error("Empty PATH section should be omitted")
		}
		if strings.Contains(output, "PLATFORMS\n") {
			t.Error("Empty PLATFORMS section should be omitted")
		}
		if strings.Contains(output, "DEPENDENCIES\n") {
			t.Error("Empty DEPENDENCIES section should be omitted")
		}
	})

	t.Run("dependencies without constraints", func(t *testing.T) {
		lf := &Lockfile{
			GemSpecs: []GemSpec{
				{Name: "minitest", Version: "5.16.3"},
			},
			Dependencies: []Dependency{
				{Name: "minitest"}, // No constraints
			},
			BundledWith: "2.3.26",
		}

		var buf bytes.Buffer
		writer := NewLockfileWriter()
		err := writer.Write(lf, &buf)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		output := buf.String()

		// Find the DEPENDENCIES section
		depsSectionIdx := strings.Index(output, "DEPENDENCIES\n")
		if depsSectionIdx == -1 {
			t.Fatal("DEPENDENCIES section not found")
		}

		// The line after DEPENDENCIES should just be "  minitest" without parentheses
		depsSection := output[depsSectionIdx:]
		lines := strings.Split(depsSection, "\n")
		if len(lines) < 2 {
			t.Fatal("DEPENDENCIES section too short")
		}

		minitestLine := strings.TrimSpace(lines[1])
		if minitestLine != "minitest" {
			t.Errorf("Expected 'minitest', got '%s'", minitestLine)
		}
		if strings.Contains(minitestLine, "(") {
			t.Error("Dependency without constraints should not have parentheses")
		}
	})
}

func TestRoundTrip(t *testing.T) {
	testFiles := []string{
		"../testdata/Gemfile.lock",
		"../testdata/git.lock",
		"../testdata/platforms.lock",
	}

	for _, testFile := range testFiles {
		t.Run(filepath.Base(testFile), func(t *testing.T) {
			// Parse the original file
			original, err := ParseFile(testFile)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", testFile, err)
			}

			// Write it back
			var buf bytes.Buffer
			writer := NewLockfileWriter()
			err = writer.Write(original, &buf)
			if err != nil {
				t.Fatalf("Failed to write lockfile: %v", err)
			}

			// Parse the written output
			reparsed, err := Parse(&buf)
			if err != nil {
				t.Fatalf("Failed to reparse written lockfile: %v", err)
			}

			// Compare key fields
			if len(original.GemSpecs) != len(reparsed.GemSpecs) {
				t.Errorf("GemSpecs count mismatch: original=%d, reparsed=%d",
					len(original.GemSpecs), len(reparsed.GemSpecs))
			}

			if len(original.GitSpecs) != len(reparsed.GitSpecs) {
				t.Errorf("GitSpecs count mismatch: original=%d, reparsed=%d",
					len(original.GitSpecs), len(reparsed.GitSpecs))
			}

			if len(original.PathSpecs) != len(reparsed.PathSpecs) {
				t.Errorf("PathSpecs count mismatch: original=%d, reparsed=%d",
					len(original.PathSpecs), len(reparsed.PathSpecs))
			}

			if len(original.Dependencies) != len(reparsed.Dependencies) {
				t.Errorf("Dependencies count mismatch: original=%d, reparsed=%d",
					len(original.Dependencies), len(reparsed.Dependencies))
			}

			if original.BundledWith != reparsed.BundledWith {
				t.Errorf("BundledWith mismatch: original=%s, reparsed=%s",
					original.BundledWith, reparsed.BundledWith)
			}

			// Verify specific gems are preserved
			for _, originalGem := range original.GemSpecs {
				found := false
				for _, reparsedGem := range reparsed.GemSpecs {
					if reparsedGem.Name == originalGem.Name &&
						reparsedGem.Version == originalGem.Version &&
						reparsedGem.Platform == originalGem.Platform {
						found = true
						// Check dependencies count
						if len(originalGem.Dependencies) != len(reparsedGem.Dependencies) {
							t.Errorf("Gem %s: dependencies count mismatch: original=%d, reparsed=%d",
								originalGem.Name, len(originalGem.Dependencies), len(reparsedGem.Dependencies))
						}
						break
					}
				}
				if !found {
					t.Errorf("Gem %s (%s) not found in reparsed output", originalGem.Name, originalGem.Version)
				}
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	lf := &Lockfile{
		GemSpecs: []GemSpec{
			{Name: "rails", Version: "8.1.0.rc1"},
		},
		Platforms:   []string{"ruby"},
		BundledWith: "2.3.26",
	}

	// Create temp directory
	tmpDir := t.TempDir()
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")

	// Write to file
	writer := NewLockfileWriter()
	err := writer.WriteFile(lf, lockfilePath)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file exists
	if _, statErr := os.Stat(lockfilePath); os.IsNotExist(statErr) {
		t.Error("Lockfile was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(lockfilePath)
	if err != nil {
		t.Fatalf("Failed to read lockfile: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "rails (8.1.0.rc1)") {
		t.Error("Written file does not contain expected content")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	lf := &Lockfile{
		GemSpecs: []GemSpec{
			{Name: "test", Version: "1.0.0"},
		},
		BundledWith: "2.3.26",
	}

	t.Run("Write function", func(t *testing.T) {
		var buf bytes.Buffer
		err := Write(lf, &buf)
		if err != nil {
			t.Fatalf("Write convenience function failed: %v", err)
		}

		if !strings.Contains(buf.String(), "test (1.0.0)") {
			t.Error("Output does not contain expected content")
		}
	})

	t.Run("WriteFile function", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockfilePath := filepath.Join(tmpDir, "test.lock")

		err := WriteFile(lf, lockfilePath)
		if err != nil {
			t.Fatalf("WriteFile convenience function failed: %v", err)
		}

		if _, err := os.Stat(lockfilePath); os.IsNotExist(err) {
			t.Error("File was not created")
		}
	})
}

func TestIndentationAndFormatting(t *testing.T) {
	lf := &Lockfile{
		GemSpecs: []GemSpec{
			{
				Name:    "rails",
				Version: "8.1.0.rc1",
				Dependencies: []Dependency{
					{Name: "actionpack", Constraints: []string{"= 8.1.0.rc1"}},
				},
			},
		},
		Platforms: []string{"ruby"},
		Dependencies: []Dependency{
			{Name: "rails", Constraints: []string{"~> 8.1.0.rc1"}},
		},
		BundledWith: "2.3.26",
	}

	var buf bytes.Buffer
	writer := NewLockfileWriter()
	err := writer.Write(lf, &buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	lines := strings.Split(buf.String(), "\n")

	// Check indentation levels
	for i, line := range lines {
		// Remote lines should have 2-space indent
		if strings.Contains(line, "remote:") && !strings.HasPrefix(line, "  remote:") {
			t.Errorf("Line %d: remote should have 2-space indent: %q", i+1, line)
		}

		// Gem specs should have 4-space indent
		if strings.Contains(line, "rails (8.1.0.rc1)") && !strings.HasPrefix(line, "    rails") {
			t.Errorf("Line %d: gem spec should have 4-space indent: %q", i+1, line)
		}

		// Dependencies under gems should have 6-space indent
		if strings.Contains(line, "actionpack (= 8.1.0.rc1)") && !strings.HasPrefix(line, "      actionpack") {
			t.Errorf("Line %d: gem dependency should have 6-space indent: %q", i+1, line)
		}

		// Platform entries should have 2-space indent
		if strings.TrimSpace(line) == "ruby" && strings.HasPrefix(buf.String()[strings.Index(buf.String(), line)-20:], "PLATFORMS") {
			if !strings.HasPrefix(line, "  ruby") {
				t.Errorf("Line %d: platform should have 2-space indent: %q", i+1, line)
			}
		}

		// Top-level dependencies should have 2-space indent
		if strings.Contains(line, "rails (~> 8.1.0.rc1)") && !strings.HasPrefix(line, "  rails") {
			t.Errorf("Line %d: dependency should have 2-space indent: %q", i+1, line)
		}

		// BUNDLED WITH version should have 3-space indent
		if line == "   2.3.26" && !strings.HasPrefix(line, "   ") {
			t.Errorf("Line %d: bundled version should have 3-space indent: %q", i+1, line)
		}
	}
}

func TestPlatformDeduplication(t *testing.T) {
	lf := &Lockfile{
		GemSpecs: []GemSpec{
			{Name: "test", Version: "1.0.0"},
		},
		Platforms:   []string{"ruby", "x86_64-linux", "ruby", "x86_64-linux"}, // Duplicates
		BundledWith: "2.3.26",
	}

	var buf bytes.Buffer
	writer := NewLockfileWriter()
	err := writer.Write(lf, &buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()

	// Count occurrences of each platform
	rubyCount := strings.Count(output, "  ruby\n")
	linuxCount := strings.Count(output, "  x86_64-linux\n")

	if rubyCount != 1 {
		t.Errorf("Expected 'ruby' platform to appear once, found %d times", rubyCount)
	}
	if linuxCount != 1 {
		t.Errorf("Expected 'x86_64-linux' platform to appear once, found %d times", linuxCount)
	}
}
