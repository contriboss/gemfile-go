// Package gemfile provides tree-sitter based parsing for Ruby .gemspec files
package gemfile

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// TreeSitterGemspecParser handles parsing of .gemspec files using tree-sitter
type TreeSitterGemspecParser struct {
	content   []byte
	helper    *RubyASTHelper
	variables map[string]string // Track variable assignments
}

// NewTreeSitterGemspecParser creates a new tree-sitter based gemspec parser
func NewTreeSitterGemspecParser(content []byte) *TreeSitterGemspecParser {
	return &TreeSitterGemspecParser{
		content:   content,
		helper:    NewRubyASTHelper(content),
		variables: make(map[string]string),
	}
}

// ParseWithTreeSitter parses a .gemspec file using tree-sitter and returns structured data
func (p *TreeSitterGemspecParser) ParseWithTreeSitter() (*GemspecFile, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(rubyLanguage); err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}

	tree := parser.Parse(p.content, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse gemspec")
	}
	defer tree.Close()

	root := tree.RootNode()

	gemspec := &GemspecFile{
		Authors:                 []string{},
		Email:                   []string{},
		RuntimeDependencies:     []GemDependency{},
		DevelopmentDependencies: []GemDependency{},
		Files:                   []string{},
		Metadata:                make(map[string]string),
	}

	// Extract data from the AST
	p.extractGemspecData(root, gemspec)

	return gemspec, nil
}

// extractGemspecData walks the AST to extract gemspec data
func (p *TreeSitterGemspecParser) extractGemspecData(node *tree_sitter.Node, gemspec *GemspecFile) {
	// Parse variable assignments first (e.g., rails_version = '~> 8.1.0')
	if node.Kind() == nodeAssignment {
		p.processVariableAssignment(node)
	}

	// Look for Gem::Specification.new block
	if p.isGemSpecBlock(node) {
		// Process the block content
		p.processSpecBlock(node, gemspec)
		return
	}

	// Recursively process children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		p.extractGemspecData(child, gemspec)
	}
}

// isGemSpecBlock checks if the node is a Gem::Specification.new block
func (p *TreeSitterGemspecParser) isGemSpecBlock(node *tree_sitter.Node) bool {
	if node.Kind() != nodeCall {
		return false
	}

	if !p.nodeHasBlock(node) {
		return false
	}

	// Handle chained calls like Gem::Specification.new.tap { ... }
	if p.containsGemSpecConstructor(node) {
		return true
	}

	return false
}

// nodeHasBlock returns true when the call node has a block/do_block child.
func (p *TreeSitterGemspecParser) nodeHasBlock(node *tree_sitter.Node) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeBlock || child.Kind() == nodeDoBlock {
			return true
		}
	}
	return false
}

// containsGemSpecConstructor walks a call expression (including chained calls)
// and reports whether it ultimately invokes Gem::Specification.new.
func (p *TreeSitterGemspecParser) containsGemSpecConstructor(node *tree_sitter.Node) bool {
	if node.Kind() != nodeCall {
		return false
	}

	if p.isGemSpecConstructor(node) {
		return true
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeCall && p.containsGemSpecConstructor(child) {
			return true
		}
	}
	return false
}

// isGemSpecConstructor checks if a call node represents Gem::Specification.new.
func (p *TreeSitterGemspecParser) isGemSpecConstructor(node *tree_sitter.Node) bool {
	if node.Kind() != nodeCall {
		return false
	}

	var hasReceiver bool
	var hasNew bool

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case nodeScopeResolution:
			if strings.Contains(p.getNodeText(child), "Gem::Specification") {
				hasReceiver = true
			}
		case nodeIdentifier:
			if p.getNodeText(child) == "new" {
				hasNew = true
			}
		}
	}

	return hasReceiver && hasNew
}

// processSpecBlock processes the Gem::Specification block
func (p *TreeSitterGemspecParser) processSpecBlock(node *tree_sitter.Node, gemspec *GemspecFile) {
	// Find the block argument
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeBlock || child.Kind() == nodeDoBlock {
			p.processBlockBody(child, gemspec)
		}
	}
}

// processBlockBody processes the body of the specification block
func (p *TreeSitterGemspecParser) processBlockBody(node *tree_sitter.Node, gemspec *GemspecFile) {
	// Look for body_statement which contains the actual statements
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeBodyStatement {
			// Process all statements in the body_statement
			for j := uint(0); j < child.ChildCount(); j++ {
				stmt := child.Child(j)
				p.processStatement(stmt, gemspec)
			}
		} else {
			// Also try to process direct children (in case structure varies)
			p.processStatement(child, gemspec)
		}
	}
}

// processStatement processes individual statements in the block
func (p *TreeSitterGemspecParser) processStatement(node *tree_sitter.Node, gemspec *GemspecFile) {
	// Handle assignment statements like spec.name = "value" or rails_version = "~> 8.1.0"
	if node.Kind() == nodeAssignment {
		// Try both: spec property assignments and variable assignments
		p.processAssignment(node, gemspec)
		p.processVariableAssignment(node)
		return
	}

	// Handle method calls like spec.add_runtime_dependency
	if node.Kind() == nodeCall {
		p.processMethodCall(node, gemspec)
		return
	}

	// Recursively process children for other node types
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		p.processStatement(child, gemspec)
	}
}

// processAssignment handles assignment statements like spec.name = "value"
func (p *TreeSitterGemspecParser) processAssignment(node *tree_sitter.Node, gemspec *GemspecFile) {
	leftSide, rightSide := p.extractAssignmentSides(node)
	if leftSide == nil || rightSide == nil {
		return
	}

	property := p.getPropertyName(leftSide)
	value := p.extractValue(rightSide)

	// Handle simple string properties
	if p.assignSimpleProperty(property, value, gemspec) {
		return
	}

	// Handle array properties
	if p.assignArrayProperty(property, value, rightSide, gemspec) {
		return
	}

	// Handle metadata assignment
	if strings.Contains(property, "metadata") {
		key := p.extractMetadataKey(leftSide)
		if key != "" {
			gemspec.Metadata[key] = value
		}
	}
}

// extractAssignmentSides extracts left and right sides from an assignment node
func (p *TreeSitterGemspecParser) extractAssignmentSides(node *tree_sitter.Node) (leftSide, rightSide *tree_sitter.Node) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		kind := child.Kind()

		if leftSide == nil && (kind == nodeCall || kind == nodeElementReference) {
			leftSide = child
			continue
		}

		if rightSide == nil {
			switch kind {
			case nodeString, nodeArray, nodeStringContent, nodeIdentifier,
				nodeConstant, nodeScopeResolution, nodeCall, nodeSymbol, nodeInteger:
				rightSide = child
			}
		}
	}

	return
}

// assignSimpleProperty assigns simple string properties to gemspec
func (p *TreeSitterGemspecParser) assignSimpleProperty(property, value string, gemspec *GemspecFile) bool {
	switch property {
	case "name":
		gemspec.Name = value
	case "version":
		gemspec.Version = value
	case "summary":
		gemspec.Summary = value
	case "description":
		gemspec.Description = value
	case "homepage":
		gemspec.Homepage = value
	case "license":
		gemspec.License = value
	case "required_ruby_version":
		gemspec.RequiredRubyVersion = value
	case "post_install_message":
		gemspec.PostInstallMessage = value
	default:
		return false
	}
	return true
}

// assignArrayProperty handles array property assignments
func (p *TreeSitterGemspecParser) assignArrayProperty(property, value string, rightSide *tree_sitter.Node, gemspec *GemspecFile) bool {
	switch property {
	case "authors", "author":
		if rightSide.Kind() == nodeArray {
			gemspec.Authors = p.extractStringArray(rightSide)
		} else {
			gemspec.Authors = []string{value}
		}
	case "email":
		if rightSide.Kind() == nodeArray {
			gemspec.Email = p.extractStringArray(rightSide)
		} else {
			gemspec.Email = []string{value}
		}
	case "licenses":
		if rightSide.Kind() == nodeArray {
			licenses := p.extractStringArray(rightSide)
			if len(licenses) > 0 {
				gemspec.License = strings.Join(licenses, ", ")
			}
		}
	case "files":
		gemspec.Files = p.extractStringArray(rightSide)
	default:
		return false
	}
	return true
}

// processMethodCall handles method calls like spec.add_runtime_dependency
func (p *TreeSitterGemspecParser) processMethodCall(node *tree_sitter.Node, gemspec *GemspecFile) {
	methodName := ""
	var args []string

	// Extract method name and arguments
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeIdentifier {
			text := p.getNodeText(child)
			if strings.HasPrefix(text, "add_") {
				methodName = text
			}
		} else if child.Kind() == nodeArgumentList {
			args = p.extractArguments(child)
		}
	}

	// Process dependency methods
	switch methodName {
	case "add_runtime_dependency", "add_dependency":
		if len(args) > 0 {
			dep := GemDependency{
				Name:        args[0],
				Constraints: args[1:],
			}
			gemspec.RuntimeDependencies = append(gemspec.RuntimeDependencies, dep)
		}
	case "add_development_dependency":
		if len(args) > 0 {
			dep := GemDependency{
				Name:        args[0],
				Constraints: args[1:],
			}
			gemspec.DevelopmentDependencies = append(gemspec.DevelopmentDependencies, dep)
		}
	}
}

// getPropertyName extracts the property name from a method call node
func (p *TreeSitterGemspecParser) getPropertyName(node *tree_sitter.Node) string {
	switch node.Kind() {
	case nodeElementReference:
		if node.ChildCount() > 0 {
			return p.getPropertyName(node.Child(0))
		}
	case nodeCall:
		var property string
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == nodeIdentifier {
				property = p.getNodeText(child)
			}
		}
		return property
	}

	var property string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeIdentifier {
			property = p.getNodeText(child)
		}
	}
	return property
}

// extractValue extracts a string value from various node types
func (p *TreeSitterGemspecParser) extractValue(node *tree_sitter.Node) string {
	nodeType := node.Kind()

	switch nodeType {
	case nodeString:
		// Extract content from string literal
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == nodeStringContent {
				return p.getNodeText(child)
			}
		}
		// If no string_content child, extract the text removing quotes
		text := p.getNodeText(node)
		return strings.Trim(text, `'"`)
	case nodeStringContent:
		return p.getNodeText(node)
	case nodeIdentifier:
		// For identifiers, check if it's a variable reference and expand it
		varName := p.getNodeText(node)
		return p.expandVariable(varName)
	case nodeConstant:
		return p.getNodeText(node)
	case nodeScopeResolution:
		// For things like MyGem::VERSION
		return p.getNodeText(node)
	case nodeCall:
		return strings.TrimSpace(p.getNodeText(node))
	case nodeSymbol:
		return strings.TrimPrefix(p.getNodeText(node), ":")
	case nodeInteger:
		return p.getNodeText(node)
	default:
		return ""
	}
}

// extractStringArray extracts an array of strings from an array node
func (p *TreeSitterGemspecParser) extractStringArray(node *tree_sitter.Node) []string {
	var result []string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case nodeString, nodeStringContent, nodeInteger:
			value := p.extractValue(child)
			if value != "" {
				result = append(result, value)
			}
		case nodeSymbol:
			result = append(result, p.extractValue(child))
		case nodeArray:
			// Handle nested arrays
			result = append(result, p.extractStringArray(child)...)
		case nodeCall:
			value := p.extractValue(child)
			if value != "" {
				result = append(result, value)
			}
		}
	}

	return result
}

// extractArguments extracts arguments from an argument_list node
func (p *TreeSitterGemspecParser) extractArguments(node *tree_sitter.Node) []string {
	var args []string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case nodeString, nodeStringContent, nodeSymbol, nodeInteger:
			value := p.extractValue(child)
			if value != "" {
				args = append(args, value)
			}
		case nodeCall, nodeScopeResolution, nodeIdentifier, nodeConstant:
			value := p.extractValue(child)
			if value != "" {
				args = append(args, value)
			}
		}
	}

	return args
}

// extractMetadataKey extracts the key from spec.metadata["key"] expression
func (p *TreeSitterGemspecParser) extractMetadataKey(node *tree_sitter.Node) string {
	switch node.Kind() {
	case nodeElementReference:
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == nodeString {
				return p.extractValue(child)
			}
		}
		return ""
	case nodeCall:
		// Look for element_reference among children
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == nodeElementReference {
				return p.extractMetadataKey(child)
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeElementReference {
			// Look for the string inside []
			for j := uint(0); j < child.ChildCount(); j++ {
				grandchild := child.Child(j)
				if grandchild.Kind() == nodeString {
					return p.extractValue(grandchild)
				}
			}
		}
	}
	return ""
}

// getNodeText returns the text content of a node
func (p *TreeSitterGemspecParser) getNodeText(node *tree_sitter.Node) string {
	return p.helper.GetNodeText(node)
}

// processVariableAssignment parses variable assignments like: rails_version = '~> 8.1.0'
func (p *TreeSitterGemspecParser) processVariableAssignment(node *tree_sitter.Node) {
	if node == nil {
		return
	}

	// Assignment node structure: left = right
	// First child is typically the variable name (identifier)
	// Last child is typically the value (string, etc.)
	var varName, varValue string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		kind := child.Kind()
		text := p.getNodeText(child)

		if kind == nodeIdentifier && varName == "" {
			varName = text
		} else if kind == nodeString {
			// Extract string value
			varValue = p.extractValue(child)
		}
	}

	if varName != "" && varValue != "" {
		p.variables[varName] = varValue
	}
}

// expandVariable expands a variable reference to its value
// Returns the value if the input is a variable name, otherwise returns the input unchanged
func (p *TreeSitterGemspecParser) expandVariable(input string) string {
	if value, exists := p.variables[input]; exists {
		return value
	}
	return input
}
