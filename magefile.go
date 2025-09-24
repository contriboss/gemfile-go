//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default target when "mage" is run without arguments
var Default = Build

// Build compiles the library and ensures it's valid
func Build() error {
	mg.Deps(Fmt, Vet)
	fmt.Println("📦 Building gemfile-go...")
	return sh.Run("go", "build", "./...")
}

// Test runs all tests with coverage
func Test() error {
	mg.Deps(Build)
	fmt.Println("🧪 Running tests with coverage...")
	return sh.Run("go", "test", "-v", "-cover", "./...")
}

// TestRace runs tests with race detector
func TestRace() error {
	fmt.Println("🏃 Running tests with race detector...")
	return sh.Run("go", "test", "-race", "./...")
}

// Fmt formats the code
func Fmt() error {
	fmt.Println("✨ Formatting code...")
	return sh.Run("go", "fmt", "./...")
}

// Vet runs go vet for static analysis
func Vet() error {
	fmt.Println("🔍 Running static analysis...")
	return sh.Run("go", "vet", "./...")
}

// Lint runs golangci-lint
func Lint() error {
	fmt.Println("🎯 Running linter...")
	if err := sh.Run("which", "golangci-lint"); err != nil {
		fmt.Println("Installing golangci-lint...")
		if err := sh.Run("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"); err != nil {
			return err
		}
	}
	return sh.Run("golangci-lint", "run")
}

// Bench runs benchmarks
func Bench() error {
	fmt.Println("⚡ Running benchmarks...")
	return sh.Run("go", "test", "-bench=.", "./...")
}

// Deps downloads dependencies
func Deps() error {
	fmt.Println("📥 Downloading dependencies...")
	return sh.Run("go", "mod", "download")
}

// Tidy tidies go.mod
func Tidy() error {
	fmt.Println("🧹 Tidying go.mod...")
	return sh.Run("go", "mod", "tidy")
}

// CI runs all checks for continuous integration
func CI() error {
	mg.SerialDeps(Deps, Fmt, Vet, Lint, Test, TestRace)
	fmt.Println("✅ All CI checks passed!")
	return nil
}

// Examples runs the example scripts
func Examples() error {
	fmt.Println("🎭 Running examples...")

	examples := []string{
		"examples/parse_lockfile.go",
		"examples/parse_gemfile.go",
		"examples/find_gem.go",
	}

	for _, example := range examples {
		if _, err := os.Stat(example); os.IsNotExist(err) {
			continue
		}
		fmt.Printf("Running %s...\n", example)
		cmd := exec.Command("go", "run", example)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %s: %w", example, err)
		}
	}

	return nil
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("🧽 Cleaning build artifacts...")
	return sh.Run("go", "clean", "-cache", "-testcache")
}
