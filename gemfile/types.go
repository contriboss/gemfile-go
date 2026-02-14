package gemfile

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

// GemspecReference represents a gemspec directive in the Gemfile.
// Ruby equivalent: gemspec path: "path", name: "name", development_group: :group
type GemspecReference struct {
	Path             string // Path to search for gemspec files (defaults to ".")
	Name             string // Specific gemspec name to load (optional)
	DevelopmentGroup string // Group for development dependencies (defaults to "development")
	Glob             string // Glob pattern for finding gemspec files (defaults to "{,*,*/*}.gemspec")
}

// ParsedGemfile represents the parsed Gemfile content.
type ParsedGemfile struct {
	Dependencies []GemDependency    // Declared gems
	Sources      []Source           // Gem sources
	RubyVersion  string             // Ruby version requirement
	GitSources   map[string]string  // Gem name to git URL mapping
	Gemspecs     []GemspecReference // Gemspec references
}

const (
	rubygemsSource     = "rubygems"
	pathSource         = "path"
	developmentGroup   = "development"
	defaultGlobPattern = "{,*,*/*}.gemspec"
	endKeyword         = "end"
	falseValue         = "false"
)
