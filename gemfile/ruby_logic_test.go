package gemfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRubyLogicInGemfile(t *testing.T) {
	tmpDir := t.TempDir()

	gemfileContent := `
if ENV.fetch("KETTLE_RB_DEV", "false").casecmp?("true")
  gem "kettle-dev", "1.0.0"
else
  gem "kettle-dev", "2.0.0"
end

if ENV["TEST_VAR"] == "1"
  gem "test-gem-1"
elsif ENV["TEST_VAR"] == "2"
  gem "test-gem-2"
else
  gem "test-gem-default"
end

unless ENV["SKIP_GEM"]
  gem "not-skipped"
end

if RUBY_VERSION >= "3.0.0"
  gem "modern-ruby-gem"
end
`
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	tests := []struct {
		name         string
		env          map[string]string
		expectedGems []string
		excludedGems []string
	}{
		{
			name: "KETTLE_RB_DEV true",
			env: map[string]string{
				"KETTLE_RB_DEV": "true",
				"TEST_VAR":      "1",
				"RUBY_VERSION":  "3.2.0",
			},
			expectedGems: []string{"kettle-dev", "test-gem-1", "not-skipped"},
			excludedGems: []string{"test-gem-2", "test-gem-default", "modern-ruby-gem"},
		},
		{
			name: "KETTLE_RB_DEV false",
			env: map[string]string{
				"KETTLE_RB_DEV": "false",
				"TEST_VAR":      "2",
				"SKIP_GEM":      "true",
				"RUBY_VERSION":  "2.7.0",
			},
			expectedGems: []string{"kettle-dev", "test-gem-2"},
			excludedGems: []string{"test-gem-1", "test-gem-default", "not-skipped", "modern-ruby-gem"},
		},
		{
			name: "KETTLE_RB_DEV empty string (should use empty, not default 'false')",
			env: map[string]string{
				"KETTLE_RB_DEV": "",
				"TEST_VAR":      "1",
				"RUBY_VERSION":  "3.2.0",
			},
			// Empty string != "true", so should use else branch (version 2.0.0)
			expectedGems: []string{"kettle-dev", "test-gem-1", "not-skipped"},
			excludedGems: []string{"test-gem-2", "test-gem-default", "modern-ruby-gem"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			parser := NewGemfileParser(gemfilePath)
			_, err := parser.Parse()
			// Now expect an error because of RUBY_VERSION condition
			if err == nil {
				t.Fatal("Expected error due to RUBY_VERSION condition, but got none")
			}
			if !strings.Contains(err.Error(), "RUBY_VERSION") {
				t.Errorf("Expected error to mention RUBY_VERSION, got: %v", err)
			}
		})
	}
}

func TestEnvUnsetVsEmptyString(t *testing.T) {
	tmpDir := t.TempDir()

	gemfileContent := `
# Test ENV["X"] == "" with unset variable (should be false)
if ENV["UNSET_VAR"] == ""
  gem "should-not-appear-unset-eq"
end

# Test ENV["X"] == "" with empty string (should be true)
if ENV["EMPTY_VAR"] == ""
  gem "should-appear-empty-eq"
end

# Test ENV["X"] != "" with unset variable (should be true, nil != "")
if ENV["UNSET_VAR"] != ""
  gem "should-appear-unset-neq"
end

# Test ENV["X"] != "" with empty string (should be false)
if ENV["EMPTY_VAR"] != ""
  gem "should-not-appear-empty-neq"
end

# Test ENV["X"] != "" with set variable (should be true)
if ENV["SET_VAR"] != ""
  gem "should-appear-set-neq"
end
`
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Set up environment variables for this test
	testEnvKeys := []string{"UNSET_VAR", "EMPTY_VAR", "SET_VAR"}
	envVars := map[string]string{
		"EMPTY_VAR": "",
		"SET_VAR":   "value",
		// UNSET_VAR intentionally not in map
	}
	for _, key := range testEnvKeys {
		if v, ok := envVars[key]; ok {
			// Variable should be set for this test case.
			t.Setenv(key, v)
		} else {
			// Variable should be unset for this test case.
			// t.Setenv records the original value (or lack thereof) and will restore it.
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}

	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	expectedGems := []string{"should-appear-empty-eq", "should-appear-unset-neq", "should-appear-set-neq"}
	excludedGems := []string{"should-not-appear-unset-eq", "should-not-appear-empty-neq"}

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

	for _, excluded := range excludedGems {
		for _, gem := range parsed.Dependencies {
			if gem.Name == excluded {
				t.Errorf("Gem %s should have been excluded", excluded)
			}
		}
	}
}

func TestEnvTruthiness(t *testing.T) {
	tmpDir := t.TempDir()

	gemfileContent := `
# Test ENV["X"] truthy with unset variable (should be false)
if ENV["UNSET_VAR"]
  gem "should-not-appear-unset"
end

# Test ENV["X"] truthy with empty string (should be true)
if ENV["EMPTY_VAR"]
  gem "should-appear-empty"
end

# Test ENV["X"] truthy with set variable (should be true)
if ENV["SET_VAR"]
  gem "should-appear-set"
end

# Test unless ENV["X"] with unset variable (should execute, since unset is falsy)
unless ENV["UNSET_VAR"]
  gem "should-appear-unless-unset"
end

# Test unless ENV["X"] with empty string (should not execute)
unless ENV["EMPTY_VAR"]
  gem "should-not-appear-unless-empty"
end
`
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0600); err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Set up environment variables for this test
	testEnvKeys := []string{"UNSET_VAR", "EMPTY_VAR", "SET_VAR"}
	envVars := map[string]string{
		"EMPTY_VAR": "",
		"SET_VAR":   "value",
		// UNSET_VAR intentionally not in map
	}
	for _, key := range testEnvKeys {
		if v, ok := envVars[key]; ok {
			// Variable should be set for this test case.
			t.Setenv(key, v)
		} else {
			// Variable should be unset for this test case.
			// t.Setenv records the original value (or lack thereof) and will restore it.
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}

	parser := NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	expectedGems := []string{"should-appear-empty", "should-appear-set", "should-appear-unless-unset"}
	excludedGems := []string{"should-not-appear-unset", "should-not-appear-unless-empty"}

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

	for _, excluded := range excludedGems {
		for _, gem := range parsed.Dependencies {
			if gem.Name == excluded {
				t.Errorf("Gem %s should have been excluded", excluded)
			}
		}
	}
}
