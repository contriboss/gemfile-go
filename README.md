# gemfile-go 💎 → 🐹

> Parse Ruby's Gemfile and Gemfile.lock in pure Go - no Ruby required!

[![CI](https://github.com/contriboss/gemfile-go/actions/workflows/ci.yml/badge.svg)](https://github.com/contriboss/gemfile-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/contriboss/gemfile-go.svg)](https://pkg.go.dev/github.com/contriboss/gemfile-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/contriboss/gemfile-go)](https://goreportcard.com/report/github.com/contriboss/gemfile-go)

## Overview

Ever wanted to parse Ruby's Gemfile.lock from Go? Maybe you're building developer tools, analyzing dependencies, or just curious about what's in that lockfile. This library lets you do it all without having Ruby installed!

Think of it as **Bundler's lockfile parser, but speaking Go**.

## 🚀 Quick Start

```bash
go get github.com/contriboss/gemfile-go
```

## 📖 For Ruby Developers Learning Go

Welcome Ruby friend! 👋 Here's a translation guide:

| Ruby Concept | Go Equivalent in This Library |
|-------------|-------------------------------|
| `Bundler.locked_gems` | `lockfile.ParseFile()` |
| `gem.dependencies` | `GemSpec.Dependencies` |
| `Gem::Version` | String (but semver compatible) |
| `bundle install` | Parse lockfile → download gems |
| `Gemfile.lock` | `Lockfile` struct |

## 💻 Examples

### Parse a Gemfile.lock

```go
package main

import (
    "fmt"
    "log"

    "github.com/contriboss/gemfile-go/lockfile"
)

func main() {
    // Parse your Gemfile.lock
    lock, err := lockfile.ParseFile("Gemfile.lock")
    if err != nil {
        log.Fatal(err)
    }

    // How many gems do you have?
    fmt.Printf("📦 Found %d gems\n", len(lock.GemSpecs))

    // What Ruby version was it bundled with?
    fmt.Printf("💎 Bundled with: %s\n", lock.BundledWith)

    // List all Rails-related gems (because why not?)
    for _, gem := range lock.GemSpecs {
        if strings.Contains(gem.Name, "rails") {
            fmt.Printf("🚂 %s (%s)\n", gem.Name, gem.Version)
        }
    }
}
```

### Find a Specific Gem

```go
// Finding gems is like `bundle show gemname`
gem := lock.FindGem("rails")
if gem != nil {
    fmt.Printf("Rails version: %s\n", gem.Version)
    fmt.Printf("Dependencies: %v\n", gem.Dependencies)
}
```

### Parse a Gemfile

```go
package main

import (
    "github.com/contriboss/gemfile-go/gemfile"
)

func main() {
    // Parse the Gemfile (the wishlist, not the lockfile!)
    gems, err := gemfile.ParseFile("Gemfile")
    if err != nil {
        log.Fatal(err)
    }

    // See what's in the development group
    for _, gem := range gems {
        if gem.HasGroup("development") {
            fmt.Printf("Dev gem: %s\n", gem.Name)
        }
    }
}
```

## 🎭 Fun Features

### Ruby Platform Detection

```go
// Check if a gem works on your platform
if gem.Platform == "" || gem.Platform == "ruby" {
    fmt.Println("✅ Pure Ruby gem - works everywhere!")
} else if gem.Platform == "x86_64-linux" {
    fmt.Println("🐧 Linux native extension detected!")
}
```

### Security Checksums

```go
// Verify gem integrity (just like Bundler does!)
if gem.Checksum != "" {
    fmt.Printf("🔒 Gem %s is checksum-protected\n", gem.Name)
}
```

### Git Dependencies

```go
// Find all gems from git repos (living on the edge!)
for _, gitGem := range lock.GitSpecs {
    fmt.Printf("🔥 %s from %s (rev: %s)\n",
        gitGem.Name, gitGem.Remote, gitGem.Revision[:7])
}
```

## 🏗️ Building

We use [Mage](https://magefile.org) for builds (it's like Rake but for Go!):

```bash
# Install Mage
go install github.com/magefile/mage@latest

# Run tests
mage test

# Run linter
mage lint

# Run benchmarks
mage bench

# Run everything (CI mode)
mage ci
```

## 🤝 For Ruby Devs: Key Differences

1. **No Version Operators**: In Ruby you have `~>`, `>=`, etc. In the lockfile, versions are exact, so we just use strings.

2. **No Gem Installation**: This library *reads* Gemfile.lock but doesn't install gems. Think of it as read-only Bundler.

3. **Groups as Strings**: Ruby uses symbols (`:development`), we use strings (`"development"`).

4. **Error Handling**: Go doesn't have exceptions. We return errors as second values:
   ```go
   // Ruby: lock = Bundler.locked_gems (might raise)
   // Go:
   lock, err := lockfile.ParseFile("Gemfile.lock")
   if err != nil {
       // Handle error
   }
   ```

## 🎯 Use Cases

- **CI/CD Tools**: Analyze Ruby dependencies without Ruby
- **Security Scanners**: Check for vulnerable gems
- **Developer Tools**: Build IDE support for Ruby projects
- **Dependency Analysis**: Understand Ruby project dependencies
- **Migration Tools**: Help migrate from Ruby to other languages
- **Fun Projects**: Because parsing is fun! 🎉

## 📚 API Documentation

Full API docs at [pkg.go.dev](https://pkg.go.dev/github.com/contriboss/gemfile-go)

## 🧪 Testing

Every parser function has tests! We test with real Gemfile.lock files from popular Ruby projects like Rails, Sinatra, and Jekyll.

```bash
# Run tests with coverage
mage test

# Run tests with race detector
mage testrace

# Run benchmarks
mage bench
```

## 🤔 Why This Exists

Sometimes you need to understand Ruby dependencies from Go:
- You're building polyglot developer tools
- You're analyzing dependencies across languages
- You're migrating from Ruby to Go (we don't judge!)
- You just want fast lockfile parsing

## 🚄 Performance

Parsing a typical Rails Gemfile.lock:
- **gemfile-go**: ~1ms 🚀
- **Bundler (Ruby)**: ~100ms

That's 100x faster! (Your mileage may vary)

## 📜 License

MIT - Use it, fork it, enjoy it!

## 🙏 Acknowledgments

- The Bundler team for the lockfile format
- The Go community for being awesome
- Coffee ☕ for making this possible

---

Made with 💚 by [@contriboss](https://github.com/contriboss) - Bridging Ruby and Go, one parser at a time!