package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	gemfileName     = "Gemfile"
	gemfileLockName = "Gemfile.lock"
	gemsRbName      = "gems.rb"
	gemsLockedName  = "gems.locked"
)

// FilePaths contains the paths to Gemfile and Gemfile.lock
type FilePaths struct {
	Gemfile     string
	GemfileLock string
}

// FindGemfiles locates the Gemfile and corresponding lock file
// Supports:
// - BUNDLE_GEMFILE environment variable
// - BUNDLE_LOCKFILE environment variable (like Bundler)
// - Gemfile / Gemfile.lock (default)
// - gems.rb / gems.locked
func FindGemfiles() (*FilePaths, error) {
	// Check BUNDLE_LOCKFILE (explicit override, like Bundler)
	bundleLockfile := os.Getenv("BUNDLE_LOCKFILE")

	// Check BUNDLE_GEMFILE environment variable first
	if bundleGemfile := os.Getenv("BUNDLE_GEMFILE"); bundleGemfile != "" {
		gemfile, err := filepath.Abs(bundleGemfile)
		if err != nil {
			return nil, fmt.Errorf("invalid BUNDLE_GEMFILE path: %w", err)
		}

		if _, statErr := os.Stat(gemfile); os.IsNotExist(statErr) {
			return nil, fmt.Errorf(
				"❌ BUNDLE_GEMFILE points to non-existent file\n   Path: %s\n"+
					"   💡 Check the file path or unset BUNDLE_GEMFILE", gemfile)
		}

		// Use BUNDLE_LOCKFILE if set, otherwise derive from gemfile
		var lockfile string
		if bundleLockfile != "" {
			lockfile, err = filepath.Abs(bundleLockfile)
			if err != nil {
				return nil, fmt.Errorf("invalid BUNDLE_LOCKFILE path: %w", err)
			}

			if _, err := os.Stat(lockfile); os.IsNotExist(err) {
				return nil, fmt.Errorf(
					"❌ BUNDLE_LOCKFILE points to non-existent file\n   Path: %s\n"+
						"   💡 Check the file path or unset BUNDLE_LOCKFILE", lockfile)
			}
		} else {
			lockfile = determineLockfilePath(gemfile)
		}

		return &FilePaths{
			Gemfile:     gemfile,
			GemfileLock: lockfile,
		}, nil
	}

	// Try standard naming conventions
	candidates := []struct {
		gemfile  string
		lockfile string
	}{
		{gemfileName, gemfileLockName},
		{gemsRbName, gemsLockedName},
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.gemfile); err != nil {
			continue
		}

		// Found Gemfile, check if lockfile exists
		lockfile := candidate.lockfile
		if _, err := os.Stat(lockfile); os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"❌ Found %s but %s is missing\n"+
					"   💡 Run 'bundle install' or 'bundle lock' to generate the lockfile",
				candidate.gemfile, lockfile)
		}

		abs_gemfile, _ := filepath.Abs(candidate.gemfile)
		abs_lockfile, _ := filepath.Abs(lockfile)

		return &FilePaths{
			Gemfile:     abs_gemfile,
			GemfileLock: abs_lockfile,
		}, nil
	}

	return nil, fmt.Errorf(
		"❌ No Gemfile found in current directory\n   Looked for: Gemfile, gems.rb\n" +
			"   💡 Create a Gemfile or set BUNDLE_GEMFILE environment variable")
}

// DetermineLockfilePath determines the lock file path based on the Gemfile path.
// For custom gemfiles (e.g., Appraisal), returns the discrete lockfile path (gemfile + ".lock")
// without fallback behavior. Each custom gemfile requires its own lockfile.
func DetermineLockfilePath(gemfilePath string) string {
	return determineLockfilePath(gemfilePath)
}

// determineLockfilePath determines the lock file path based on the Gemfile path
// For custom gemfiles (e.g., Appraisal), returns the discrete lockfile path (gemfile + ".lock")
// without fallback behavior. Each custom gemfile requires its own lockfile.
func determineLockfilePath(gemfilePath string) string {
	dir := filepath.Dir(gemfilePath)
	base := filepath.Base(gemfilePath)

	switch base {
	case gemfileName:
		return filepath.Join(dir, gemfileLockName)
	case gemsRbName:
		return filepath.Join(dir, gemsLockedName)
	default:
		// For custom names (e.g., Appraisal gemfiles), use discrete lockfile
		return gemfilePath + ".lock"
	}
}

// FindLockfileOnly finds just the lockfile for install-only operations
func FindLockfileOnly() (string, error) {
	paths, err := FindGemfiles()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(paths.GemfileLock); os.IsNotExist(err) {
		return "", fmt.Errorf("❌ Lockfile not found: %s\n   💡 Run 'bundle install' to generate the lockfile", filepath.Base(paths.GemfileLock))
	}

	return paths.GemfileLock, nil
}

// GetGemfileName returns a user-friendly name for the Gemfile
func (fp *FilePaths) GetGemfileName() string {
	base := filepath.Base(fp.Gemfile)
	if base == gemfileName || base == gemsRbName {
		return base
	}
	return fmt.Sprintf("%s (via BUNDLE_GEMFILE)", base)
}

// GetLockfileName returns a user-friendly name for the lockfile
func (fp *FilePaths) GetLockfileName() string {
	return filepath.Base(fp.GemfileLock)
}
