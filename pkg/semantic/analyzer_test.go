package semantic

import (
	"strings"
	"testing"
)

func TestParseGo_Function(t *testing.T) {
	content := []byte(`package main

func hello() string {
	return "hello"
}

func add(a, b int) int {
	return a + b
}
`)

	analysis := parseGo(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	// Check first function
	if analysis.Definitions[0].Name != "hello" {
		t.Errorf("expected name 'hello', got '%s'", analysis.Definitions[0].Name)
	}
	if analysis.Definitions[0].Kind != "function" {
		t.Errorf("expected kind 'function', got '%s'", analysis.Definitions[0].Kind)
	}
}

func TestParseGo_Method(t *testing.T) {
	content := []byte(`package main

type Person struct {
	Name string
}

func (p *Person) Greet() string {
	return "Hello, " + p.Name
}

func (p Person) GetName() string {
	return p.Name
}
`)

	analysis := parseGo(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should find: Person struct + 2 methods
	if len(analysis.Definitions) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	// Verify we found the struct and methods
	kinds := make(map[string]int)
	for _, def := range analysis.Definitions {
		kinds[def.Kind]++
	}

	if kinds["struct"] != 1 {
		t.Errorf("expected 1 struct, got %d", kinds["struct"])
	}
	if kinds["method"] != 2 {
		t.Errorf("expected 2 methods, got %d", kinds["method"])
	}
}

func TestParseGo_Struct(t *testing.T) {
	content := []byte(`package main

type Person struct {
	Name string
	Age  int
}

type Config struct {
	Host string
	Port int
}
`)

	analysis := parseGo(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	for _, def := range analysis.Definitions {
		if def.Kind != "struct" {
			t.Errorf("expected kind 'struct', got '%s' for %s", def.Kind, def.Name)
		}
	}
}

func TestParseGo_Interface(t *testing.T) {
	content := []byte(`package main

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}
`)

	analysis := parseGo(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	for _, def := range analysis.Definitions {
		if def.Kind != "interface" {
			t.Errorf("expected kind 'interface', got '%s' for %s", def.Kind, def.Name)
		}
	}
}

func TestParseGo_Const(t *testing.T) {
	content := []byte(`package main

const MaxSize = 100

const (
	StatusOK = 200
	StatusNotFound = 404
)
`)

	analysis := parseGo(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) < 1 {
		t.Errorf("expected at least 1 definition, got %d", len(analysis.Definitions))
	}

	for _, def := range analysis.Definitions {
		if def.Kind != "const" {
			t.Errorf("expected kind 'const', got '%s' for %s", def.Kind, def.Name)
		}
	}
}

func TestParseRust_Function(t *testing.T) {
	content := []byte(`fn hello() -> String {
    "hello".to_string()
}

fn add(a: i32, b: i32) -> i32 {
    a + b
}
`)

	analysis := parseRust(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	if analysis.Definitions[0].Name != "hello" {
		t.Errorf("expected name 'hello', got '%s'", analysis.Definitions[0].Name)
	}
	if analysis.Definitions[0].Kind != "function" {
		t.Errorf("expected kind 'function', got '%s'", analysis.Definitions[0].Kind)
	}
}

func TestParseRust_Struct(t *testing.T) {
	content := []byte(`struct Person {
    name: String,
    age: u32,
}

struct Config {
    host: String,
    port: u16,
}
`)

	analysis := parseRust(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	for _, def := range analysis.Definitions {
		if def.Kind != "struct" {
			t.Errorf("expected kind 'struct', got '%s' for %s", def.Kind, def.Name)
		}
	}
}

func TestParseRust_Enum(t *testing.T) {
	content := []byte(`enum Color {
    Red,
    Green,
    Blue,
}

enum Option<T> {
    Some(T),
    None,
}
`)

	analysis := parseRust(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	for _, def := range analysis.Definitions {
		if def.Kind != "enum" {
			t.Errorf("expected kind 'enum', got '%s' for %s", def.Kind, def.Name)
		}
	}
}

func TestParseRust_Impl(t *testing.T) {
	content := []byte(`struct Person {
    name: String,
}

impl Person {
    fn new(name: String) -> Self {
        Person { name }
    }
}
`)

	analysis := parseRust(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should find struct + impl
	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	kinds := make(map[string]int)
	for _, def := range analysis.Definitions {
		kinds[def.Kind]++
	}

	if kinds["struct"] != 1 {
		t.Errorf("expected 1 struct, got %d", kinds["struct"])
	}
	if kinds["impl"] != 1 {
		t.Errorf("expected 1 impl, got %d", kinds["impl"])
	}
}

func TestParseRust_Trait(t *testing.T) {
	content := []byte(`trait Greet {
    fn greet(&self) -> String;
}

trait Display {
    fn display(&self);
}
`)

	analysis := parseRust(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	for _, def := range analysis.Definitions {
		if def.Kind != "trait" {
			t.Errorf("expected kind 'trait', got '%s' for %s", def.Kind, def.Name)
		}
	}
}

func TestParseRust_Const(t *testing.T) {
	content := []byte(`const MAX_SIZE: usize = 100;

static GLOBAL: &str = "hello";
`)

	analysis := parseRust(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	// Check we have const and static
	kinds := make(map[string]bool)
	for _, def := range analysis.Definitions {
		kinds[def.Kind] = true
	}

	if !kinds["const"] {
		t.Error("expected to find const")
	}
	if !kinds["static"] {
		t.Error("expected to find static")
	}
}

func TestDetectLanguage_Go(t *testing.T) {
	tests := []struct {
		file     string
		expected Language
	}{
		{"main.go", LangGo},
		{"pkg/foo/bar.go", LangGo},
		{"test_test.go", LangGo},
	}

	for _, test := range tests {
		if got := DetectLanguage(test.file); got != test.expected {
			t.Errorf("DetectLanguage(%q) = %v, want %v", test.file, got, test.expected)
		}
	}
}

func TestDetectLanguage_Rust(t *testing.T) {
	tests := []struct {
		file     string
		expected Language
	}{
		{"main.rs", LangRust},
		{"src/lib.rs", LangRust},
		{"tests/test.rs", LangRust},
	}

	for _, test := range tests {
		if got := DetectLanguage(test.file); got != test.expected {
			t.Errorf("DetectLanguage(%q) = %v, want %v", test.file, got, test.expected)
		}
	}
}

func TestIsSemanticFile_GoAndRust(t *testing.T) {
	tests := []struct {
		file     string
		expected bool
	}{
		{"main.go", true},
		{"lib.rs", true},
		{"main.py", true},
		{"app.js", true},
		{"README.md", false},
		{"Makefile", false},
	}

	for _, test := range tests {
		if got := IsSemanticFile(test.file); got != test.expected {
			t.Errorf("IsSemanticFile(%q) = %v, want %v", test.file, got, test.expected)
		}
	}
}

func TestParsePython_Function(t *testing.T) {
	content := []byte(`def hello():
    return "hello"

def add(a, b):
    return a + b
`)

	analysis := parsePython(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	if analysis.Definitions[0].Name != "hello" {
		t.Errorf("expected name 'hello', got '%s'", analysis.Definitions[0].Name)
	}
	if analysis.Definitions[0].Kind != "function" {
		t.Errorf("expected kind 'function', got '%s'", analysis.Definitions[0].Kind)
	}
}

func TestParsePython_ClassWithMethods(t *testing.T) {
	content := []byte(`class Calculator:
    def __init__(self, value):
        self.value = value

    def add(self, x):
        return self.value + x

    def multiply(self, x):
        return self.value * x
`)

	analysis := parsePython(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should extract 3 methods: Calculator.__init__, Calculator.add, Calculator.multiply
	if len(analysis.Definitions) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	// Check that methods have qualified names
	expectedNames := map[string]bool{
		"Calculator.__init__": false,
		"Calculator.add":      false,
		"Calculator.multiply": false,
	}

	for _, def := range analysis.Definitions {
		if _, ok := expectedNames[def.Name]; ok {
			expectedNames[def.Name] = true
		}
		if def.Kind != "method" {
			t.Errorf("expected kind 'method', got '%s' for %s", def.Kind, def.Name)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected to find method %s", name)
		}
	}
}

func TestParsePython_ClassWithDecoratedMethods(t *testing.T) {
	content := []byte(`class MyClass:
    @staticmethod
    def static_method():
        return "static"

    @classmethod
    def class_method(cls):
        return "class"

    @property
    def my_property(self):
        return self._value
`)

	analysis := parsePython(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should extract 3 methods
	if len(analysis.Definitions) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	expectedNames := map[string]bool{
		"MyClass.static_method": false,
		"MyClass.class_method":  false,
		"MyClass.my_property":   false,
	}

	for _, def := range analysis.Definitions {
		if _, ok := expectedNames[def.Name]; ok {
			expectedNames[def.Name] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected to find method %s", name)
		}
	}
}

func TestParseJavaScript_Function(t *testing.T) {
	content := []byte(`function hello() {
    return "hello";
}

function add(a, b) {
    return a + b;
}
`)

	analysis := parseJavaScript(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
	}

	if analysis.Definitions[0].Name != "hello" {
		t.Errorf("expected name 'hello', got '%s'", analysis.Definitions[0].Name)
	}
	if analysis.Definitions[0].Kind != "function" {
		t.Errorf("expected kind 'function', got '%s'", analysis.Definitions[0].Kind)
	}
}

func TestParseJavaScript_ClassWithMethods(t *testing.T) {
	content := []byte(`class Calculator {
    constructor(value) {
        this.value = value;
    }

    add(x) {
        return this.value + x;
    }

    multiply(x) {
        return this.value * x;
    }
}
`)

	analysis := parseJavaScript(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should extract 3 methods: Calculator.constructor, Calculator.add, Calculator.multiply
	if len(analysis.Definitions) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	expectedNames := map[string]bool{
		"Calculator.constructor": false,
		"Calculator.add":         false,
		"Calculator.multiply":    false,
	}

	for _, def := range analysis.Definitions {
		if _, ok := expectedNames[def.Name]; ok {
			expectedNames[def.Name] = true
		}
		if def.Kind != "method" {
			t.Errorf("expected kind 'method', got '%s' for %s", def.Kind, def.Name)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected to find method %s", name)
		}
	}
}

func TestParseJavaScript_ClassWithGettersSetters(t *testing.T) {
	content := []byte(`class Person {
    constructor(name) {
        this._name = name;
    }

    get name() {
        return this._name;
    }

    set name(value) {
        this._name = value;
    }
}
`)

	analysis := parseJavaScript(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should extract 3: constructor, getter, setter
	if len(analysis.Definitions) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	// Check for getter and setter kinds
	kinds := make(map[string]int)
	for _, def := range analysis.Definitions {
		kinds[def.Kind]++
	}

	if kinds["getter"] != 1 {
		t.Errorf("expected 1 getter, got %d", kinds["getter"])
	}
	if kinds["setter"] != 1 {
		t.Errorf("expected 1 setter, got %d", kinds["setter"])
	}
}

func TestParseTypeScript_ClassWithMethods(t *testing.T) {
	content := []byte(`class Calculator {
    private value: number;

    constructor(value: number) {
        this.value = value;
    }

    add(x: number): number {
        return this.value + x;
    }

    static create(): Calculator {
        return new Calculator(0);
    }
}
`)

	analysis := parseTypeScript(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should extract methods (constructor, add, create)
	if len(analysis.Definitions) < 3 {
		t.Errorf("expected at least 3 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	// Check that we found Calculator.add
	foundAdd := false
	for _, def := range analysis.Definitions {
		if def.Name == "Calculator.add" {
			foundAdd = true
			break
		}
	}
	if !foundAdd {
		t.Error("expected to find Calculator.add method")
	}
}

func TestParseJavaScript_ArrowFunctionClassField(t *testing.T) {
	content := []byte(`class EventHandler {
    handleClick = () => {
        console.log("clicked");
    }

    handleSubmit = async (event) => {
        await submit(event);
    }
}
`)

	analysis := parseJavaScript(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should extract arrow function class fields
	if len(analysis.Definitions) < 1 {
		t.Errorf("expected at least 1 definition, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	// Check for EventHandler.handleClick
	foundHandleClick := false
	for _, def := range analysis.Definitions {
		if def.Name == "EventHandler.handleClick" {
			foundHandleClick = true
			break
		}
	}
	if !foundHandleClick {
		t.Error("expected to find EventHandler.handleClick")
		for _, d := range analysis.Definitions {
			t.Logf("  found: %s (%s)", d.Name, d.Kind)
		}
	}
}

// =============================================================================
// IsBinaryFile Tests
// =============================================================================

func TestIsBinaryFile(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "empty file is not binary",
			content:  []byte{},
			expected: false,
		},
		{
			name:     "plain text is not binary",
			content:  []byte("Hello, World!\nThis is a text file."),
			expected: false,
		},
		{
			name:     "python code is not binary",
			content:  []byte("def foo():\n    return 42\n"),
			expected: false,
		},
		{
			name:     "json is not binary",
			content:  []byte(`{"key": "value", "number": 123}`),
			expected: false,
		},
		{
			name:     "null byte makes it binary",
			content:  []byte("text\x00with null"),
			expected: true,
		},
		{
			name:     "null byte at start is binary",
			content:  []byte("\x00binary content"),
			expected: true,
		},
		{
			name:     "null byte at end is binary",
			content:  []byte("binary content\x00"),
			expected: true,
		},
		{
			name:     "png header is binary",
			content:  []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"),
			expected: true,
		},
		{
			name:     "gif header is binary",
			content:  []byte("GIF89a\x00\x00\x00"),
			expected: true,
		},
		{
			name:     "unicode text is not binary",
			content:  []byte("Hello 世界! こんにちは 🎉"),
			expected: false,
		},
		{
			name:     "high ASCII but no null is not binary",
			content:  []byte{0x80, 0x81, 0xFF, 0xFE},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBinaryFile(tt.content)
			if result != tt.expected {
				t.Errorf("IsBinaryFile() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsBinaryFile_LargeFile(t *testing.T) {
	// Test that we only check first 8000 bytes
	t.Run("null byte after 8000 bytes is not detected", func(t *testing.T) {
		content := make([]byte, 10000)
		for i := range content {
			content[i] = 'a' // Fill with text
		}
		content[8500] = 0 // Add null byte after 8000

		result := IsBinaryFile(content)
		if result != false {
			t.Error("null byte after 8000 bytes should not be detected")
		}
	})

	t.Run("null byte within 8000 bytes is detected", func(t *testing.T) {
		content := make([]byte, 10000)
		for i := range content {
			content[i] = 'a'
		}
		content[7999] = 0 // Add null byte within 8000

		result := IsBinaryFile(content)
		if result != true {
			t.Error("null byte within 8000 bytes should be detected")
		}
	})
}

// =============================================================================
// YAML Parsing Tests
// =============================================================================

func TestParseYAML_SimpleKeys(t *testing.T) {
	content := []byte(`name: myapp
version: 1.0.0
description: A sample application
`)

	analysis := parseYAML(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	expectedKeys := map[string]bool{
		"name":        false,
		"version":     false,
		"description": false,
	}

	for _, def := range analysis.Definitions {
		if _, ok := expectedKeys[def.Name]; ok {
			expectedKeys[def.Name] = true
		}
		if def.Kind != "key" {
			t.Errorf("expected kind 'key', got '%s' for %s", def.Kind, def.Name)
		}
	}

	for key, found := range expectedKeys {
		if !found {
			t.Errorf("expected to find key '%s'", key)
		}
	}
}

func TestParseYAML_NestedStructure(t *testing.T) {
	content := []byte(`database:
  host: localhost
  port: 5432
server:
  host: 0.0.0.0
  port: 8080
`)

	analysis := parseYAML(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should only extract top-level keys (database, server)
	// Not nested keys (host, port)
	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 top-level definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}

	foundDatabase := false
	foundServer := false
	for _, def := range analysis.Definitions {
		if def.Name == "database" {
			foundDatabase = true
		}
		if def.Name == "server" {
			foundServer = true
		}
	}

	if !foundDatabase {
		t.Error("expected to find 'database' key")
	}
	if !foundServer {
		t.Error("expected to find 'server' key")
	}
}

func TestParseYAML_WithLists(t *testing.T) {
	content := []byte(`dependencies:
  - express
  - lodash
  - axios
scripts:
  build: npm run build
  test: npm test
`)

	analysis := parseYAML(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(analysis.Definitions))
		for _, d := range analysis.Definitions {
			t.Logf("  - %s (%s)", d.Name, d.Kind)
		}
	}
}

func TestParseYAML_JSON(t *testing.T) {
	// YAML parser should also handle JSON
	content := []byte(`{"name": "myapp", "version": "1.0.0"}`)

	analysis := parseYAML(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) < 1 {
		t.Errorf("expected at least 1 definition, got %d", len(analysis.Definitions))
	}
}

func TestParseYAML_QuotedKeys(t *testing.T) {
	content := []byte(`"quoted-key": value
'single-quoted': another
unquoted: third
`)

	analysis := parseYAML(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should strip quotes from key names
	for _, def := range analysis.Definitions {
		if strings.Contains(def.Name, `"`) || strings.Contains(def.Name, `'`) {
			t.Errorf("key name should not contain quotes: %s", def.Name)
		}
	}
}

func TestParseYAML_Empty(t *testing.T) {
	content := []byte(``)

	analysis := parseYAML(content)

	// Empty YAML should parse without error
	if analysis.ParseError != nil {
		t.Fatalf("parse error on empty YAML: %v", analysis.ParseError)
	}

	if len(analysis.Definitions) != 0 {
		t.Errorf("expected 0 definitions for empty YAML, got %d", len(analysis.Definitions))
	}
}

func TestParseYAML_MultiDocument(t *testing.T) {
	content := []byte(`---
first: document
---
second: document
`)

	analysis := parseYAML(content)

	if analysis.ParseError != nil {
		t.Fatalf("parse error: %v", analysis.ParseError)
	}

	// Should handle multi-document YAML
	if len(analysis.Definitions) < 1 {
		t.Errorf("expected at least 1 definition, got %d", len(analysis.Definitions))
	}
}

// =============================================================================
// mapDefinitions Helper Tests
// =============================================================================

func TestMapDefinitions(t *testing.T) {
	t.Run("empty slice returns empty map", func(t *testing.T) {
		result := mapDefinitions([]Definition{})
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("maps definitions by name", func(t *testing.T) {
		defs := []Definition{
			{Name: "foo", Kind: "function", Body: "body1"},
			{Name: "bar", Kind: "class", Body: "body2"},
			{Name: "baz", Kind: "variable", Body: "body3"},
		}

		result := mapDefinitions(defs)

		if len(result) != 3 {
			t.Errorf("expected 3 entries, got %d", len(result))
		}

		if result["foo"] == nil || result["foo"].Kind != "function" {
			t.Error("expected to find 'foo' as function")
		}
		if result["bar"] == nil || result["bar"].Kind != "class" {
			t.Error("expected to find 'bar' as class")
		}
		if result["baz"] == nil || result["baz"].Kind != "variable" {
			t.Error("expected to find 'baz' as variable")
		}
	})

	t.Run("duplicate names - last wins", func(t *testing.T) {
		defs := []Definition{
			{Name: "foo", Kind: "function", Body: "first"},
			{Name: "foo", Kind: "function", Body: "second"},
		}

		result := mapDefinitions(defs)

		if len(result) != 1 {
			t.Errorf("expected 1 entry, got %d", len(result))
		}

		if result["foo"].Body != "second" {
			t.Errorf("expected last definition to win, got body: %s", result["foo"].Body)
		}
	})
}

// =============================================================================
// analyzeDefinitionChange Tests
// =============================================================================

func TestAnalyzeDefinitionChange(t *testing.T) {
	t.Run("no change returns nil", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "same body"}
		local := &Definition{Kind: "function", Body: "same body"}
		remote := &Definition{Kind: "function", Body: "same body"}

		result := analyzeDefinitionChange("test.py", "foo", base, local, remote)

		if result != nil {
			t.Errorf("expected nil for no change, got: %v", result)
		}
	})

	t.Run("added in both identical", func(t *testing.T) {
		local := &Definition{Kind: "function", Body: "new func"}
		remote := &Definition{Kind: "function", Body: "new func"}

		result := analyzeDefinitionChange("test.py", "foo", nil, local, remote)

		if result == nil {
			t.Fatal("expected conflict")
		}
		if result.Status != "Can Auto-merge" {
			t.Errorf("identical adds should auto-merge, got: %s", result.Status)
		}
	})

	t.Run("added in both different", func(t *testing.T) {
		local := &Definition{Kind: "function", Body: "local impl"}
		remote := &Definition{Kind: "function", Body: "remote impl"}

		result := analyzeDefinitionChange("test.py", "foo", nil, local, remote)

		if result == nil {
			t.Fatal("expected conflict")
		}
		if result.Status != "Needs Resolution" {
			t.Errorf("different adds need resolution, got: %s", result.Status)
		}
	})

	t.Run("deleted in both", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "original"}

		result := analyzeDefinitionChange("test.py", "foo", base, nil, nil)

		if result == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(result.ConflictType, "Deleted") {
			t.Errorf("should indicate deletion, got: %s", result.ConflictType)
		}
	})

	t.Run("delete/modify conflict", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "original"}
		remote := &Definition{Kind: "function", Body: "modified"}

		result := analyzeDefinitionChange("test.py", "foo", base, nil, remote)

		if result == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(result.ConflictType, "Delete/Modify") {
			t.Errorf("should indicate delete/modify, got: %s", result.ConflictType)
		}
	})

	t.Run("modified same in both", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "original"}
		local := &Definition{Kind: "function", Body: "same change"}
		remote := &Definition{Kind: "function", Body: "same change"}

		result := analyzeDefinitionChange("test.py", "foo", base, local, remote)

		if result == nil {
			t.Fatal("expected conflict")
		}
		if result.Status != "Can Auto-merge" {
			t.Errorf("same modifications should auto-merge, got: %s", result.Status)
		}
	})

	t.Run("formatted change", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "original"}
		local := &Definition{Kind: "function", Body: "modified  with  spaces"}
		remote := &Definition{Kind: "function", Body: "modified with spaces"}

		result := analyzeDefinitionChange("test.py", "foo", base, local, remote)

		if result == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(result.ConflictType, "Formatted") {
			t.Errorf("whitespace-only diff should be formatted, got: %s", result.ConflictType)
		}
		if result.Status != "Can Auto-merge" {
			t.Errorf("formatted changes should auto-merge, got: %s", result.Status)
		}
	})

	t.Run("modified differently", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "original"}
		local := &Definition{Kind: "function", Body: "local change"}
		remote := &Definition{Kind: "function", Body: "remote change"}

		result := analyzeDefinitionChange("test.py", "foo", base, local, remote)

		if result == nil {
			t.Fatal("expected conflict")
		}
		if result.Status != "Needs Resolution" {
			t.Errorf("different modifications need resolution, got: %s", result.Status)
		}
	})
}

// =============================================================================
// ParseFile Dispatch Tests
// =============================================================================

func TestParseFile_UnsupportedLanguage(t *testing.T) {
	content := []byte("some content")

	analysis := ParseFile(content, LangUnknown)

	if analysis.ParseError == nil {
		t.Error("expected parse error for unknown language")
	}
}

func TestParseFile_AllLanguages(t *testing.T) {
	// Test that ParseFile dispatches correctly for all supported languages
	tests := []struct {
		lang    Language
		content []byte
	}{
		{LangPython, []byte("def foo(): pass")},
		{LangJavaScript, []byte("function foo() {}")},
		{LangTypeScript, []byte("function foo(): void {}")},
		{LangGo, []byte("package main\nfunc foo() {}")},
		{LangRust, []byte("fn foo() {}")},
		{LangYAML, []byte("key: value")},
	}

	for _, tt := range tests {
		t.Run(tt.lang.String(), func(t *testing.T) {
			analysis := ParseFile(tt.content, tt.lang)
			if analysis.ParseError != nil {
				t.Errorf("parse error for %v: %v", tt.lang, analysis.ParseError)
			}
		})
	}
}

// String method for Language for test output
func (l Language) String() string {
	switch l {
	case LangPython:
		return "Python"
	case LangJavaScript:
		return "JavaScript"
	case LangTypeScript:
		return "TypeScript"
	case LangGo:
		return "Go"
	case LangRust:
		return "Rust"
	case LangYAML:
		return "YAML"
	default:
		return "Unknown"
	}
}

// =============================================================================
// DetectLanguage Additional Tests
// =============================================================================

func TestDetectLanguage_AllExtensions(t *testing.T) {
	tests := []struct {
		file     string
		expected Language
	}{
		// Python
		{"main.py", LangPython},
		{"script.PY", LangPython}, // uppercase

		// JavaScript
		{"app.js", LangJavaScript},
		{"module.mjs", LangJavaScript},
		{"common.cjs", LangJavaScript},
		{"component.jsx", LangJavaScript},

		// TypeScript
		{"app.ts", LangTypeScript},
		{"module.mts", LangTypeScript},
		{"common.cts", LangTypeScript},
		{"component.tsx", LangTypeScript},

		// Go
		{"main.go", LangGo},

		// Rust
		{"lib.rs", LangRust},

		// YAML/JSON
		{"config.yaml", LangYAML},
		{"config.yml", LangYAML},
		{"data.json", LangYAML},

		// Unknown
		{"README.md", LangUnknown},
		{"Makefile", LangUnknown},
		{"script.sh", LangUnknown},
		{"style.css", LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := DetectLanguage(tt.file)
			if result != tt.expected {
				t.Errorf("DetectLanguage(%q) = %v, want %v", tt.file, result, tt.expected)
			}
		})
	}
}
