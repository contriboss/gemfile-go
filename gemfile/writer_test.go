package gemfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGemfileWriter_AddGem tests adding gems to a Gemfile
func TestGemfileWriter_AddGem(t *testing.T) {
	tests := []struct {
		name            string
		initialGemfile  string
		gem             GemDependency
		expectedErr     string
		expectedContent string
	}{
		{
			name: "add simple gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:   "rspec",
				Groups: []string{"default"},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec'`,
		},
		{
			name: "add gem with version",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:        "rspec",
				Constraints: []string{"~> 3.0"},
				Groups:      []string{"default"},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec', '~> 3.0'`,
		},
		{
			name: "add gem with group",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:   "rspec",
				Groups: []string{"test"},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec', group: :test`,
		},
		{
			name: "add gem with multiple groups",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:   "factory_bot",
				Groups: []string{"development", "test"},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'factory_bot', groups: [:development, :test]`,
		},
		{
			name: "add git gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:   "my_gem",
				Groups: []string{"default"},
				Source: &Source{
					Type: "git",
					URL:  "https://github.com/user/my_gem.git",
				},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'my_gem', github: 'user/my_gem'`,
		},
		{
			name: "add github gem with branch",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:   "my_gem",
				Groups: []string{"default"},
				Source: &Source{
					Type:   "git",
					URL:    "https://github.com/user/my_gem.git",
					Branch: "main",
				},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'my_gem', github: 'user/my_gem', branch: 'main'`,
		},
		{
			name: "add path gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:   "local_gem",
				Groups: []string{"default"},
				Source: &Source{
					Type: "path",
					URL:  "./vendor/local_gem",
				},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'local_gem', path: './vendor/local_gem'`,
		},
		{
			name: "add gem with require false",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:    "bootsnap",
				Groups:  []string{"default"},
				Require: func() *string { s := ""; return &s }(),
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'bootsnap', require: false`,
		},
		{
			name: "duplicate gem error",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gem: GemDependency{
				Name:   "rails",
				Groups: []string{"default"},
			},
			expectedErr: `gem "rails" already exists in Gemfile`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			gemfilePath := filepath.Join(tmpDir, "Gemfile")

			// Write initial content
			err := os.WriteFile(gemfilePath, []byte(tt.initialGemfile), 0644)
			if err != nil {
				t.Fatalf("Failed to write initial Gemfile: %v", err)
			}

			// Create writer and add gem
			writer := NewGemfileWriter(gemfilePath)
			err = writer.AddGem(tt.gem)

			// Check error expectation
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("Expected error %q but got none", tt.expectedErr)
				}
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf("Expected error containing %q but got %q", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check content
			content, err := os.ReadFile(gemfilePath)
			if err != nil {
				t.Fatalf("Failed to read Gemfile: %v", err)
			}

			if string(content) != tt.expectedContent {
				t.Fatalf("Expected content:\n%s\n\nActual content:\n%s", tt.expectedContent, string(content))
			}
		})
	}
}

// TestGemfileWriter_RemoveGem tests removing gems from a Gemfile
func TestGemfileWriter_RemoveGem(t *testing.T) {
	tests := []struct {
		name            string
		initialGemfile  string
		gemToRemove     string
		expectedErr     string
		expectedContent string
	}{
		{
			name: "remove simple gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec'`,
			gemToRemove: "rspec",
			expectedContent: `source 'https://rubygems.org'

gem 'rails'`,
		},
		{
			name: "remove gem with version",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails', '~> 7.0'
gem 'rspec', '~> 3.0'`,
			gemToRemove: "rspec",
			expectedContent: `source 'https://rubygems.org'

gem 'rails', '~> 7.0'`,
		},
		{
			name: "remove gem with complex options",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'
gem 'my_gem', github: 'user/my_gem', branch: 'main'`,
			gemToRemove: "my_gem",
			expectedContent: `source 'https://rubygems.org'

gem 'rails'`,
		},
		{
			name: "remove nonexistent gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			gemToRemove: "rspec",
			expectedErr: `gem "rspec" not found in Gemfile`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			gemfilePath := filepath.Join(tmpDir, "Gemfile")

			// Write initial content
			err := os.WriteFile(gemfilePath, []byte(tt.initialGemfile), 0644)
			if err != nil {
				t.Fatalf("Failed to write initial Gemfile: %v", err)
			}

			// Create writer and remove gem
			writer := NewGemfileWriter(gemfilePath)
			err = writer.RemoveGem(tt.gemToRemove)

			// Check error expectation
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("Expected error %q but got none", tt.expectedErr)
				}
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf("Expected error containing %q but got %q", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check content
			content, err := os.ReadFile(gemfilePath)
			if err != nil {
				t.Fatalf("Failed to read Gemfile: %v", err)
			}

			if string(content) != tt.expectedContent {
				t.Fatalf("Expected content:\n%s\n\nActual content:\n%s", tt.expectedContent, string(content))
			}
		})
	}
}

// TestExtractGitHubPath tests GitHub URL parsing
func TestExtractGitHubPath(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/user/repo.git", "user/repo"},
		{"https://github.com/user/repo", "user/repo"},
		{"git@github.com:user/repo.git", "user/repo"},
		{"https://github.com/user/repo/", "user/repo"},
		{"https://example.com/repo.git", ""},
		{"not-a-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := extractGitHubPath(tt.url)
			if result != tt.expected {
				t.Fatalf("Expected %q but got %q", tt.expected, result)
			}
		})
	}
}

// TestIsDefaultGroup tests default group detection
func TestIsDefaultGroup(t *testing.T) {
	tests := []struct {
		groups   []string
		expected bool
	}{
		{[]string{"default"}, true},
		{[]string{"development"}, false},
		{[]string{"development", "test"}, false},
		{[]string{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.groups, ","), func(t *testing.T) {
			result := isDefaultGroup(tt.groups)
			if result != tt.expected {
				t.Fatalf("Expected %v but got %v", tt.expected, result)
			}
		})
	}
}
