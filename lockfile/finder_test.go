package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGemfiles(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test 1: Standard Gemfile/Gemfile.lock
	if err := os.WriteFile("Gemfile", []byte("gem 'rails'"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("Gemfile.lock", []byte("GEM\n  specs:\n"), 0600); err != nil {
		t.Fatal(err)
	}

	paths, err := FindGemfiles()
	if err != nil {
		t.Fatalf("Expected to find Gemfile, got error: %v", err)
	}

	if filepath.Base(paths.Gemfile) != "Gemfile" {
		t.Errorf("Expected Gemfile, got %s", paths.Gemfile)
	}

	if filepath.Base(paths.GemfileLock) != "Gemfile.lock" {
		t.Errorf("Expected Gemfile.lock, got %s", paths.GemfileLock)
	}

	// Clean up
	_ = os.Remove("Gemfile")
	_ = os.Remove("Gemfile.lock")

	// Test 2: gems.rb/gems.locked
	if writeErr := os.WriteFile("gems.rb", []byte("gem 'rails'"), 0600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if writeErr := os.WriteFile("gems.locked", []byte("GEM\n  specs:\n"), 0600); writeErr != nil {
		t.Fatal(writeErr)
	}

	paths, err = FindGemfiles()
	if err != nil {
		t.Fatalf("Expected to find gems.rb, got error: %v", err)
	}

	if filepath.Base(paths.Gemfile) != "gems.rb" {
		t.Errorf("Expected gems.rb, got %s", paths.Gemfile)
	}

	if filepath.Base(paths.GemfileLock) != "gems.locked" {
		t.Errorf("Expected gems.locked, got %s", paths.GemfileLock)
	}

	// Clean up
	os.Remove("gems.rb")
	os.Remove("gems.locked")

	// Test 3: No files found
	_, err = FindGemfiles()
	if err == nil {
		t.Error("Expected error when no Gemfile found")
	}
}

func TestFindGemfilesWithBundleGemfile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create custom Gemfile
	customPath := filepath.Join(tmpDir, "MyGemfile")
	if err := os.WriteFile(customPath, []byte("gem 'rails'"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customPath+".lock", []byte("GEM\n  specs:\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	oldEnv := os.Getenv("BUNDLE_GEMFILE")
	defer os.Setenv("BUNDLE_GEMFILE", oldEnv)
	os.Setenv("BUNDLE_GEMFILE", customPath)

	paths, err := FindGemfiles()
	if err != nil {
		t.Fatalf("Expected to find custom Gemfile, got error: %v", err)
	}

	if filepath.Base(paths.Gemfile) != "MyGemfile" {
		t.Errorf("Expected MyGemfile, got %s", paths.Gemfile)
	}

	if filepath.Base(paths.GemfileLock) != "MyGemfile.lock" {
		t.Errorf("Expected MyGemfile.lock, got %s", paths.GemfileLock)
	}
}

func TestDetermineLockfilePath(t *testing.T) {
	tests := []struct {
		gemfile  string
		expected string
	}{
		{"/path/to/Gemfile", "/path/to/Gemfile.lock"},
		{"/path/to/gems.rb", "/path/to/gems.locked"},
		{"/path/to/MyGems", "/path/to/MyGems.lock"},
	}

	for _, test := range tests {
		result := determineLockfilePath(test.gemfile)
		if result != test.expected {
			t.Errorf("For %s, expected %s, got %s", test.gemfile, test.expected, result)
		}
	}
}
