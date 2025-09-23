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

	// Parse the lockfile
	lock := parseLockfileFromArgs()

	// Display statistics and analysis
	displayStatistics(lock)
	analyzePopularDependencies(lock)
	displayFunFacts(lock)

	fmt.Println("\n✅ Parsing complete! Happy coding! 🎊")
}

// parseLockfileFromArgs parses lockfile from command line arguments or default path
func parseLockfileFromArgs() *lockfile.Lockfile {
	path := "testdata/Gemfile.lock"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	// Parse the lockfile
	lock, err := lockfile.ParseFile(path)
	if err != nil {
		log.Fatalf("❌ Oops! Couldn't parse %s: %v", path, err)
	}
	return lock
}

// displayStatistics shows basic lockfile statistics
func displayStatistics(lock *lockfile.Lockfile) {
	fmt.Printf("\n📊 Gemfile.lock Statistics:\n")
	fmt.Printf("├─ Total gems: %d\n", len(lock.GemSpecs))
	fmt.Printf("├─ Git gems: %d\n", len(lock.GitSpecs))
	fmt.Printf("├─ Path gems: %d\n", len(lock.PathSpecs))
	fmt.Printf("├─ Platforms: %v\n", lock.Platforms)
	fmt.Printf("└─ Bundled with: %s\n", lock.BundledWith)
}

// analyzePopularDependencies finds and displays most popular dependencies
func analyzePopularDependencies(lock *lockfile.Lockfile) {
	depCount := make(map[string]int)
	for i := range lock.GemSpecs {
		gem := &lock.GemSpecs[i]
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
}

// displayFunFacts shows interesting facts about the lockfile
func displayFunFacts(lock *lockfile.Lockfile) {
	fmt.Printf("\n🎉 Fun Facts:\n")

	displayRailsGems(lock)
	displayTestFrameworks(lock)
	displayWebServers(lock)
	displaySecurityInfo(lock)
	displayPlatformGems(lock)
}

// displayRailsGems counts and displays Rails-related gems
func displayRailsGems(lock *lockfile.Lockfile) {
	railsGems := 0
	for i := range lock.GemSpecs {
		gem := &lock.GemSpecs[i]
		if strings.Contains(gem.Name, "rails") || strings.HasPrefix(gem.Name, "action") || strings.HasPrefix(gem.Name, "active") {
			railsGems++
		}
	}
	if railsGems > 0 {
		fmt.Printf("   🚂 Found %d Rails-related gems\n", railsGems)
	}
}

// displayTestFrameworks identifies and displays test frameworks
func displayTestFrameworks(lock *lockfile.Lockfile) {
	testFrameworks := []string{}
	for i := range lock.GemSpecs {
		gem := &lock.GemSpecs[i]
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
}

// displayWebServers identifies and displays web servers
func displayWebServers(lock *lockfile.Lockfile) {
	for i := range lock.GemSpecs {
		gem := &lock.GemSpecs[i]
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
}

// displaySecurityInfo shows security-related information
func displaySecurityInfo(lock *lockfile.Lockfile) {
	gemsWithChecksum := 0
	for i := range lock.GemSpecs {
		gem := &lock.GemSpecs[i]
		if gem.Checksum != "" {
			gemsWithChecksum++
		}
	}
	fmt.Printf("   🔒 %d/%d gems have security checksums\n", gemsWithChecksum, len(lock.GemSpecs))
}

// displayPlatformGems shows platform-specific gem information
func displayPlatformGems(lock *lockfile.Lockfile) {
	platformGems := 0
	for i := range lock.GemSpecs {
		gem := &lock.GemSpecs[i]
		if gem.Platform != "" && gem.Platform != "ruby" {
			platformGems++
		}
	}
	if platformGems > 0 {
		fmt.Printf("   💻 %d platform-specific gems found\n", platformGems)
	}
}
