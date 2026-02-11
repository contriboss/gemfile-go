package gemfile

import (
	"os"
	"path/filepath"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for this test case and ensure they are restored afterwards.
			testEnvKeys := []string{"KETTLE_RB_DEV", "TEST_VAR", "SKIP_GEM", "RUBY_VERSION"}
			for _, key := range testEnvKeys {
				if v, ok := tt.env[key]; ok {
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

			for _, expected := range tt.expectedGems {
				found := false
				for _, gem := range parsed.Dependencies {
					if gem.Name == expected {
						found = true
						if expected == "kettle-dev" {
							expectedVersion := "2.0.0"
							if tt.env["KETTLE_RB_DEV"] == "true" {
								expectedVersion = "1.0.0"
							}
							if len(gem.Constraints) == 0 || gem.Constraints[0] != expectedVersion {
								t.Errorf("Expected kettle-dev version %s, got %v", expectedVersion, gem.Constraints)
							}
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected gem %s not found", expected)
				}
			}

			for _, excluded := range tt.excludedGems {
				for _, gem := range parsed.Dependencies {
					if gem.Name == excluded {
						t.Errorf("Gem %s should have been excluded", excluded)
					}
				}
			}
		})
	}
}
