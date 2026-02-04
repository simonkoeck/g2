package semantic

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"

	"github.com/simonkoeck/g2/pkg/ui"
)

// Language represents a supported programming language
type Language int

const (
	LangUnknown Language = iota
	LangPython
	LangJavaScript
	LangTypeScript
	LangYAML
)

// Definition represents a code definition (function, class, or key)
type Definition struct {
	Name      string
	Kind      string // "function", "class", "key", "variable", etc.
	Signature string
	Body      string
	StartLine uint32
	EndLine   uint32
}

// FileAnalysis contains parsed definitions from a file
type FileAnalysis struct {
	Definitions []Definition
	ParseError  error
}

// ConflictAnalysis contains the result of analyzing a conflicting file
type ConflictAnalysis struct {
	File      string
	Conflicts []ui.Conflict
}

// GetConflictingFiles returns list of files with merge conflicts
func GetConflictingFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get conflicting files: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// GetFileVersion retrieves a specific version of a file during merge
// stage: 1=base (common ancestor), 2=local (ours), 3=remote (theirs)
func GetFileVersion(file string, stage int) ([]byte, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf(":%d:%s", stage, file))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}

// DetectLanguage determines the language of a file based on extension
func DetectLanguage(file string) Language {
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".py":
		return LangPython
	case ".js", ".mjs", ".cjs", ".jsx":
		return LangJavaScript
	case ".ts", ".mts", ".cts", ".tsx":
		return LangTypeScript
	case ".json", ".yaml", ".yml":
		return LangYAML
	default:
		return LangUnknown
	}
}

// IsSemanticFile checks if a file supports semantic analysis
func IsSemanticFile(file string) bool {
	return DetectLanguage(file) != LangUnknown
}

// IsBinaryFile checks if content appears to be binary
func IsBinaryFile(content []byte) bool {
	checkLen := len(content)
	if checkLen > 8000 {
		checkLen = 8000
	}
	return bytes.Contains(content[:checkLen], []byte{0})
}

// ParseFile parses content based on detected language
func ParseFile(content []byte, lang Language) *FileAnalysis {
	switch lang {
	case LangPython:
		return parsePython(content)
	case LangJavaScript:
		return parseJavaScript(content)
	case LangTypeScript:
		return parseTypeScript(content)
	case LangYAML:
		return parseYAML(content)
	default:
		return &FileAnalysis{ParseError: fmt.Errorf("unsupported language")}
	}
}

// parsePython parses Python content
func parsePython(content []byte) *FileAnalysis {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return &FileAnalysis{ParseError: err}
	}

	analysis := &FileAnalysis{}
	rootNode := tree.RootNode()

	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(i)
		extractPythonDefinitions(child, content, &analysis.Definitions)
	}

	return analysis
}

// extractPythonDefinitions extracts Python function and class definitions
func extractPythonDefinitions(node *sitter.Node, content []byte, defs *[]Definition) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_definition":
		if def := extractPythonFunction(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "class_definition":
		if def := extractPythonClass(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "decorated_definition":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "function_definition" || child.Type() == "class_definition" {
				extractPythonDefinitions(child, content, defs)
			}
		}
	}
}

func extractPythonFunction(node *sitter.Node, content []byte) *Definition {
	var name, params string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "identifier":
			name = child.Content(content)
		case "parameters":
			params = child.Content(content)
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "function",
		Signature: fmt.Sprintf("def %s%s", name, params),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
	}
}

func extractPythonClass(node *sitter.Node, content []byte) *Definition {
	var name string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "identifier" {
			name = child.Content(content)
			break
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "class",
		Signature: fmt.Sprintf("class %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
	}
}

// parseJavaScript parses JavaScript content
func parseJavaScript(content []byte) *FileAnalysis {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return &FileAnalysis{ParseError: err}
	}

	analysis := &FileAnalysis{}
	rootNode := tree.RootNode()

	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(i)
		extractJSDefinitions(child, content, &analysis.Definitions)
	}

	return analysis
}

// parseTypeScript parses TypeScript content
func parseTypeScript(content []byte) *FileAnalysis {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return &FileAnalysis{ParseError: err}
	}

	analysis := &FileAnalysis{}
	rootNode := tree.RootNode()

	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(i)
		extractJSDefinitions(child, content, &analysis.Definitions)
	}

	return analysis
}

// extractJSDefinitions extracts JavaScript/TypeScript definitions
func extractJSDefinitions(node *sitter.Node, content []byte, defs *[]Definition) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "function_declaration":
		if def := extractJSFunction(node, content); def != nil {
			*defs = append(*defs, *def)
		}

	case "class_declaration":
		if def := extractJSClass(node, content); def != nil {
			*defs = append(*defs, *def)
		}

	case "lexical_declaration", "variable_declaration":
		// Handle const/let/var declarations (including arrow functions)
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "variable_declarator" {
				if def := extractJSVariableDeclarator(child, content); def != nil {
					*defs = append(*defs, *def)
				}
			}
		}

	case "export_statement":
		// Handle exported declarations
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			extractJSDefinitions(child, content, defs)
		}

	case "interface_declaration":
		if def := extractTSInterface(node, content); def != nil {
			*defs = append(*defs, *def)
		}

	case "type_alias_declaration":
		if def := extractTSTypeAlias(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	}
}

func extractJSFunction(node *sitter.Node, content []byte) *Definition {
	var name, params string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "identifier":
			name = child.Content(content)
		case "formal_parameters":
			params = child.Content(content)
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "function",
		Signature: fmt.Sprintf("function %s%s", name, params),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
	}
}

func extractJSClass(node *sitter.Node, content []byte) *Definition {
	var name string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "identifier" || child.Type() == "type_identifier" {
			name = child.Content(content)
			break
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "class",
		Signature: fmt.Sprintf("class %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
	}
}

func extractJSVariableDeclarator(node *sitter.Node, content []byte) *Definition {
	var name string
	var value *sitter.Node

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "identifier":
			name = child.Content(content)
		case "arrow_function", "function":
			value = child
		}
	}

	if name == "" {
		return nil
	}

	// Determine kind based on value type
	kind := "variable"
	sig := name
	if value != nil {
		switch value.Type() {
		case "arrow_function", "function":
			kind = "function"
			// Extract parameters for arrow functions
			for i := 0; i < int(value.NamedChildCount()); i++ {
				child := value.NamedChild(i)
				if child.Type() == "formal_parameters" || child.Type() == "identifier" {
					sig = fmt.Sprintf("const %s = %s", name, child.Content(content))
					break
				}
			}
			if sig == name {
				sig = fmt.Sprintf("const %s = () =>", name)
			}
		}
	}

	return &Definition{
		Name:      name,
		Kind:      kind,
		Signature: sig,
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
	}
}

func extractTSInterface(node *sitter.Node, content []byte) *Definition {
	var name string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "type_identifier" {
			name = child.Content(content)
			break
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "interface",
		Signature: fmt.Sprintf("interface %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
	}
}

func extractTSTypeAlias(node *sitter.Node, content []byte) *Definition {
	var name string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "type_identifier" {
			name = child.Content(content)
			break
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "type",
		Signature: fmt.Sprintf("type %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
	}
}

// parseYAML parses YAML/JSON content and extracts top-level keys
func parseYAML(content []byte) *FileAnalysis {
	parser := sitter.NewParser()
	parser.SetLanguage(yaml.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return &FileAnalysis{ParseError: err}
	}

	analysis := &FileAnalysis{}
	rootNode := tree.RootNode()

	// YAML root is a stream containing documents
	extractYAMLKeys(rootNode, content, &analysis.Definitions)

	return analysis
}

func extractYAMLKeys(node *sitter.Node, content []byte, defs *[]Definition) {
	if node == nil {
		return
	}

	// Recursively search for block_mapping_pair or flow_pair nodes
	nodeType := node.Type()

	if nodeType == "block_mapping_pair" || nodeType == "flow_pair" {
		// First child is typically the key
		if node.NamedChildCount() > 0 {
			keyNode := node.NamedChild(0)
			if keyNode != nil {
				keyName := strings.Trim(keyNode.Content(content), "\"'")
				if keyName != "" {
					*defs = append(*defs, Definition{
						Name:      keyName,
						Kind:      "key",
						Signature: fmt.Sprintf("%s:", keyName),
						Body:      node.Content(content),
						StartLine: node.StartPoint().Row,
						EndLine:   node.EndPoint().Row,
					})
				}
			}
		}
		return // Don't recurse into nested keys
	}

	// Recurse into children
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		extractYAMLKeys(child, content, defs)
	}
}

// AnalyzeConflict analyzes a conflicting file and returns semantic conflict info
func AnalyzeConflict(file string) *ConflictAnalysis {
	result := &ConflictAnalysis{File: file}
	lang := DetectLanguage(file)

	// Get all three versions
	baseContent, baseErr := GetFileVersion(file, 1)
	localContent, localErr := GetFileVersion(file, 2)
	remoteContent, remoteErr := GetFileVersion(file, 3)

	// Handle binary files
	if (localErr == nil && IsBinaryFile(localContent)) ||
		(remoteErr == nil && IsBinaryFile(remoteContent)) {
		result.Conflicts = append(result.Conflicts, ui.Conflict{
			File:         file,
			ConflictType: "Binary Conflict",
			Status:       "Needs Resolution",
		})
		return result
	}

	// Parse all versions
	var baseAnalysis, localAnalysis, remoteAnalysis *FileAnalysis

	if baseErr == nil {
		baseAnalysis = ParseFile(baseContent, lang)
	} else {
		baseAnalysis = &FileAnalysis{} // Empty base (new file)
	}

	if localErr != nil {
		result.Conflicts = append(result.Conflicts, ui.Conflict{
			File:         file,
			ConflictType: "File Missing (local)",
			Status:       "Needs Resolution",
		})
		return result
	}
	localAnalysis = ParseFile(localContent, lang)

	if remoteErr != nil {
		result.Conflicts = append(result.Conflicts, ui.Conflict{
			File:         file,
			ConflictType: "File Missing (remote)",
			Status:       "Needs Resolution",
		})
		return result
	}
	remoteAnalysis = ParseFile(remoteContent, lang)

	// Check for parse errors
	if localAnalysis.ParseError != nil || remoteAnalysis.ParseError != nil {
		result.Conflicts = append(result.Conflicts, ui.Conflict{
			File:         file,
			ConflictType: "Parse Error",
			Status:       "Needs Resolution",
		})
		return result
	}

	// Map definitions by name
	baseDefs := mapDefinitions(baseAnalysis.Definitions)
	localDefs := mapDefinitions(localAnalysis.Definitions)
	remoteDefs := mapDefinitions(remoteAnalysis.Definitions)

	// Find all unique definition names
	allNames := make(map[string]bool)
	for name := range baseDefs {
		allNames[name] = true
	}
	for name := range localDefs {
		allNames[name] = true
	}
	for name := range remoteDefs {
		allNames[name] = true
	}

	// Analyze each definition
	for name := range allNames {
		baseDef := baseDefs[name]
		localDef := localDefs[name]
		remoteDef := remoteDefs[name]

		conflict := analyzeDefinitionChange(file, name, baseDef, localDef, remoteDef)
		if conflict != nil {
			result.Conflicts = append(result.Conflicts, *conflict)
		}
	}

	// If no specific conflicts found but file is conflicting, mark as text conflict
	if len(result.Conflicts) == 0 {
		result.Conflicts = append(result.Conflicts, ui.Conflict{
			File:         file,
			ConflictType: "Text Conflict",
			Status:       "Needs Resolution",
		})
	}

	return result
}

// mapDefinitions creates a map of definitions by name
func mapDefinitions(defs []Definition) map[string]*Definition {
	m := make(map[string]*Definition)
	for i := range defs {
		m[defs[i].Name] = &defs[i]
	}
	return m
}

// normalize collapses all whitespace to single spaces for semantic comparison
// This allows detecting when two changes are semantically identical but differ in formatting
func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// analyzeDefinitionChange determines what kind of conflict exists for a definition
func analyzeDefinitionChange(file, name string, base, local, remote *Definition) *ui.Conflict {
	// Determine the kind (use whichever version has it)
	kind := "definition"
	if local != nil {
		kind = local.Kind
	} else if remote != nil {
		kind = remote.Kind
	} else if base != nil {
		kind = base.Kind
	}

	// Format kind nicely (capitalize first letter)
	kindStr := strings.ToUpper(kind[:1]) + kind[1:]

	// Normalize bodies for semantic comparison (ignores whitespace differences)
	var baseNorm, localNorm, remoteNorm string
	if base != nil {
		baseNorm = normalize(base.Body)
	}
	if local != nil {
		localNorm = normalize(local.Body)
	}
	if remote != nil {
		remoteNorm = normalize(remote.Body)
	}

	// Case 1: Added in both branches (didn't exist in base)
	if base == nil && local != nil && remote != nil {
		if localNorm == remoteNorm {
			return &ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s Added (identical)", kindStr),
				Status:       "Can Auto-merge",
			}
		}
		return &ui.Conflict{
			File:         file,
			ConflictType: fmt.Sprintf("%s '%s' Added (differs)", kindStr, name),
			Status:       "Needs Resolution",
		}
	}

	// Case 2: Removed in one branch, modified in other
	if base != nil {
		if local == nil && remote != nil && remoteNorm != baseNorm {
			return &ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s '%s' Delete/Modify", kindStr, name),
				Status:       "Needs Resolution",
			}
		}
		if remote == nil && local != nil && localNorm != baseNorm {
			return &ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s '%s' Modify/Delete", kindStr, name),
				Status:       "Needs Resolution",
			}
		}
	}

	// Case 3: Modified in both branches
	if base != nil && local != nil && remote != nil {
		localChanged := localNorm != baseNorm
		remoteChanged := remoteNorm != baseNorm

		if localChanged && remoteChanged {
			if localNorm == remoteNorm {
				return &ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Modified (same)", kindStr, name),
					Status:       "Can Auto-merge",
				}
			}
			return &ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s '%s' Modified", kindStr, name),
				Status:       "Needs Resolution",
			}
		}
	}

	// No conflict for this definition
	return nil
}

// AnalyzeNonSemanticFile creates a generic conflict entry for unsupported files
func AnalyzeNonSemanticFile(file string) *ConflictAnalysis {
	localContent, err := GetFileVersion(file, 2)
	if err == nil && IsBinaryFile(localContent) {
		return &ConflictAnalysis{
			File: file,
			Conflicts: []ui.Conflict{{
				File:         file,
				ConflictType: "Binary Conflict",
				Status:       "Needs Resolution",
			}},
		}
	}

	return &ConflictAnalysis{
		File: file,
		Conflicts: []ui.Conflict{{
			File:         file,
			ConflictType: "Text Conflict",
			Status:       "Needs Resolution",
		}},
	}
}
