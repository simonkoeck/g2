package semantic

import (
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
