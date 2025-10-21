package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/gemfile"
)

func main() {
	// Get the test Gemfile path
	testGemfile := filepath.Join("testdata", "source_blocks_test_gemfile")

	// Parse the Gemfile
	parser := gemfile.NewGemfileParser(testGemfile)
	parsed, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing Gemfile: %v\n", err)
		os.Exit(1)
	}

	// Display parsed sources
	fmt.Println("=== SOURCES ===")
	for i, source := range parsed.Sources {
		fmt.Printf("%d. Type: %s, URL: %s\n", i+1, source.Type, source.URL)
	}

	// Display gems with their sources
	fmt.Println("\n=== GEMS AND THEIR SOURCES ===")
	for _, dep := range parsed.Dependencies {
		fmt.Printf("%-20s", dep.Name)
		if dep.Source != nil {
			fmt.Printf(" -> Source: %s (%s)", dep.Source.URL, dep.Source.Type)
		} else {
			fmt.Printf(" -> Source: (default)")
		}
		if len(dep.Constraints) > 0 {
			fmt.Printf(", Version: %v", dep.Constraints)
		}
		if len(dep.Groups) > 0 && !(len(dep.Groups) == 1 && dep.Groups[0] == "default") {
			fmt.Printf(", Groups: %v", dep.Groups)
		}
		fmt.Println()
	}
}
