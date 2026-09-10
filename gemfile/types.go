package gemfile

// GemfileParser parses Gemfile syntax into structured data.
// Ruby equivalent: Bundler::Dsl
type GemfileParser struct {
	filepath string
	content  string
}

// ParsedGemfile represents the parsed Gemfile content.
type ParsedGemfile struct {
	Dependencies []GemDependency    // Declared gems
	Sources      []Source           // Gem sources
	RubyVersion  string             // Ruby version requirement
	GitSources   map[string]string  // Gem name to git URL mapping
	Gemspecs     []GemspecReference // Gemspec references
}

// GemDependency represents a gem dependency.
// Ruby equivalent: gem "name", "version", options
type GemDependency struct {
	Name        string   // Gem name
	Constraints []string // Version constraints (e.g., "~> 2.0" means >= 2.0.0 and < 3.0.0)
	Source      *Source  // Git, path, source block URL, or nil for default source
	Groups      []string // Groups (empty means :default)
	Require     *string  // Require behavior (nil = normal, "false" = no auto-require)
	Platforms   []string // Platform restrictions (e.g., [:jruby, :windows_31])
	Comment     string   // Inline comment if present
}

// Source represents a gem source (RubyGems, Git, Path)
type Source struct {
	Type   string // "rubygems", "git", "path"
	URL    string
	Branch string // for git sources
	Tag    string // for git sources
	Ref    string // for git sources
}

// GemspecParser handles parsing of .gemspec files
type GemspecParser struct {
	filepath string
}

// GemspecReference represents a gemspec directive in the Gemfile.
// Ruby equivalent: gemspec path: "path", name: "name", development_group: :group
type GemspecReference struct {
	Path             string // Path to search for gemspec files (defaults to ".")
	Name             string // Specific gemspec name to load (optional)
	DevelopmentGroup string // Group for development dependencies (defaults to "development")
	Glob             string // Glob pattern for finding gemspec files (defaults to "{,*,*/*}.gemspec")
}

// GemspecFile represents a parsed .gemspec file
type GemspecFile struct {
	Name                    string            // Gem name from spec.name
	Version                 string            // Gem version from spec.version
	Summary                 string            // Gem summary
	Description             string            // Gem description
	Authors                 []string          // Gem authors
	Email                   []string          // Contact emails
	Homepage                string            // Project homepage
	License                 string            // License identifier
	RuntimeDependencies     []GemDependency   // Runtime dependencies from add_runtime_dependency
	DevelopmentDependencies []GemDependency   // Development dependencies from add_development_dependency
	RequiredRubyVersion     string            // Required Ruby version
	Files                   []string          // Files included in the gem
	Metadata                map[string]string // Additional metadata
	PostInstallMessage      string            // Post-install message
}
