package lockfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParseLockfile benchmarks parsing a real-world Gemfile.lock
func BenchmarkParseLockfile(b *testing.B) {
	path := filepath.Join("..", "examples", "benchmark", "Gemfile.lock")

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		b.Skip("Benchmark Gemfile.lock not found, skipping benchmark")
	}

	// Read the file once to ensure it's cached
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Parse(bytes.NewReader(data))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseFile benchmarks parsing using the file path directly
func BenchmarkParseFile(b *testing.B) {
	path := filepath.Join("..", "examples", "benchmark", "Gemfile.lock")

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		b.Skip("Benchmark Gemfile.lock not found, skipping benchmark")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParseFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFindGem benchmarks finding a specific gem
func BenchmarkFindGem(b *testing.B) {
	path := filepath.Join("..", "examples", "benchmark", "Gemfile.lock")

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		b.Skip("Benchmark Gemfile.lock not found, skipping benchmark")
	}

	lock, err := ParseFile(path)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gem := lock.FindGem("railties")
		if gem == nil {
			b.Fatal("Expected to find railties gem")
		}
	}
}
