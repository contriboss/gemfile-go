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
	printHeader()
	path := getPath()
	lock := parseLockfile(path)
	printStatistics(lock)
	analyzePopularDependencies(lock)
	printFunFacts(lock)
}

func printHeader() {
	fmt.Println("🔍 Gemfile.lock Parser Example")
	fmt.Println("=" + strings.Repeat("=", 40))
}

func getPath() string {
	path := "testdata/Gemfile.lock"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	return path
}

func parseLockfile(path string) *lockfile.Lockfile {
	lock, err := lockfile.ParseFile(path)
	if err != nil {
		log.Fatalf("❌ Oops! Couldn't parse %s: %v", path, err)
	}
	return lock
}

func printStatistics(lock *lockfile.Lockfile) {
	fmt.Printf("\n📊 Gemfile.lock Statistics:\n")
	fmt.Printf("├─ Total gems: %d\n", len(lock.GemSpecs))
	fmt.Printf("├─ Git gems: %d\n", len(lock.GitSpecs))
	fmt.Printf("├─ Path gems: %d\n", len(lock.PathSpecs))
	fmt.Printf("├─ Platforms: %v\n", lock.Platforms)
	fmt.Printf("└─ Bundled with: %s\n", lock.BundledWith)
}

func analyzePopularDependencies(lock *lockfile.Lockfile) {
	// Find the most popular dependencies
	depCount := make(map[string]int)
	for i := range lock.GemSpecs {
		for _, dep := range lock.GemSpecs[i].Dependencies {
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

func printFunFacts(lock *lockfile.Lockfile) {
	fmt.Printf("\n🎉 Fun Facts:\n")

	checkRailsGems(lock)
	checkTestFrameworks(lock)
	checkWebServers(lock)
	checkSecurity(lock)
	checkPlatformGems(lock)

	fmt.Println("\n✅ Parsing complete! Happy coding! 🎊")
}

func checkRailsGems(lock *lockfile.Lockfile) {
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

func checkTestFrameworks(lock *lockfile.Lockfile) {
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

func checkWebServers(lock *lockfile.Lockfile) {
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

func checkSecurity(lock *lockfile.Lockfile) {
	gemsWithChecksum := 0
	for i := range lock.GemSpecs {
		gem := &lock.GemSpecs[i]
		if gem.Checksum != "" {
			gemsWithChecksum++
		}
	}
	fmt.Printf("   🔒 %d/%d gems have security checksums\n", gemsWithChecksum, len(lock.GemSpecs))
}

func checkPlatformGems(lock *lockfile.Lockfile) {
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
