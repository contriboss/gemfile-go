package lockfile

import (
	"os"
	"path/filepath"
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

	// Create Appraisal.root.gemfile and Gemfile.lock (no Appraisal.root.gemfile.lock)
	// This simulates the Appraisal pattern where custom gemfiles share the main lockfile
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

func TestFindGemfilesAppraisalFallback(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create Appraisal.root.gemfile and Gemfile.lock (no Appraisal.root.gemfile.lock)
	// Test automatic fallback WITHOUT setting BUNDLE_LOCKFILE
	customGemfile := filepath.Join(tmpDir, "Appraisal.root.gemfile")
	if err := os.WriteFile(customGemfile, []byte("gemspec"), 0600); err != nil {
		t.Fatal(err)
	}
	mainLockfile := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(mainLockfile, []byte("GEM\n  specs:\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Set only BUNDLE_GEMFILE - should auto-fallback to Gemfile.lock
	oldGemfile := os.Getenv("BUNDLE_GEMFILE")
	oldLockfile := os.Getenv("BUNDLE_LOCKFILE")
	defer func() {
		os.Setenv("BUNDLE_GEMFILE", oldGemfile)
		os.Setenv("BUNDLE_LOCKFILE", oldLockfile)
	}()

	os.Setenv("BUNDLE_GEMFILE", customGemfile)
	os.Setenv("BUNDLE_LOCKFILE", "") // Explicitly unset

	paths, err := FindGemfiles()
	if err != nil {
		t.Fatalf("Expected to find files with fallback, got error: %v", err)
	}

	if filepath.Base(paths.Gemfile) != appraisalRootGemfile {
		t.Errorf("Expected %s, got %s", appraisalRootGemfile, paths.Gemfile)
	}

	// Should auto-fallback to Gemfile.lock since Appraisal.root.gemfile.lock doesn't exist
	if filepath.Base(paths.GemfileLock) != gemfileLockName {
		t.Errorf("Expected Gemfile.lock (fallback), got %s", paths.GemfileLock)
	}
}

func TestFindGemfilesSubdirectoryFallback(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create gemfiles/style.gemfile and Gemfile.lock in parent (project root)
	// This is the typical Appraisal directory structure
	gemfilesDir := filepath.Join(tmpDir, "gemfiles")
	if err := os.MkdirAll(gemfilesDir, 0755); err != nil {
		t.Fatal(err)
	}
	customGemfile := filepath.Join(gemfilesDir, "style.gemfile")
	if err := os.WriteFile(customGemfile, []byte("gemspec"), 0600); err != nil {
		t.Fatal(err)
	}
	// Lockfile in project root (parent of gemfiles/)
	mainLockfile := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(mainLockfile, []byte("GEM\n  specs:\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Set only BUNDLE_GEMFILE - should auto-fallback to parent's Gemfile.lock
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
		t.Fatalf("Expected to find files with parent fallback, got error: %v", err)
	}

	if filepath.Base(paths.Gemfile) != "style.gemfile" {
		t.Errorf("Expected style.gemfile, got %s", paths.Gemfile)
	}

	// Should auto-fallback to parent's Gemfile.lock
	if filepath.Base(paths.GemfileLock) != gemfileLockName {
		t.Errorf("Expected Gemfile.lock (parent fallback), got %s", paths.GemfileLock)
	}

	// Verify it's the parent's lockfile, not gemfiles/Gemfile.lock
	if filepath.Dir(paths.GemfileLock) == gemfilesDir {
		t.Errorf("Expected parent Gemfile.lock, got one in gemfiles/")
	}
}
