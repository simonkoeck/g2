package semantic

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"

	"github.com/simonkoeck/g2/pkg/git"
	"github.com/simonkoeck/g2/pkg/ui"
)

// Package-level git executor (can be replaced for testing)
var gitExec git.Executor = git.NewDefaultExecutor()

// SetGitExecutor sets the git executor (for testing).
func SetGitExecutor(exec git.Executor) {
	gitExec = exec
}

// MaxFileSize is the maximum file size to process (default 10MB).
// Files larger than this will be skipped with an error.
var MaxFileSize int64 = 10 * 1024 * 1024

// Language represents a supported programming language
type Language int

const (
	LangUnknown Language = iota
	LangPython
	LangJavaScript
	LangTypeScript
	LangYAML
	LangGo
	LangRust
)

// Definition represents a code definition (function, class, or key)
type Definition struct {
	Name      string
	Kind      string // "function", "class", "key", "variable", etc.
	Signature string
	Body      string
	StartLine uint32
	EndLine   uint32
	StartByte uint32
	EndByte   uint32
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
	return GetConflictingFilesWithContext(context.Background())
}

// GetConflictingFilesWithContext returns list of files with merge conflicts using context.
func GetConflictingFilesWithContext(ctx context.Context) ([]string, error) {
	output, err := gitExec.Output(ctx, "diff", "--name-only", "--diff-filter=U")
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
	return GetFileVersionWithContext(context.Background(), file, stage)
}

// GetFileVersionWithContext retrieves a specific version of a file during merge using context.
// stage: 1=base (common ancestor), 2=local (ours), 3=remote (theirs)
func GetFileVersionWithContext(ctx context.Context, file string, stage int) ([]byte, error) {
	output, err := gitExec.Output(ctx, "show", fmt.Sprintf(":%d:%s", stage, file))
	if err != nil {
		return nil, err
	}

	// Check file size limit
	if MaxFileSize > 0 && int64(len(output)) > MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(output), MaxFileSize)
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
	case ".go":
		return LangGo
	case ".rs":
		return LangRust
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
	case LangGo:
		return parseGo(content)
	case LangRust:
		return parseRust(content)
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
	var name, params, body string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "identifier":
			name = child.Content(content)
		case "parameters":
			params = child.Content(content)
		case "block":
			// Extract just the function body (indented block), not the signature
			body = child.Content(content)
		}
	}
	if name == "" {
		return nil
	}
	// If we couldn't extract the body separately, fall back to full node content
	if body == "" {
		body = node.Content(content)
	}
	return &Definition{
		Name:      name,
		Kind:      "function",
		Signature: fmt.Sprintf("def %s%s", name, params),
		Body:      body,
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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
	var name, params, body string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "identifier":
			name = child.Content(content)
		case "formal_parameters":
			params = child.Content(content)
		case "statement_block":
			// Extract just the function body, not the signature
			body = child.Content(content)
		}
	}
	if name == "" {
		return nil
	}
	// If we couldn't extract the body separately, fall back to full node content
	if body == "" {
		body = node.Content(content)
	}
	return &Definition{
		Name:      name,
		Kind:      "function",
		Signature: fmt.Sprintf("function %s%s", name, params),
		Body:      body,
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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
						StartByte: node.StartByte(),
						EndByte:   node.EndByte(),
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

// parseGo parses Go content
func parseGo(content []byte) *FileAnalysis {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return &FileAnalysis{ParseError: err}
	}

	analysis := &FileAnalysis{}
	rootNode := tree.RootNode()

	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(i)
		extractGoDefinitions(child, content, &analysis.Definitions)
	}

	return analysis
}

// extractGoDefinitions extracts Go function, method, and type definitions
func extractGoDefinitions(node *sitter.Node, content []byte, defs *[]Definition) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_declaration":
		if def := extractGoFunction(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "method_declaration":
		if def := extractGoMethod(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "type_declaration":
		// Type declarations contain type_spec children
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "type_spec" {
				if def := extractGoTypeSpec(child, content); def != nil {
					*defs = append(*defs, *def)
				}
			}
		}
	case "const_declaration", "var_declaration":
		extractGoVarOrConst(node, content, defs)
	}
}

func extractGoFunction(node *sitter.Node, content []byte) *Definition {
	var name, params string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "identifier":
			name = child.Content(content)
		case "parameter_list":
			params = child.Content(content)
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "function",
		Signature: fmt.Sprintf("func %s%s", name, params),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractGoMethod(node *sitter.Node, content []byte) *Definition {
	var name, receiver, params string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "field_identifier":
			name = child.Content(content)
		case "parameter_list":
			if receiver == "" {
				receiver = child.Content(content)
			} else {
				params = child.Content(content)
			}
		}
	}
	if name == "" {
		return nil
	}
	return &Definition{
		Name:      name,
		Kind:      "method",
		Signature: fmt.Sprintf("func %s %s%s", receiver, name, params),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractGoTypeSpec(node *sitter.Node, content []byte) *Definition {
	var name, kind string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier":
			name = child.Content(content)
		case "struct_type":
			kind = "struct"
		case "interface_type":
			kind = "interface"
		default:
			if kind == "" {
				kind = "type"
			}
		}
	}
	if name == "" {
		return nil
	}
	if kind == "" {
		kind = "type"
	}
	return &Definition{
		Name:      name,
		Kind:      kind,
		Signature: fmt.Sprintf("type %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractGoVarOrConst(node *sitter.Node, content []byte, defs *[]Definition) {
	kind := "variable"
	if node.Type() == "const_declaration" {
		kind = "const"
	}

	// Handle both single and grouped declarations
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "const_spec" || child.Type() == "var_spec" {
			// Extract identifier(s) from the spec
			for j := 0; j < int(child.NamedChildCount()); j++ {
				specChild := child.NamedChild(j)
				if specChild.Type() == "identifier" {
					name := specChild.Content(content)
					if name != "" {
						*defs = append(*defs, Definition{
							Name:      name,
							Kind:      kind,
							Signature: fmt.Sprintf("%s %s", kind, name),
							Body:      child.Content(content),
							StartLine: child.StartPoint().Row,
							EndLine:   child.EndPoint().Row,
							StartByte: child.StartByte(),
							EndByte:   child.EndByte(),
						})
					}
					break // Only first identifier per spec
				}
			}
		}
	}
}

// parseRust parses Rust content
func parseRust(content []byte) *FileAnalysis {
	parser := sitter.NewParser()
	parser.SetLanguage(rust.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return &FileAnalysis{ParseError: err}
	}

	analysis := &FileAnalysis{}
	rootNode := tree.RootNode()

	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(i)
		extractRustDefinitions(child, content, &analysis.Definitions)
	}

	return analysis
}

// extractRustDefinitions extracts Rust fn, impl, struct, enum, and trait definitions
func extractRustDefinitions(node *sitter.Node, content []byte, defs *[]Definition) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_item":
		if def := extractRustFunction(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "impl_item":
		if def := extractRustImpl(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "struct_item":
		if def := extractRustStruct(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "enum_item":
		if def := extractRustEnum(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "trait_item":
		if def := extractRustTrait(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "type_item":
		if def := extractRustTypeAlias(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	case "const_item", "static_item":
		if def := extractRustConstOrStatic(node, content); def != nil {
			*defs = append(*defs, *def)
		}
	}
}

func extractRustFunction(node *sitter.Node, content []byte) *Definition {
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
		Signature: fmt.Sprintf("fn %s%s", name, params),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractRustImpl(node *sitter.Node, content []byte) *Definition {
	var typeName, traitName string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier", "generic_type":
			if typeName == "" {
				typeName = child.Content(content)
			}
		case "trait_bounds":
			traitName = child.Content(content)
		}
	}

	if typeName == "" {
		return nil
	}

	name := typeName
	sig := fmt.Sprintf("impl %s", typeName)
	if traitName != "" {
		name = fmt.Sprintf("%s for %s", traitName, typeName)
		sig = fmt.Sprintf("impl %s for %s", traitName, typeName)
	}

	return &Definition{
		Name:      name,
		Kind:      "impl",
		Signature: sig,
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractRustStruct(node *sitter.Node, content []byte) *Definition {
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
		Kind:      "struct",
		Signature: fmt.Sprintf("struct %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractRustEnum(node *sitter.Node, content []byte) *Definition {
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
		Kind:      "enum",
		Signature: fmt.Sprintf("enum %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractRustTrait(node *sitter.Node, content []byte) *Definition {
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
		Kind:      "trait",
		Signature: fmt.Sprintf("trait %s", name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractRustTypeAlias(node *sitter.Node, content []byte) *Definition {
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
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
	}
}

func extractRustConstOrStatic(node *sitter.Node, content []byte) *Definition {
	kind := "const"
	if node.Type() == "static_item" {
		kind = "static"
	}

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
		Kind:      kind,
		Signature: fmt.Sprintf("%s %s", kind, name),
		Body:      node.Content(content),
		StartLine: node.StartPoint().Row,
		EndLine:   node.EndPoint().Row,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
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

	// Case 1b: Added only in local (orphan add)
	if base == nil && local != nil && remote == nil {
		return &ui.Conflict{
			File:         file,
			ConflictType: fmt.Sprintf("%s '%s' Added (local)", kindStr, name),
			Status:       "Needs Resolution",
		}
	}

	// Case 1c: Added only in remote (orphan add)
	if base == nil && local == nil && remote != nil {
		return &ui.Conflict{
			File:         file,
			ConflictType: fmt.Sprintf("%s '%s' Added (remote)", kindStr, name),
			Status:       "Needs Resolution",
		}
	}

	// Case 2: Removed in one branch, modified in other
	if base != nil {
		// Deleted on both branches (orphan delete)
		if local == nil && remote == nil {
			return &ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s '%s' Deleted", kindStr, name),
				Status:       "Needs Resolution",
			}
		}
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
			// Check if bodies are exactly identical
			if local.Body == remote.Body {
				return &ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Modified (same)", kindStr, name),
					Status:       "Can Auto-merge",
				}
			}
			// Check if semantically identical but different formatting
			if localNorm == remoteNorm {
				return &ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Formatted Change", kindStr, name),
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
