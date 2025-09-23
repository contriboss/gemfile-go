// Parse a Gemfile.lock and display fun statistics!
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
)

func main() {
	fmt.Println("🔍 Gemfile.lock Parser Example")
	fmt.Println("=" + strings.Repeat("=", 40))

	// You can pass a Gemfile.lock path as an argument
	path := "Gemfile.lock"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	// Parse the lockfile
	lock, err := lockfile.ParseFile(path)
	if err != nil {
		log.Fatalf("❌ Oops! Couldn't parse %s: %v", path, err)
	}

	fmt.Printf("\n📊 Gemfile.lock Statistics:\n")
	fmt.Printf("├─ Total gems: %d\n", len(lock.GemSpecs))
	fmt.Printf("├─ Git gems: %d\n", len(lock.GitSpecs))
	fmt.Printf("├─ Path gems: %d\n", len(lock.PathSpecs))
	fmt.Printf("├─ Platforms: %v\n", lock.Platforms)
	fmt.Printf("└─ Bundled with: %s\n", lock.BundledWith)

	// Find the most popular dependencies
	depCount := make(map[string]int)
	for _, gem := range lock.GemSpecs {
		for _, dep := range gem.Dependencies {
			depCount[dep.Name]++
		}
	}

	fmt.Printf("\n🏆 Top 5 Most Depended Upon Gems:\n")
	// Simple top 5 (in production, you'd sort properly)
	count := 0
	for name, uses := range depCount {
		if count >= 5 {
			break
		}
		fmt.Printf("   %d. %s (used by %d gems)\n", count+1, name, uses)
		count++
	}

	// Fun facts!
	fmt.Printf("\n🎉 Fun Facts:\n")

	// Count Rails gems
	railsGems := 0
	for _, gem := range lock.GemSpecs {
		if strings.Contains(gem.Name, "rails") || strings.HasPrefix(gem.Name, "action") || strings.HasPrefix(gem.Name, "active") {
			railsGems++
		}
	}
	if railsGems > 0 {
		fmt.Printf("   🚂 Found %d Rails-related gems\n", railsGems)
	}

	// Check for test frameworks
	testFrameworks := []string{}
	for _, gem := range lock.GemSpecs {
		switch gem.Name {
		case "rspec", "rspec-core":
			testFrameworks = append(testFrameworks, "RSpec")
		case "minitest":
			testFrameworks = append(testFrameworks, "Minitest")
		case "test-unit":
			testFrameworks = append(testFrameworks, "Test::Unit")
		}
	}
	if len(testFrameworks) > 0 {
		fmt.Printf("   🧪 Testing with: %s\n", strings.Join(testFrameworks, ", "))
	}

	// Check for web servers
	for _, gem := range lock.GemSpecs {
		switch gem.Name {
		case "puma":
			fmt.Println("   🐆 Puma server detected - Fast & concurrent!")
		case "unicorn":
			fmt.Println("   🦄 Unicorn server detected - Battle-tested!")
		case "thin":
			fmt.Println("   💨 Thin server detected - Lightweight!")
		case "passenger":
			fmt.Println("   🚊 Passenger detected - Enterprise ready!")
		}
	}

	// Security check
	gemsWithChecksum := 0
	for _, gem := range lock.GemSpecs {
		if gem.Checksum != "" {
			gemsWithChecksum++
		}
	}
	fmt.Printf("   🔒 %d/%d gems have security checksums\n", gemsWithChecksum, len(lock.GemSpecs))

	// Platform-specific gems
	platformGems := 0
	for _, gem := range lock.GemSpecs {
		if gem.Platform != "" && gem.Platform != "ruby" {
			platformGems++
		}
	}
	if platformGems > 0 {
		fmt.Printf("   💻 %d platform-specific gems found\n", platformGems)
	}

	fmt.Println("\n✅ Parsing complete! Happy coding! 🎊")
}