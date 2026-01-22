package lockfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const appraisalRootGemfile = "Appraisal.root.gemfile"

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

	if filepath.Base(paths.GemfileLock) != gemfileLockName {
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

func TestFindGemfilesWithBundleLockfile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create custom gemfile and test explicit BUNDLE_LOCKFILE override
	// Note: Appraisals normally use discrete lockfiles (e.g., gemfiles/*.gemfile.lock)
	// This test verifies that BUNDLE_LOCKFILE can explicitly override the default behavior
	customGemfile := filepath.Join(tmpDir, "Appraisal.root.gemfile")
	if err := os.WriteFile(customGemfile, []byte("gemspec"), 0600); err != nil {
		t.Fatal(err)
	}
	mainLockfile := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(mainLockfile, []byte("GEM\n  specs:\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Set both env vars (like Bundler supports)
	oldGemfile := os.Getenv("BUNDLE_GEMFILE")
	oldLockfile := os.Getenv("BUNDLE_LOCKFILE")
	defer func() {
		os.Setenv("BUNDLE_GEMFILE", oldGemfile)
		os.Setenv("BUNDLE_LOCKFILE", oldLockfile)
	}()

	os.Setenv("BUNDLE_GEMFILE", customGemfile)
	os.Setenv("BUNDLE_LOCKFILE", mainLockfile)

	paths, err := FindGemfiles()
	if err != nil {
		t.Fatalf("Expected to find files, got error: %v", err)
	}

	if filepath.Base(paths.Gemfile) != appraisalRootGemfile {
		t.Errorf("Expected %s, got %s", appraisalRootGemfile, paths.Gemfile)
	}

	if filepath.Base(paths.GemfileLock) != gemfileLockName {
		t.Errorf("Expected Gemfile.lock, got %s", paths.GemfileLock)
	}
}

func TestFindGemfilesWithInvalidBundleLockfile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a valid gemfile
	customGemfile := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(customGemfile, []byte("gem 'rails'"), 0600); err != nil {
		t.Fatal(err)
	}

	// Set BUNDLE_LOCKFILE to a non-existent file
	oldGemfile := os.Getenv("BUNDLE_GEMFILE")
	oldLockfile := os.Getenv("BUNDLE_LOCKFILE")
	defer func() {
		os.Setenv("BUNDLE_GEMFILE", oldGemfile)
		os.Setenv("BUNDLE_LOCKFILE", oldLockfile)
	}()

	os.Setenv("BUNDLE_GEMFILE", customGemfile)
	os.Setenv("BUNDLE_LOCKFILE", "/nonexistent/path/to/lockfile.lock")

	_, err := FindGemfiles()
	if err == nil {
		t.Fatal("Expected error when BUNDLE_LOCKFILE points to non-existent file, got nil")
	}

	expectedMsg := "BUNDLE_LOCKFILE points to non-existent file"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got: %v", expectedMsg, err)
	}
}

func TestFindGemfilesAppraisalDiscreteLockfile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create Appraisal.root.gemfile and its discrete lockfile
	customGemfile := filepath.Join(tmpDir, "Appraisal.root.gemfile")
	if err := os.WriteFile(customGemfile, []byte("gemspec"), 0600); err != nil {
		t.Fatal(err)
	}
	discreteLockfile := filepath.Join(tmpDir, "Appraisal.root.gemfile.lock")
	if err := os.WriteFile(discreteLockfile, []byte("GEM\n  specs:\n"), 0600); err != nil {
		t.Fatal(err)
	}

	oldGemfile := os.Getenv("BUNDLE_GEMFILE")
	oldLockfile := os.Getenv("BUNDLE_LOCKFILE")
	defer func() {
		os.Setenv("BUNDLE_GEMFILE", oldGemfile)
		os.Setenv("BUNDLE_LOCKFILE", oldLockfile)
	}()

	os.Setenv("BUNDLE_GEMFILE", customGemfile)
	os.Setenv("BUNDLE_LOCKFILE", "")

	paths, err := FindGemfiles()
	if err != nil {
		t.Fatalf("Expected to find files with discrete lockfile, got error: %v", err)
	}

	if filepath.Base(paths.Gemfile) != appraisalRootGemfile {
		t.Errorf("Expected %s, got %s", appraisalRootGemfile, paths.Gemfile)
	}

	// Should use discrete lockfile (Appraisal.root.gemfile.lock)
	expectedLockfile := "Appraisal.root.gemfile.lock"
	if filepath.Base(paths.GemfileLock) != expectedLockfile {
		t.Errorf("Expected %s (discrete lockfile), got %s", expectedLockfile, paths.GemfileLock)
	}
}
