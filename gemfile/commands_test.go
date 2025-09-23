package gemfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAddGemCommand tests the add gem command
func TestAddGemCommand(t *testing.T) {
	tests := []struct {
		name            string
		initialGemfile  string
		opts            AddOptions
		expectedErr     string
		expectedContent string
	}{
		{
			name: "add simple gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name: "rspec",
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec'`,
		},
		{
			name: "add gem with version",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name:    "rspec",
				Version: "~> 3.0",
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec', '~> 3.0'`,
		},
		{
			name: "add gem with strict version",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name:    "rspec",
				Version: "3.12.0",
				Strict:  true,
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec', '= 3.12.0'`,
		},
		{
			name: "add gem with optimistic version",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name:       "rspec",
				Version:    "3.0.0",
				Optimistic: true,
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec', '>= 3.0.0'`,
		},
		{
			name: "add gem with groups",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name:   "rspec",
				Groups: []string{"development", "test"},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec', groups: [:development, :test]`,
		},
		{
			name: "add git gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name: "my_gem",
				Git:  "https://github.com/user/my_gem.git",
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'my_gem', github: 'user/my_gem'`,
		},
		{
			name: "add github gem with branch",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name:   "my_gem",
				Github: "user/my_gem",
				Branch: "develop",
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'my_gem', github: 'user/my_gem', branch: 'develop'`,
		},
		{
			name: "add path gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name: "local_gem",
				Path: "./vendor/local_gem",
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'local_gem', path: './vendor/local_gem'`,
		},
		{
			name: "add gem with require false",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name:    "bootsnap",
				Require: func() *string { s := "false"; return &s }(),
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'
gem 'bootsnap', require: false`,
		},
		{
			name:           "error on empty name",
			initialGemfile: `source 'https://rubygems.org'`,
			opts: AddOptions{
				Name: "",
			},
			expectedErr: "gem name is required",
		},
		{
			name: "error on duplicate gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: AddOptions{
				Name: "rails",
			},
			expectedErr: "failed to add gem to Gemfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			gemfilePath := filepath.Join(tmpDir, "Gemfile")

			// Write initial content
			err := os.WriteFile(gemfilePath, []byte(tt.initialGemfile), 0600)
			if err != nil {
				t.Fatalf("Failed to write initial Gemfile: %v", err)
			}

			// Run add command
			err = AddGemCommand(gemfilePath, tt.opts)

			// Check error expectation
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q but got none", tt.expectedErr)
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

// TestRemoveGemCommand tests the remove gem command
func TestRemoveGemCommand(t *testing.T) {
	tests := []struct {
		name            string
		initialGemfile  string
		opts            RemoveOptions
		expectedErr     string
		expectedContent string
	}{
		{
			name: "remove single gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec'`,
			opts: RemoveOptions{
				GemNames: []string{"rspec"},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'`,
		},
		{
			name: "remove multiple gems",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'
gem 'rspec'
gem 'factory_bot'`,
			opts: RemoveOptions{
				GemNames: []string{"rspec", "factory_bot"},
			},
			expectedContent: `source 'https://rubygems.org'

gem 'rails'`,
		},
		{
			name: "error on empty gem names",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: RemoveOptions{
				GemNames: []string{},
			},
			expectedErr: "at least one gem name is required",
		},
		{
			name: "error on nonexistent gem",
			initialGemfile: `source 'https://rubygems.org'

gem 'rails'`,
			opts: RemoveOptions{
				GemNames: []string{"nonexistent"},
			},
			expectedErr: "failed to remove gem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			gemfilePath := filepath.Join(tmpDir, "Gemfile")

			// Write initial content
			err := os.WriteFile(gemfilePath, []byte(tt.initialGemfile), 0600)
			if err != nil {
				t.Fatalf("Failed to write initial Gemfile: %v", err)
			}

			// Run remove command
			err = RemoveGemCommand(gemfilePath, tt.opts)

			// Check error expectation
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q but got none", tt.expectedErr)
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

// TestParseGroups tests group parsing
func TestParseGroups(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{"default"}},
		{"development", []string{"development"}},
		{"development,test", []string{"development", "test"}},
		{"development, test", []string{"development", "test"}},
		{" development , test ", []string{"development", "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseGroups(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %v but got %v", tt.expected, result)
			}
			for i, group := range result {
				if group != tt.expected[i] {
					t.Fatalf("Expected %v but got %v", tt.expected, result)
				}
			}
		})
	}
}

// TestParseRequire tests require parsing
func TestParseRequire(t *testing.T) {
	tests := []struct {
		input    string
		expected *string
	}{
		{"", nil},
		{"false", func() *string { s := ""; return &s }()},
		{"specific_file", func() *string { s := "specific_file"; return &s }()},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseRequire(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Fatalf("Expected nil but got %v", *result)
				}
			} else {
				if result == nil {
					t.Fatalf("Expected %v but got nil", *tt.expected)
				}
				if *result != *tt.expected {
					t.Fatalf("Expected %v but got %v", *tt.expected, *result)
				}
			}
		})
	}
}

// TestFindGemfile tests Gemfile discovery
func TestFindGemfile(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test default when no files exist
	result := findGemfile()
	if result != "Gemfile" {
		t.Fatalf("Expected 'Gemfile' but got %q", result)
	}

	// Test Gemfile found
	_ = os.WriteFile("Gemfile", []byte("# test"), 0600)
	result = findGemfile()
	if result != "Gemfile" {
		t.Fatalf("Expected 'Gemfile' but got %q", result)
	}

	// Test gems.rb found when Gemfile doesn't exist
	os.Remove("Gemfile")
	_ = os.WriteFile("gems.rb", []byte("# test"), 0600)
	result = findGemfile()
	if result != "gems.rb" {
		t.Fatalf("Expected 'gems.rb' but got %q", result)
	}

	// Test Gemfile takes precedence over gems.rb
	_ = os.WriteFile("Gemfile", []byte("# test"), 0600)
	result = findGemfile()
	if result != "Gemfile" {
		t.Fatalf("Expected 'Gemfile' but got %q", result)
	}
}
