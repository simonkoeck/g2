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
