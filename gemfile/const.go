package gemfile

// Source types and Gemfile DSL string values.
const (
	rubygemsSource     = "rubygems"
	pathSource         = "path"
	developmentGroup   = "development"
	defaultGlobPattern = "{,*,*/*}.gemspec"
	falseValue         = "false"
	trueValue          = "true"
	defaultGroup       = "default"
	rubygemsURL        = "https://rubygems.org"
)

// Ruby keyword and method name constants.
const (
	gemspecDirective = "gemspec"
	groupMethod      = "group"
	platformMethod   = "platform"
	platformsMethod  = "platforms"
	gitKey           = "git"
	githubKey        = "github"
	groupsKey        = "groups"
	sourceKey        = "source"
	envConstant      = "ENV"
	gitSource        = "git"
)

// Tree-sitter node type constants for Ruby AST.
const (
	nodeCall             = "call"
	nodeBlock            = "block"
	nodeDoBlock          = "do_block"
	nodeScopeResolution  = "scope_resolution"
	nodeIdentifier       = "identifier"
	nodeElementReference = "element_reference"
	nodeArray            = "array"
	nodeString           = "string"
	nodeStringContent    = "string_content"
	nodeConstant         = "constant"
	nodeSymbol           = "symbol"
	nodeSimpleSymbol     = "simple_symbol"
	nodeInteger          = "integer"
	nodeBodyStatement    = "body_statement"
	nodeAssignment       = "assignment"
	nodeArgumentList     = "argument_list"
	nodeMethod           = "method"
	nodeIf               = "if"
	nodeUnless           = "unless"
	nodeMethodCall       = "method_call"
	nodePair             = "pair"
	nodeHashKeySymbol    = "hash_key_symbol"
	endKeyword           = "end"
)

// Gemspec attribute keys.
const (
	gemspecNameKey                = "name"
	gemspecVersionKey             = "version"
	gemspecSummaryKey             = "summary"
	gemspecDescriptionKey         = "description"
	gemspecHomepageKey            = "homepage"
	gemspecLicenseKey             = "license"
	gemspecRequiredRubyVersionKey = "required_ruby_version"
)
