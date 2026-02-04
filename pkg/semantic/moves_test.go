package semantic

import (
	"strings"
	"testing"

	"github.com/simonkoeck/g2/pkg/ui"
)

// Helper function to create a delete conflict
func makeDeleteConflict(name, body string, start, end uint32) SynthesisConflict {
	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "test.py",
			ConflictType: "Function '" + name + "' Deleted",
			Status:       "Needs Resolution",
		},
		Base: &Definition{
			Name:      name,
			Kind:      "function",
			Body:      body,
			StartByte: start,
			EndByte:   end,
		},
		Local:  nil,
		Remote: nil,
	}
}

// Helper function to create an add conflict (local side)
func makeAddConflict(name, body string, start, end uint32) SynthesisConflict {
	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "test.py",
			ConflictType: "Function '" + name + "' Added",
			Status:       "Needs Resolution",
		},
		Base: nil,
		Local: &Definition{
			Name:      name,
			Kind:      "function",
			Body:      body,
			StartByte: start,
			EndByte:   end,
		},
		Remote: nil,
	}
}

// Helper function to create an add conflict (remote side)
func makeRemoteAddConflict(name, body string, start, end uint32) SynthesisConflict {
	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "test.py",
			ConflictType: "Function '" + name + "' Added",
			Status:       "Needs Resolution",
		},
		Base:  nil,
		Local: nil,
		Remote: &Definition{
			Name:      name,
			Kind:      "function",
			Body:      body,
			StartByte: start,
			EndByte:   end,
		},
	}
}

// TestDetectMoves_ExactMove tests exact body match move detection
func TestDetectMoves_ExactMove(t *testing.T) {
	body := "def foo():\n    return calculate_something()\n    with_multiple_lines()"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", body, 0, 100),
		makeAddConflict("foo", body, 200, 300),
	}

	result := DetectMoves(conflicts)

	if len(result) != 1 {
		t.Fatalf("expected 1 conflict (merged move), got %d", len(result))
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "Moved") {
		t.Errorf("expected 'Moved' conflict type, got: %s", result[0].UIConflict.ConflictType)
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "Exact Match") {
		t.Errorf("expected 'Exact Match' in conflict type, got: %s", result[0].UIConflict.ConflictType)
	}

	if result[0].UIConflict.Status != "Can Auto-merge" {
		t.Errorf("move conflict should be auto-mergeable, got: %s", result[0].UIConflict.Status)
	}

	// Verify the conflict structure
	if result[0].Base == nil {
		t.Error("move conflict should have Base (source location)")
	}
	if result[0].Local == nil {
		t.Error("move conflict should have Local (destination)")
	}
}

// TestDetectMoves_FuzzyMatch tests fuzzy body matching
func TestDetectMoves_FuzzyMatch(t *testing.T) {
	// Bodies that are similar but not identical (>10 tokens for fuzzy matching)
	deleteBody := "def foo():\n    x = calculate_value(input_param)\n    y = process_data(x, config)\n    z = transform_result(y)\n    return x + y + z"
	addBody := "def foo():\n    x = calculate_value(input_param)\n    y = process_data(x, config)\n    z = transform_result(y)\n    return x * y * z" // changed + to *

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", deleteBody, 0, 100),
		makeAddConflict("foo", addBody, 200, 300),
	}

	result := DetectMoves(conflicts)

	if len(result) != 1 {
		t.Fatalf("expected 1 conflict (fuzzy match move), got %d", len(result))
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "Moved") {
		t.Errorf("expected 'Moved' conflict type, got: %s", result[0].UIConflict.ConflictType)
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "% Match") {
		t.Errorf("expected percentage match in conflict type, got: %s", result[0].UIConflict.ConflictType)
	}
}

// TestDetectMoves_BoilerplateSafety tests that small bodies are not matched
func TestDetectMoves_BoilerplateSafety(t *testing.T) {
	// Very small bodies that shouldn't be matched
	conflicts := []SynthesisConflict{
		makeDeleteConflict("x", "x = 1", 0, 5),
		makeAddConflict("y", "y = 2", 10, 15),
	}

	result := DetectMoves(conflicts)

	// Both should remain separate - too small for fuzzy matching
	if len(result) != 2 {
		t.Fatalf("expected 2 separate conflicts (boilerplate safety), got %d", len(result))
	}

	// Neither should be marked as moved
	for _, c := range result {
		if strings.Contains(c.UIConflict.ConflictType, "Moved") {
			t.Errorf("small body should not be matched as move: %s", c.UIConflict.ConflictType)
		}
	}
}

// TestDetectMoves_RenameAndMove tests detection of renamed functions with same body
func TestDetectMoves_RenameAndMove(t *testing.T) {
	body := "def function():\n    result = complex_calculation()\n    return transform(result)"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("oldName", body, 0, 100),
		makeAddConflict("newName", body, 200, 300),
	}

	result := DetectMoves(conflicts)

	if len(result) != 1 {
		t.Fatalf("expected 1 conflict (rename+move), got %d", len(result))
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "Renamed+Moved") {
		t.Errorf("expected 'Renamed+Moved' conflict type, got: %s", result[0].UIConflict.ConflictType)
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "oldName") {
		t.Errorf("should mention old name, got: %s", result[0].UIConflict.ConflictType)
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "newName") {
		t.Errorf("should mention new name, got: %s", result[0].UIConflict.ConflictType)
	}
}

// TestDetectMoves_MultipleMoves tests detection of multiple independent moves
func TestDetectMoves_MultipleMoves(t *testing.T) {
	body1 := "def foo():\n    return complex_logic_here()\n    with_multiple_statements()"
	body2 := "def bar():\n    return different_complex_logic()\n    also_multiple_lines()"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", body1, 0, 100),
		makeDeleteConflict("bar", body2, 110, 200),
		makeAddConflict("foo", body1, 300, 400),
		makeAddConflict("bar", body2, 410, 500),
	}

	result := DetectMoves(conflicts)

	if len(result) != 2 {
		t.Fatalf("expected 2 move conflicts, got %d", len(result))
	}

	moveCount := 0
	for _, c := range result {
		if strings.Contains(c.UIConflict.ConflictType, "Moved") {
			moveCount++
		}
	}

	if moveCount != 2 {
		t.Errorf("expected 2 moves, got %d", moveCount)
	}
}

// TestDetectMoves_UnmatchedOrphans tests that unmatched deletes/adds remain separate
func TestDetectMoves_UnmatchedOrphans(t *testing.T) {
	deleteBody := "def foo():\n    return completely_different_implementation()\n    nothing_similar()"
	addBody := "def bar():\n    return totally_unrelated_code()\n    with_different_logic()"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", deleteBody, 0, 100),
		makeAddConflict("bar", addBody, 200, 300),
	}

	result := DetectMoves(conflicts)

	// Should remain as 2 separate conflicts (no match)
	if len(result) != 2 {
		t.Fatalf("expected 2 separate conflicts (no match), got %d", len(result))
	}

	// Neither should be marked as moved
	for _, c := range result {
		if strings.Contains(c.UIConflict.ConflictType, "Moved") {
			t.Errorf("unmatched conflicts should not be marked as moved: %s", c.UIConflict.ConflictType)
		}
	}
}

// TestDetectMoves_NoOrphans tests that non-orphan conflicts pass through unchanged
func TestDetectMoves_NoOrphans(t *testing.T) {
	// Regular modified conflict (has base, local, and remote)
	modifiedConflict := SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "test.py",
			ConflictType: "Function 'foo' Modified",
			Status:       "Needs Resolution",
		},
		Base:   &Definition{Name: "foo", Kind: "function", Body: "def foo(): pass"},
		Local:  &Definition{Name: "foo", Kind: "function", Body: "def foo(): return 1"},
		Remote: &Definition{Name: "foo", Kind: "function", Body: "def foo(): return 2"},
	}

	conflicts := []SynthesisConflict{modifiedConflict}

	result := DetectMoves(conflicts)

	if len(result) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result))
	}

	if result[0].UIConflict.ConflictType != "Function 'foo' Modified" {
		t.Errorf("conflict should pass through unchanged, got: %s", result[0].UIConflict.ConflictType)
	}
}

// TestDetectMoves_KindMismatch tests that different kinds don't match
func TestDetectMoves_KindMismatch(t *testing.T) {
	body := "content that is the same\nwith multiple lines\nto ensure it's not too small"

	// Delete a function
	deleteConflict := SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "test.py",
			ConflictType: "Function 'foo' Deleted",
			Status:       "Needs Resolution",
		},
		Base: &Definition{
			Name: "foo",
			Kind: "function", // function
			Body: body,
		},
	}

	// Add a class with same body
	addConflict := SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "test.py",
			ConflictType: "Class 'foo' Added",
			Status:       "Needs Resolution",
		},
		Local: &Definition{
			Name: "foo",
			Kind: "class", // class - different kind!
			Body: body,
		},
	}

	conflicts := []SynthesisConflict{deleteConflict, addConflict}

	result := DetectMoves(conflicts)

	// Should remain as 2 separate conflicts (kind mismatch)
	if len(result) != 2 {
		t.Fatalf("expected 2 separate conflicts (kind mismatch), got %d", len(result))
	}
}

// TestDetectMoves_RemoteAdd tests move detection with remote-side adds
func TestDetectMoves_RemoteAdd(t *testing.T) {
	body := "def foo():\n    return calculate_something()\n    with_multiple_lines()"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", body, 0, 100),
		makeRemoteAddConflict("foo", body, 200, 300),
	}

	result := DetectMoves(conflicts)

	if len(result) != 1 {
		t.Fatalf("expected 1 conflict (move with remote add), got %d", len(result))
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "Moved") {
		t.Errorf("expected 'Moved' conflict type, got: %s", result[0].UIConflict.ConflictType)
	}

	// Verify the destination is from remote
	if result[0].Remote == nil {
		t.Error("move conflict should have Remote set (destination)")
	}
}

// TestDetectMoves_EmptyConflicts tests handling of empty conflict list
func TestDetectMoves_EmptyConflicts(t *testing.T) {
	result := DetectMoves([]SynthesisConflict{})

	if len(result) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(result))
	}
}

// TestDetectMoves_SingleConflict tests handling of single conflict
func TestDetectMoves_SingleConflict(t *testing.T) {
	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", "def foo(): pass", 0, 15),
	}

	result := DetectMoves(conflicts)

	if len(result) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(result))
	}
}

// TestDetectMoves_WhitespaceNormalization tests that whitespace differences don't prevent matching
func TestDetectMoves_WhitespaceNormalization(t *testing.T) {
	// Same code, different formatting
	deleteBody := "def foo():\n    return   value\n    pass"
	addBody := "def foo():\n  return value\n  pass" // different indentation

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", deleteBody, 0, 50),
		makeAddConflict("foo", addBody, 100, 150),
	}

	result := DetectMoves(conflicts)

	if len(result) != 1 {
		t.Fatalf("expected 1 conflict (whitespace normalized), got %d", len(result))
	}

	if !strings.Contains(result[0].UIConflict.ConflictType, "Exact Match") {
		t.Errorf("whitespace-only difference should be exact match, got: %s", result[0].UIConflict.ConflictType)
	}
}

// TestDetectMoves_PreservesOtherConflicts tests that non-orphan conflicts are preserved
func TestDetectMoves_PreservesOtherConflicts(t *testing.T) {
	moveBody := "def moved():\n    return complex_implementation()\n    with_multiple_lines()"

	conflicts := []SynthesisConflict{
		// A regular modified conflict
		{
			UIConflict: ui.Conflict{
				File:         "test.py",
				ConflictType: "Function 'existing' Modified",
				Status:       "Needs Resolution",
			},
			Base:   &Definition{Name: "existing", Kind: "function", Body: "old"},
			Local:  &Definition{Name: "existing", Kind: "function", Body: "new1"},
			Remote: &Definition{Name: "existing", Kind: "function", Body: "new2"},
		},
		// A move pair
		makeDeleteConflict("moved", moveBody, 0, 100),
		makeAddConflict("moved", moveBody, 200, 300),
	}

	result := DetectMoves(conflicts)

	// Should have 2: the preserved modified conflict + the merged move
	if len(result) != 2 {
		t.Fatalf("expected 2 conflicts (1 preserved + 1 move), got %d", len(result))
	}

	// Check we have both types
	hasModified := false
	hasMoved := false
	for _, c := range result {
		if strings.Contains(c.UIConflict.ConflictType, "Modified") {
			hasModified = true
		}
		if strings.Contains(c.UIConflict.ConflictType, "Moved") {
			hasMoved = true
		}
	}

	if !hasModified {
		t.Error("should preserve modified conflict")
	}
	if !hasMoved {
		t.Error("should have move conflict")
	}
}

// TestIsOrphanDelete tests the orphan delete detection
func TestIsOrphanDelete(t *testing.T) {
	tests := []struct {
		name     string
		conflict SynthesisConflict
		expected bool
	}{
		{
			name: "true orphan delete",
			conflict: SynthesisConflict{
				Base:   &Definition{Name: "foo"},
				Local:  nil,
				Remote: nil,
			},
			expected: true,
		},
		{
			name: "has local - not orphan",
			conflict: SynthesisConflict{
				Base:   &Definition{Name: "foo"},
				Local:  &Definition{Name: "foo"},
				Remote: nil,
			},
			expected: false,
		},
		{
			name: "has remote - not orphan",
			conflict: SynthesisConflict{
				Base:   &Definition{Name: "foo"},
				Local:  nil,
				Remote: &Definition{Name: "foo"},
			},
			expected: false,
		},
		{
			name: "no base - not orphan delete",
			conflict: SynthesisConflict{
				Base:   nil,
				Local:  nil,
				Remote: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOrphanDelete(&tt.conflict)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsOrphanAdd tests the orphan add detection
func TestIsOrphanAdd(t *testing.T) {
	tests := []struct {
		name     string
		conflict SynthesisConflict
		expected bool
	}{
		{
			name: "local add",
			conflict: SynthesisConflict{
				Base:   nil,
				Local:  &Definition{Name: "foo"},
				Remote: nil,
			},
			expected: true,
		},
		{
			name: "remote add",
			conflict: SynthesisConflict{
				Base:   nil,
				Local:  nil,
				Remote: &Definition{Name: "foo"},
			},
			expected: true,
		},
		{
			name: "both add",
			conflict: SynthesisConflict{
				Base:   nil,
				Local:  &Definition{Name: "foo"},
				Remote: &Definition{Name: "foo"},
			},
			expected: true,
		},
		{
			name: "has base - not orphan add",
			conflict: SynthesisConflict{
				Base:   &Definition{Name: "foo"},
				Local:  &Definition{Name: "foo"},
				Remote: nil,
			},
			expected: false,
		},
		{
			name: "nothing - not orphan add",
			conflict: SynthesisConflict{
				Base:   nil,
				Local:  nil,
				Remote: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOrphanAdd(&tt.conflict)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestDetectMovesWithConfig_DisableExactMatch tests disabling exact matching
func TestDetectMovesWithConfig_DisableExactMatch(t *testing.T) {
	// Body with >10 tokens to pass fuzzy matching threshold
	body := "def foo():\n    result = calculate_something(input_param)\n    processed = transform_data(result, config)\n    return finalize_output(processed)"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", body, 0, 100),
		makeAddConflict("foo", body, 200, 300),
	}

	config := DefaultMoveDetectionConfig()
	config.EnableExactMatch = false

	result := DetectMovesWithConfig(conflicts, config)

	// Should still match via fuzzy (since bodies are identical, Jaccard = 1.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result))
	}

	// Should be fuzzy match since exact is disabled
	if strings.Contains(result[0].UIConflict.ConflictType, "Exact Match") {
		t.Error("should not use exact match when disabled")
	}
}

// TestDetectMovesWithConfig_DisableFuzzyMatch tests disabling fuzzy matching
func TestDetectMovesWithConfig_DisableFuzzyMatch(t *testing.T) {
	// Bodies that are similar but not identical
	deleteBody := "def foo():\n    x = calculate_value()\n    y = process_data()\n    return x + y"
	addBody := "def foo():\n    x = calculate_value()\n    y = process_data()\n    return x * y"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", deleteBody, 0, 100),
		makeAddConflict("foo", addBody, 200, 300),
	}

	config := DefaultMoveDetectionConfig()
	config.EnableFuzzyMatch = false

	result := DetectMovesWithConfig(conflicts, config)

	// Should remain separate since exact match won't work and fuzzy is disabled
	if len(result) != 2 {
		t.Fatalf("expected 2 conflicts (fuzzy disabled), got %d", len(result))
	}
}

// TestDetectMovesWithConfig_CustomThreshold tests custom fuzzy threshold
func TestDetectMovesWithConfig_CustomThreshold(t *testing.T) {
	// Bodies with ~50% token overlap
	deleteBody := "def foo():\n    a = 1\n    b = 2\n    c = 3\n    d = 4"
	addBody := "def foo():\n    a = 1\n    b = 2\n    e = 5\n    f = 6"

	conflicts := []SynthesisConflict{
		makeDeleteConflict("foo", deleteBody, 0, 100),
		makeAddConflict("foo", addBody, 200, 300),
	}

	t.Run("low threshold - matches", func(t *testing.T) {
		config := DefaultMoveDetectionConfig()
		config.FuzzyThreshold = 0.4

		result := DetectMovesWithConfig(conflicts, config)

		if len(result) != 1 {
			t.Errorf("expected 1 conflict with low threshold, got %d", len(result))
		}
	})

	t.Run("high threshold - no match", func(t *testing.T) {
		config := DefaultMoveDetectionConfig()
		config.FuzzyThreshold = 0.9

		result := DetectMovesWithConfig(conflicts, config)

		if len(result) != 2 {
			t.Errorf("expected 2 conflicts with high threshold, got %d", len(result))
		}
	})
}

// BenchmarkDetectMoves_LargeFile benchmarks move detection with many conflicts
func BenchmarkDetectMoves_LargeFile(b *testing.B) {
	// Create 50 conflicts (25 deletes, 25 adds, 10 matching pairs)
	var conflicts []SynthesisConflict

	for i := 0; i < 25; i++ {
		body := strings.Repeat("def func"+string(rune('A'+i))+"(): return value\n", 5)
		conflicts = append(conflicts, makeDeleteConflict("func"+string(rune('A'+i)), body, uint32(i*100), uint32(i*100+50)))
	}

	for i := 0; i < 10; i++ {
		body := strings.Repeat("def func"+string(rune('A'+i))+"(): return value\n", 5)
		conflicts = append(conflicts, makeAddConflict("func"+string(rune('A'+i)), body, uint32(3000+i*100), uint32(3000+i*100+50)))
	}

	for i := 10; i < 25; i++ {
		body := strings.Repeat("def different"+string(rune('A'+i))+"(): return other\n", 5)
		conflicts = append(conflicts, makeAddConflict("different"+string(rune('A'+i)), body, uint32(3000+i*100), uint32(3000+i*100+50)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectMoves(conflicts)
	}
}
