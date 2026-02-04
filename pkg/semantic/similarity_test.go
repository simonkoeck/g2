package semantic

import (
	"testing"
)

// TestHashBody tests body hashing
func TestHashBody(t *testing.T) {
	t.Run("same content - same hash", func(t *testing.T) {
		hash1 := hashBody("def foo(): pass")
		hash2 := hashBody("def foo(): pass")
		if hash1 != hash2 {
			t.Error("same content should produce same hash")
		}
	})

	t.Run("different content - different hash", func(t *testing.T) {
		hash1 := hashBody("def foo(): pass")
		hash2 := hashBody("def bar(): pass")
		if hash1 == hash2 {
			t.Error("different content should produce different hash")
		}
	})

	t.Run("whitespace normalized", func(t *testing.T) {
		hash1 := hashBody("def foo():    pass")
		hash2 := hashBody("def foo(): pass")
		if hash1 != hash2 {
			t.Error("whitespace differences should be normalized")
		}
	})
}

// TestTokenize tests tokenization
func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"foo bar", []string{"foo", "bar"}},
		{"foo_bar", []string{"foo_bar"}},
		{"def foo(): return 1", []string{"def", "foo", "return", "1"}},
		{"x = y + z", []string{"x", "y", "z"}},
		{"", []string{}},
		{"   ", []string{}},
		{"CamelCase", []string{"CamelCase"}},
		{"snake_case_name", []string{"snake_case_name"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := tokenize(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, tok := range result {
				if tok != tt.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tt.expected[i], tok)
				}
			}
		})
	}
}

// TestCalculateJaccard tests Jaccard similarity calculation
func TestCalculateJaccard(t *testing.T) {
	tests := []struct {
		name     string
		tokens1  []string
		tokens2  []string
		expected float64
	}{
		{
			name:     "identical",
			tokens1:  []string{"a", "b", "c"},
			tokens2:  []string{"a", "b", "c"},
			expected: 1.0,
		},
		{
			name:     "no overlap",
			tokens1:  []string{"a", "b"},
			tokens2:  []string{"c", "d"},
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			tokens1:  []string{"a", "b", "c"},
			tokens2:  []string{"b", "c", "d"},
			expected: 0.5, // 2 common / 4 total
		},
		{
			name:     "both empty",
			tokens1:  []string{},
			tokens2:  []string{},
			expected: 1.0,
		},
		{
			name:     "one empty",
			tokens1:  []string{"a"},
			tokens2:  []string{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateJaccard(tt.tokens1, tt.tokens2)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

// BenchmarkTokenize benchmarks tokenization performance
func BenchmarkTokenize(b *testing.B) {
	input := `def complex_function():
    result = calculate_something()
    processed = transform_data(result)
    return finalize_output(processed)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenize(input)
	}
}

// BenchmarkCalculateJaccard benchmarks Jaccard calculation
func BenchmarkCalculateJaccard(b *testing.B) {
	tokens1 := tokenize(`def foo(): return calculate_something() with multiple tokens here`)
	tokens2 := tokenize(`def foo(): return calculate_something() with different tokens here`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateJaccard(tokens1, tokens2)
	}
}

// TestStripPythonComments tests Python comment stripping
func TestStripPythonComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line comment",
			input:    "x = 1  # this is a comment",
			expected: "x = 1  ",
		},
		{
			name:     "full line comment",
			input:    "# comment\nx = 1",
			expected: "\nx = 1",
		},
		{
			name:     "multiple comments",
			input:    "x = 1  # first\ny = 2  # second",
			expected: "x = 1  \ny = 2  ",
		},
		{
			name:     "no comments",
			input:    "x = 1\ny = 2",
			expected: "x = 1\ny = 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripPythonComments(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestStripCStyleComments tests C-style comment stripping
func TestStripCStyleComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line comment",
			input:    "int x = 1; // comment",
			expected: "int x = 1; ",
		},
		{
			name:     "multi-line comment",
			input:    "int x /* comment */ = 1;",
			expected: "int x  = 1;",
		},
		{
			name:     "multi-line block",
			input:    "int x = 1;\n/* multi\nline\ncomment */\nint y = 2;",
			expected: "int x = 1;\n\nint y = 2;",
		},
		{
			name:     "mixed comments",
			input:    "int x = 1; // inline\n/* block */ int y = 2;",
			expected: "int x = 1; \n int y = 2;",
		},
		{
			name:     "no comments",
			input:    "int x = 1;\nint y = 2;",
			expected: "int x = 1;\nint y = 2;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripCStyleComments(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestNormalizeForLanguage tests language-aware normalization
func TestNormalizeForLanguage(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		lang     Language
		expected string
	}{
		{
			name:     "python with comment",
			body:     "def foo():  # comment\n    return 1",
			lang:     LangPython,
			expected: "def foo(): return 1",
		},
		{
			name:     "go with comment",
			body:     "func foo() { // comment\n    return 1\n}",
			lang:     LangGo,
			expected: "func foo() { return 1 }",
		},
		{
			name:     "js with semicolons",
			body:     "function foo() { return 1; }",
			lang:     LangJavaScript,
			expected: "function foo() { return 1 }",
		},
		{
			name:     "rust with comment",
			body:     "fn foo() -> i32 { // returns 1\n    1\n}",
			lang:     LangRust,
			expected: "fn foo() -> i32 { 1 }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeForLanguage(tt.body, tt.lang)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestStripCommentsPreservesCode tests that code is preserved after stripping
func TestStripCommentsPreservesCode(t *testing.T) {
	// Two functions that differ only by comments should normalize to the same thing
	goCode1 := `func calculate(x int) int {
    // Add one to x
    return x + 1
}`
	goCode2 := `func calculate(x int) int {
    /* Different comment style */
    return x + 1
}`

	norm1 := normalizeForLanguage(goCode1, LangGo)
	norm2 := normalizeForLanguage(goCode2, LangGo)

	if norm1 != norm2 {
		t.Errorf("code with different comments should normalize the same:\n  %q\n  %q", norm1, norm2)
	}
}
