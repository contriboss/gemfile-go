// Package gemfile provides shared tree-sitter utilities for parsing Ruby files
package gemfile

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
)

var (
	// rubyLanguage caches the tree-sitter language instance so we only create it once.
	rubyLanguage = tree_sitter.NewLanguage(tree_sitter_ruby.Language())
)

// Tree-sitter node type constants for Ruby AST
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
)

// Ruby keyword and method name constants
const (
	gemspecDirective = "gemspec"
	groupMethod      = "group"
	platformMethod   = "platform"
	platformsMethod  = "platforms"
	gitKey           = "git"
	githubKey        = "github"
	groupsKey        = "groups"
	sourceKey        = "source"
	trueValue        = "true"
	falseValue       = "false"
)

// RubyASTHelper provides common tree-sitter helper methods
type RubyASTHelper struct {
	content []byte
}

// NewRubyASTHelper creates a new helper with the given source content
func NewRubyASTHelper(content []byte) *RubyASTHelper {
	return &RubyASTHelper{content: content}
}

// GetNodeText returns the text content of a node
func (h *RubyASTHelper) GetNodeText(node *tree_sitter.Node) string {
	return node.Utf8Text(h.content)
}

// ExtractStringValue extracts the string value from a string node
func (h *RubyASTHelper) ExtractStringValue(node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}

	// For string nodes, look for string_content child
	if node.Kind() == nodeString {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == nodeStringContent {
				return h.GetNodeText(child)
			}
		}
		// Fallback to full text and strip quotes
		text := h.GetNodeText(node)
		if len(text) >= 2 {
			return text[1 : len(text)-1]
		}
	}

	return h.GetNodeText(node)
}

// ExtractSymbolValue extracts the symbol value (without the leading colon)
func (h *RubyASTHelper) ExtractSymbolValue(node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}

	kind := node.Kind()
	if kind != nodeSymbol && kind != nodeSimpleSymbol {
		return ""
	}

	text := h.GetNodeText(node)
	if text != "" && text[0] == ':' {
		return text[1:]
	}
	return text
}

// ExtractIdentifier extracts an identifier name from a node
func (h *RubyASTHelper) ExtractIdentifier(node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}

	if node.Kind() == nodeIdentifier || node.Kind() == nodeConstant {
		return h.GetNodeText(node)
	}

	return ""
}

// FindChildByKind finds the first child node with the specified kind
func (h *RubyASTHelper) FindChildByKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	if node == nil {
		return nil
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == kind {
			return child
		}
	}

	return nil
}

// FindChildrenByKind finds all child nodes with the specified kind
func (h *RubyASTHelper) FindChildrenByKind(node *tree_sitter.Node, kind string) []*tree_sitter.Node {
	if node == nil {
		return nil
	}

	var children []*tree_sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == kind {
			children = append(children, child)
		}
	}

	return children
}

// WalkAST walks the AST and calls the visitor function for each node
func (h *RubyASTHelper) WalkAST(node *tree_sitter.Node, visitor func(*tree_sitter.Node) bool) {
	if node == nil {
		return
	}

	// Call visitor, if it returns false, stop walking
	if !visitor(node) {
		return
	}

	// Recursively walk children
	for i := uint(0); i < node.ChildCount(); i++ {
		h.WalkAST(node.Child(i), visitor)
	}
}

// IsBlockNode checks if a node is a block node (do_block or block)
func (h *RubyASTHelper) IsBlockNode(node *tree_sitter.Node) bool {
	if node == nil {
		return false
	}
	kind := node.Kind()
	return kind == nodeDoBlock || kind == nodeBlock
}

// ExtractBlockParameter extracts the parameter name from a block
// e.g., in "do |spec|", returns "spec"
func (h *RubyASTHelper) ExtractBlockParameter(node *tree_sitter.Node) string {
	if node == nil || !h.IsBlockNode(node) {
		return ""
	}

	// Look for block_parameters node
	blockParams := h.FindChildByKind(node, "block_parameters")
	if blockParams == nil {
		return ""
	}

	// Find the identifier inside
	identifier := h.FindChildByKind(blockParams, nodeIdentifier)
	if identifier != nil {
		return h.GetNodeText(identifier)
	}

	return ""
}
