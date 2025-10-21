package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/gemfile"
)

func main() {
	// Get the test Gemfile path
	testGemfile := filepath.Join("testdata", "nested_blocks_gemfile")

	// Parse the Gemfile
	parser := gemfile.NewGemfileParser(testGemfile)
	parsed, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing Gemfile: %v\n", err)
		os.Exit(1)
	}

	// Display gems with their sources
	fmt.Println("=== NESTED BLOCKS TEST ===")
	for _, dep := range parsed.Dependencies {
		fmt.Printf("%-20s", dep.Name)
		if dep.Source != nil {
			fmt.Printf(" -> Source: %s", dep.Source.URL)
		} else {
			fmt.Printf(" -> Source: (default)")
		}
		if len(dep.Groups) > 0 && !(len(dep.Groups) == 1 && dep.Groups[0] == "default") {
			fmt.Printf(", Groups: %v", dep.Groups)
		}
		fmt.Println()
	}

	// Expected:
	// - rake: (default)
	// - minitest: gem.coop
	// - debug: gem.coop (still in source block), Groups: [development]
	// - rspec: gem.coop
	// - rack: (default)
}
