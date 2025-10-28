// Package gemfile provides tree-sitter based parsing for Ruby Gemfile files
package gemfile

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// TreeSitterGemfileParser handles parsing of Gemfile using tree-sitter
type TreeSitterGemfileParser struct {
	content      []byte
	helper       *RubyASTHelper
	contextStack *parserContextStack
}

// parserContext tracks the current parsing context (groups, platforms, sources, conditions)
type parserContext struct {
	groups      []string       // Current group(s) being parsed
	platforms   []string       // Current platform restrictions
	source      *Source        // Current source block
	conditional bool           // Whether we're inside a conditional
	parent      *parserContext // Parent context for nested blocks
}

// parserContextStack manages the context stack for nested blocks
type parserContextStack struct {
	current *parserContext
}

// newParserContextStack creates a new context stack with default context
func newParserContextStack() *parserContextStack {
	return &parserContextStack{
		current: &parserContext{
			groups: []string{"default"},
		},
	}
}

// push creates a new context by copying current and adding modifications
func (s *parserContextStack) push(modifyFn func(*parserContext)) {
	// Create new context by copying current
	newCtx := &parserContext{
		groups:      make([]string, len(s.current.groups)),
		platforms:   make([]string, len(s.current.platforms)),
		source:      s.current.source,
		conditional: s.current.conditional,
		parent:      s.current,
	}
	copy(newCtx.groups, s.current.groups)
	copy(newCtx.platforms, s.current.platforms)

	// Apply modifications
	if modifyFn != nil {
		modifyFn(newCtx)
	}

	s.current = newCtx
}

// pop restores the parent context
func (s *parserContextStack) pop() {
	if s.current.parent != nil {
		s.current = s.current.parent
	}
}

// NewTreeSitterGemfileParser creates a new tree-sitter based Gemfile parser
func NewTreeSitterGemfileParser(content []byte) *TreeSitterGemfileParser {
	return &TreeSitterGemfileParser{
		content:      content,
		helper:       NewRubyASTHelper(content),
		contextStack: newParserContextStack(),
	}
}

// ParseWithTreeSitter parses a Gemfile using tree-sitter and returns structured data
func (p *TreeSitterGemfileParser) ParseWithTreeSitter() (*ParsedGemfile, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(rubyLanguage); err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}

	tree := parser.Parse(p.content, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse Gemfile")
	}
	defer tree.Close()

	root := tree.RootNode()

	gemfile := &ParsedGemfile{
		Sources:      []Source{},
		Dependencies: []GemDependency{},
		Gemspecs:     []GemspecReference{},
	}

	// Walk the AST and extract Gemfile data
	p.extractGemfileData(root, gemfile)

	return gemfile, nil
}

// extractGemfileData walks the AST to extract Gemfile data
func (p *TreeSitterGemfileParser) extractGemfileData(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	if node == nil {
		return
	}

	kind := node.Kind()

	// Process different node types
	switch kind {
	case nodeCall, nodeMethodCall:
		// processCall will handle traversal for special methods (gem, group, etc.)
		// and return true if it handled the node completely
		p.processCall(node, gemfile)

	case nodeIdentifier:
		// Handle bare identifiers like "gemspec" (no arguments, no parentheses)
		identName := p.helper.GetNodeText(node)
		if identName == gemspecDirective {
			p.processGemspec(node, gemfile)
		}
		// Still traverse children
		for i := uint(0); i < node.ChildCount(); i++ {
			p.extractGemfileData(node.Child(i), gemfile)
		}

	case nodeIf, nodeUnless:
		p.processConditional(node, gemfile)

	default:
		// Recursively process children for all other node types
		for i := uint(0); i < node.ChildCount(); i++ {
			p.extractGemfileData(node.Child(i), gemfile)
		}
	}
}

// processCall processes method call nodes (gem, group, source, platforms, etc.)
func (p *TreeSitterGemfileParser) processCall(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	// Find the method name
	methodName := p.extractMethodName(node)

	switch methodName {
	case "gem":
		p.processGem(node, gemfile)
	case groupMethod:
		p.processGroup(node, gemfile)
	case platformsMethod, platformMethod:
		p.processPlatform(node, gemfile)
	case "source":
		p.processSource(node, gemfile)
	case "ruby":
		p.processRubyVersion(node, gemfile)
	case gemspecDirective:
		p.processGemspec(node, gemfile)
	case "git_source":
		// Skip git_source definitions for now
	default:
		// For unknown methods, still traverse children
		for i := uint(0); i < node.ChildCount(); i++ {
			p.extractGemfileData(node.Child(i), gemfile)
		}
	}
}

// processGem processes a gem declaration
func (p *TreeSitterGemfileParser) processGem(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	args := p.extractArguments(node)
	if len(args) == 0 {
		return
	}

	dep := GemDependency{
		Name:      args[0],
		Groups:    make([]string, len(p.contextStack.current.groups)),
		Platforms: make([]string, len(p.contextStack.current.platforms)),
		Source:    p.contextStack.current.source,
	}
	copy(dep.Groups, p.contextStack.current.groups)
	copy(dep.Platforms, p.contextStack.current.platforms)

	// Extract version constraints (strings after the gem name)
	for i := 1; i < len(args); i++ {
		// Skip if it looks like an option hash
		if !strings.Contains(args[i], ":") {
			dep.Constraints = append(dep.Constraints, args[i])
		}
	}

	// Extract hash options (require, platforms, groups, git, path, etc.)
	p.extractGemOptions(node, &dep)

	gemfile.Dependencies = append(gemfile.Dependencies, dep)
}

// processGroup processes a group block
func (p *TreeSitterGemfileParser) processGroup(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	// Extract group names from arguments
	groupNames := p.extractSymbolArguments(node)
	if len(groupNames) == 0 {
		return
	}

	// Find the block
	block := p.helper.FindChildByKind(node, nodeDoBlock)
	if block == nil {
		block = p.helper.FindChildByKind(node, nodeBlock)
	}

	if block != nil {
		// Push new context with these groups
		p.contextStack.push(func(ctx *parserContext) {
			ctx.groups = groupNames
		})

		// Process block body
		p.extractGemfileData(block, gemfile)

		// Pop context when done
		p.contextStack.pop()
	}
}

// processPlatform processes a platforms/platform block
func (p *TreeSitterGemfileParser) processPlatform(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	// Extract platform names from arguments
	platformNames := p.extractSymbolArguments(node)
	if len(platformNames) == 0 {
		return
	}

	// Find the block
	block := p.helper.FindChildByKind(node, nodeDoBlock)
	if block == nil {
		block = p.helper.FindChildByKind(node, nodeBlock)
	}

	if block != nil {
		// Push new context with these platforms
		p.contextStack.push(func(ctx *parserContext) {
			ctx.platforms = platformNames
		})

		// Process block body
		p.extractGemfileData(block, gemfile)

		// Pop context when done
		p.contextStack.pop()
	}
}

// processSource processes a source declaration or source block
func (p *TreeSitterGemfileParser) processSource(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	args := p.extractArguments(node)
	if len(args) == 0 {
		return
	}

	sourceURL := args[0]
	source := Source{
		Type: "rubygems",
		URL:  sourceURL,
	}

	// Check if there's a block
	block := p.helper.FindChildByKind(node, nodeDoBlock)
	if block == nil {
		block = p.helper.FindChildByKind(node, nodeBlock)
	}

	if block != nil {
		// Source block - add to sources and push context
		gemfile.Sources = append(gemfile.Sources, source)

		p.contextStack.push(func(ctx *parserContext) {
			ctx.source = &source
		})

		// Process block body
		p.extractGemfileData(block, gemfile)

		// Pop context when done
		p.contextStack.pop()
	} else {
		// Global source declaration
		gemfile.Sources = append(gemfile.Sources, source)
	}
}

// processRubyVersion processes a ruby version declaration
func (p *TreeSitterGemfileParser) processRubyVersion(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	args := p.extractArguments(node)
	if len(args) > 0 {
		gemfile.RubyVersion = args[0]
	}
}

// processGemspec processes a gemspec directive
func (p *TreeSitterGemfileParser) processGemspec(_ *tree_sitter.Node, gemfile *ParsedGemfile) {
	ref := GemspecReference{
		DevelopmentGroup: developmentGroup, // Default to development group
	}

	// Extract hash options
	// TODO: Extract path, name, development_group options from hash argument

	gemfile.Gemspecs = append(gemfile.Gemspecs, ref)
}

// processConditional processes if/unless blocks
func (p *TreeSitterGemfileParser) processConditional(node *tree_sitter.Node, gemfile *ParsedGemfile) {
	// Mark gems inside conditionals as conditional
	p.contextStack.push(func(ctx *parserContext) {
		ctx.conditional = true
	})

	// Process the consequence/then branch
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		// Look for then/body nodes
		if child.Kind() == "then" || child.Kind() == nodeBodyStatement {
			p.extractGemfileData(child, gemfile)
		}
	}

	p.contextStack.pop()
}

// extractMethodName extracts the method name from a call node
func (p *TreeSitterGemfileParser) extractMethodName(node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}

	// Look for identifier child
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeIdentifier {
			return p.helper.GetNodeText(child)
		}
	}

	return ""
}

// extractArguments extracts string arguments from a call node
func (p *TreeSitterGemfileParser) extractArguments(node *tree_sitter.Node) []string {
	var args []string

	// Find argument_list child
	argList := p.helper.FindChildByKind(node, nodeArgumentList)
	if argList == nil {
		return args
	}

	// Extract string arguments
	for i := uint(0); i < argList.ChildCount(); i++ {
		child := argList.Child(i)
		if child.Kind() == nodeString {
			value := p.helper.ExtractStringValue(child)
			args = append(args, value)
		}
	}

	return args
}

// extractSymbolArguments extracts symbol arguments (for groups, platforms)
func (p *TreeSitterGemfileParser) extractSymbolArguments(node *tree_sitter.Node) []string {
	var symbols []string

	// Find argument_list child
	argList := p.helper.FindChildByKind(node, nodeArgumentList)
	if argList == nil {
		return symbols
	}

	// Extract symbol arguments
	for i := uint(0); i < argList.ChildCount(); i++ {
		child := argList.Child(i)
		kind := child.Kind()
		if kind == nodeSymbol || kind == nodeSimpleSymbol {
			value := p.helper.ExtractSymbolValue(child)
			if value != "" {
				symbols = append(symbols, value)
			}
		}
	}

	return symbols
}

// extractGemOptions extracts hash options from a gem declaration
func (p *TreeSitterGemfileParser) extractGemOptions(node *tree_sitter.Node, dep *GemDependency) {
	// Find argument_list
	argList := p.helper.FindChildByKind(node, nodeArgumentList)
	if argList == nil {
		return
	}

	// Look for pair nodes directly in argument_list (Ruby 2.x+ style) or hash node (older style)
	for i := uint(0); i < argList.ChildCount(); i++ {
		child := argList.Child(i)
		switch child.Kind() {
		case nodePair:
			p.extractPairOption(child, dep)
		case "hash":
			p.extractHashOptions(child, dep)
		}
	}
}

// extractPairOption extracts a single key-value pair option
func (p *TreeSitterGemfileParser) extractPairOption(pair *tree_sitter.Node, dep *GemDependency) {
	var key, value string
	var arrayValues []string
	hasArray := false

	// Extract key and value from pair node
	for j := uint(0); j < pair.ChildCount(); j++ {
		child := pair.Child(j)
		kind := child.Kind()

		switch kind {
		case nodeHashKeySymbol:
			// Extract the symbol name without colon
			key = p.helper.GetNodeText(child)
		case nodeSymbol, nodeSimpleSymbol:
			symbolValue := p.helper.ExtractSymbolValue(child)
			if key == "" {
				key = symbolValue
			} else {
				value = symbolValue
			}
		case nodeString:
			value = p.helper.ExtractStringValue(child)
		case falseValue, trueValue:
			value = p.helper.GetNodeText(child)
		case nodeArray:
			// Handle array values (for platforms, groups)
			arrayValues = p.extractArraySymbols(child)
			hasArray = true
		}
	}

	// Handle array values
	if hasArray {
		switch key {
		case platformsMethod, platformMethod:
			dep.Platforms = arrayValues
		case groupsKey, groupMethod:
			dep.Groups = arrayValues
		}
		return
	}

	// Apply scalar options
	p.applyGemOption(key, value, dep)
}

// extractHashOptions extracts options from a hash node
func (p *TreeSitterGemfileParser) extractHashOptions(hashNode *tree_sitter.Node, dep *GemDependency) {
	for i := uint(0); i < hashNode.ChildCount(); i++ {
		pair := hashNode.Child(i)
		if pair.Kind() == nodePair {
			p.extractPairOption(pair, dep)
		}
	}
}

// applyGemOption applies a single gem option
//
//nolint:gocyclo // Switch statement with many gem options is acceptable
func (p *TreeSitterGemfileParser) applyGemOption(key, value string, dep *GemDependency) {
	switch key {
	case "require":
		if value == falseValue {
			emptyStr := ""
			dep.Require = &emptyStr
		} else if value != "" {
			dep.Require = &value
		}
	case platformsMethod, platformMethod:
		if value != "" {
			dep.Platforms = []string{value}
		}
	case groupsKey, groupMethod:
		if value != "" {
			dep.Groups = []string{value}
		}
	case gitKey, githubKey:
		// Always create a new source for explicit git/github options
		dep.Source = &Source{Type: gitKey}
		if key == githubKey {
			dep.Source.URL = fmt.Sprintf("https://github.com/%s.git", value)
		} else {
			dep.Source.URL = value
		}
	case "path":
		// Always create a new source for explicit path options
		dep.Source = &Source{Type: "path"}
		dep.Source.URL = value
	case "branch":
		// Create new git source if nil or not git (to avoid mutating context source)
		if dep.Source == nil || dep.Source.Type != gitKey {
			dep.Source = &Source{Type: gitKey}
		}
		dep.Source.Branch = value
	case "tag":
		// Create new git source if nil or not git (to avoid mutating context source)
		if dep.Source == nil || dep.Source.Type != gitKey {
			dep.Source = &Source{Type: gitKey}
		}
		dep.Source.Tag = value
	case "ref":
		// Create new git source if nil or not git (to avoid mutating context source)
		if dep.Source == nil || dep.Source.Type != gitKey {
			dep.Source = &Source{Type: gitKey}
		}
		dep.Source.Ref = value
	}
}

// extractArraySymbols extracts symbol values from an array node
func (p *TreeSitterGemfileParser) extractArraySymbols(arrayNode *tree_sitter.Node) []string {
	var symbols []string

	for i := uint(0); i < arrayNode.ChildCount(); i++ {
		child := arrayNode.Child(i)
		kind := child.Kind()
		if kind == nodeSymbol || kind == nodeSimpleSymbol {
			value := p.helper.ExtractSymbolValue(child)
			if value != "" {
				symbols = append(symbols, value)
			}
		}
	}

	return symbols
}
