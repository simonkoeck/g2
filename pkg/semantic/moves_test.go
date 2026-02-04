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
		// Also set tiered thresholds low so small bodies use low threshold
		config.SmallBodyThreshold = 0.4
		config.LargeBodyThreshold = 0.4

		result := DetectMovesWithConfig(conflicts, config)

		if len(result) != 1 {
			t.Errorf("expected 1 conflict with low threshold, got %d", len(result))
		}
	})

	t.Run("high threshold - no match", func(t *testing.T) {
		config := DefaultMoveDetectionConfig()
		config.FuzzyThreshold = 0.9
		// Also set tiered thresholds high
		config.SmallBodyThreshold = 0.9
		config.LargeBodyThreshold = 0.9

		result := DetectMovesWithConfig(conflicts, config)

		if len(result) != 2 {
			t.Errorf("expected 2 conflicts with high threshold, got %d", len(result))
		}
	})
}

// TestTieredThresholds tests that different body sizes use appropriate thresholds
func TestTieredThresholds(t *testing.T) {
	config := MoveDetectionConfig{
		MinTokenCount:      5,
		FuzzyThreshold:     0.75,
		EnableExactMatch:   true,
		EnableFuzzyMatch:   true,
		SmallBodyTokens:    15,
		SmallBodyThreshold: 0.90, // Strict for small
		LargeBodyTokens:    50,
		LargeBodyThreshold: 0.60, // Lenient for large
	}

	t.Run("small body uses strict threshold", func(t *testing.T) {
		threshold := getThresholdForSize(10, config) // Below SmallBodyTokens
		if threshold != 0.90 {
			t.Errorf("expected 0.90 for small body, got %f", threshold)
		}
	})

	t.Run("medium body uses default threshold", func(t *testing.T) {
		threshold := getThresholdForSize(30, config) // Between small and large
		if threshold != 0.75 {
			t.Errorf("expected 0.75 for medium body, got %f", threshold)
		}
	})

	t.Run("large body uses lenient threshold", func(t *testing.T) {
		threshold := getThresholdForSize(60, config) // Above LargeBodyTokens
		if threshold != 0.60 {
			t.Errorf("expected 0.60 for large body, got %f", threshold)
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

// =============================================================================
// Inter-File Move Detection Tests
// =============================================================================

// Helper to create a SynthesisAnalysis for testing
func makeAnalysis(file string, conflicts []SynthesisConflict) *SynthesisAnalysis {
	return &SynthesisAnalysis{
		File:      file,
		Conflicts: conflicts,
	}
}

// Helper to create a delete conflict for a specific file
func makeDeleteConflictForFile(file, name, body string) SynthesisConflict {
	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         file,
			ConflictType: "Function '" + name + "' Deleted",
			Status:       "Needs Resolution",
		},
		Base: &Definition{
			Name: name,
			Kind: "function",
			Body: body,
		},
		Local:  nil,
		Remote: nil,
	}
}

// Helper to create a local add conflict for a specific file
func makeAddConflictForFile(file, name, body string) SynthesisConflict {
	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         file,
			ConflictType: "Function '" + name + "' Added (local)",
			Status:       "Needs Resolution",
		},
		Base: nil,
		Local: &Definition{
			Name: name,
			Kind: "function",
			Body: body,
		},
		Remote: nil,
	}
}

// Helper to create a remote add conflict for a specific file
func makeRemoteAddConflictForFile(file, name, body string) SynthesisConflict {
	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         file,
			ConflictType: "Function '" + name + "' Added (remote)",
			Status:       "Needs Resolution",
		},
		Base:  nil,
		Local: nil,
		Remote: &Definition{
			Name: name,
			Kind: "function",
			Body: body,
		},
	}
}

// TestDetectInterFileMoves_ExactMatch tests exact body match across files
func TestDetectInterFileMoves_ExactMatch(t *testing.T) {
	body := "def helper():\n    return calculate_something()\n    with_multiple_lines()"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", body),
		}),
		makeAnalysis("newutils.py", []SynthesisConflict{
			makeAddConflictForFile("newutils.py", "helper", body),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 1 {
		t.Fatalf("expected 1 inter-file move, got %d", len(moves))
	}

	move := moves[0]
	if move.SourceFile != "utils.py" {
		t.Errorf("expected source file 'utils.py', got '%s'", move.SourceFile)
	}
	if move.DestFile != "newutils.py" {
		t.Errorf("expected dest file 'newutils.py', got '%s'", move.DestFile)
	}
	if move.MatchType != "Exact Match" {
		t.Errorf("expected 'Exact Match', got '%s'", move.MatchType)
	}
	if move.Similarity != 1.0 {
		t.Errorf("expected similarity 1.0, got %f", move.Similarity)
	}
}

// TestDetectInterFileMoves_FuzzyMatch tests fuzzy body match across files
func TestDetectInterFileMoves_FuzzyMatch(t *testing.T) {
	deleteBody := "def calculate_total(items):\n    total = 0\n    for item in items:\n        total += item.price\n    return total"
	addBody := "def calculate_total(items):\n    total = 0\n    for item in items:\n        total += item.price\n    return total * 1.1" // added tax

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "calculate_total", deleteBody),
		}),
		makeAnalysis("pricing.py", []SynthesisConflict{
			makeAddConflictForFile("pricing.py", "calculate_total", addBody),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 1 {
		t.Fatalf("expected 1 inter-file move, got %d", len(moves))
	}

	move := moves[0]
	if move.MatchType != "Fuzzy Match" {
		t.Errorf("expected 'Fuzzy Match', got '%s'", move.MatchType)
	}
	if move.Similarity < 0.75 || move.Similarity >= 1.0 {
		t.Errorf("expected similarity between 0.75 and 1.0, got %f", move.Similarity)
	}
}

// TestDetectInterFileMoves_SameFileNotMatched tests that same-file orphans are not matched
func TestDetectInterFileMoves_SameFileNotMatched(t *testing.T) {
	body := "def helper():\n    return calculate_something()\n    with_multiple_lines()"

	// Both delete and add in the same file - should NOT be matched by inter-file detection
	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", body),
			makeAddConflictForFile("utils.py", "helper", body),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 0 {
		t.Errorf("expected 0 inter-file moves (same file), got %d", len(moves))
	}
}

// TestDetectInterFileMoves_KindMismatch tests that different kinds don't match across files
func TestDetectInterFileMoves_KindMismatch(t *testing.T) {
	body := "content that is the same\nwith multiple lines\nto ensure matching"

	// Delete a function in one file
	deleteConflict := SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "utils.py",
			ConflictType: "Function 'foo' Deleted",
			Status:       "Needs Resolution",
		},
		Base: &Definition{
			Name: "foo",
			Kind: "function",
			Body: body,
		},
	}

	// Add a class with same body in another file
	addConflict := SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "models.py",
			ConflictType: "Class 'foo' Added",
			Status:       "Needs Resolution",
		},
		Local: &Definition{
			Name: "foo",
			Kind: "class", // different kind
			Body: body,
		},
	}

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{deleteConflict}),
		makeAnalysis("models.py", []SynthesisConflict{addConflict}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 0 {
		t.Errorf("expected 0 moves (kind mismatch), got %d", len(moves))
	}
}

// TestDetectInterFileMoves_MultipleFiles tests moves across multiple files
func TestDetectInterFileMoves_MultipleFiles(t *testing.T) {
	body1 := "def helper1():\n    return first_implementation()\n    with_multiple_lines()"
	body2 := "def helper2():\n    return second_implementation()\n    also_multiple_lines()"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("old_utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("old_utils.py", "helper1", body1),
			makeDeleteConflictForFile("old_utils.py", "helper2", body2),
		}),
		makeAnalysis("new_utils1.py", []SynthesisConflict{
			makeAddConflictForFile("new_utils1.py", "helper1", body1),
		}),
		makeAnalysis("new_utils2.py", []SynthesisConflict{
			makeAddConflictForFile("new_utils2.py", "helper2", body2),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 2 {
		t.Fatalf("expected 2 inter-file moves, got %d", len(moves))
	}

	// Verify both moves are detected
	foundMove1 := false
	foundMove2 := false
	for _, move := range moves {
		if move.SourceFile == "old_utils.py" && move.DestFile == "new_utils1.py" {
			foundMove1 = true
		}
		if move.SourceFile == "old_utils.py" && move.DestFile == "new_utils2.py" {
			foundMove2 = true
		}
	}

	if !foundMove1 {
		t.Error("expected move from old_utils.py to new_utils1.py")
	}
	if !foundMove2 {
		t.Error("expected move from old_utils.py to new_utils2.py")
	}
}

// TestDetectInterFileMoves_NoOrphans tests when there are no orphan conflicts
func TestDetectInterFileMoves_NoOrphans(t *testing.T) {
	// Regular modified conflict (has base, local, and remote) - not an orphan
	modifiedConflict := SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "utils.py",
			ConflictType: "Function 'foo' Modified",
			Status:       "Needs Resolution",
		},
		Base:   &Definition{Name: "foo", Kind: "function", Body: "def foo(): pass"},
		Local:  &Definition{Name: "foo", Kind: "function", Body: "def foo(): return 1"},
		Remote: &Definition{Name: "foo", Kind: "function", Body: "def foo(): return 2"},
	}

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{modifiedConflict}),
		makeAnalysis("other.py", []SynthesisConflict{modifiedConflict}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 0 {
		t.Errorf("expected 0 moves (no orphans), got %d", len(moves))
	}
}

// TestDetectInterFileMoves_EmptyAnalyses tests empty input
func TestDetectInterFileMoves_EmptyAnalyses(t *testing.T) {
	moves := DetectInterFileMoves([]*SynthesisAnalysis{})

	if len(moves) != 0 {
		t.Errorf("expected 0 moves for empty input, got %d", len(moves))
	}
}

// TestDetectInterFileMoves_SingleFile tests single file (no inter-file possible)
func TestDetectInterFileMoves_SingleFile(t *testing.T) {
	body := "def helper():\n    return something()\n    multiple_lines()"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", body),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 0 {
		t.Errorf("expected 0 moves for single file, got %d", len(moves))
	}
}

// TestDetectInterFileMoves_OnlyDeletes tests when there are only deletes (no adds)
func TestDetectInterFileMoves_OnlyDeletes(t *testing.T) {
	body := "def helper():\n    return something()\n    multiple_lines()"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", body),
		}),
		makeAnalysis("other.py", []SynthesisConflict{
			makeDeleteConflictForFile("other.py", "other_func", body),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 0 {
		t.Errorf("expected 0 moves (only deletes), got %d", len(moves))
	}
}

// TestDetectInterFileMoves_OnlyAdds tests when there are only adds (no deletes)
func TestDetectInterFileMoves_OnlyAdds(t *testing.T) {
	body := "def helper():\n    return something()\n    multiple_lines()"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeAddConflictForFile("utils.py", "helper", body),
		}),
		makeAnalysis("other.py", []SynthesisConflict{
			makeAddConflictForFile("other.py", "other_func", body),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 0 {
		t.Errorf("expected 0 moves (only adds), got %d", len(moves))
	}
}

// TestDetectInterFileMoves_RemoteAdd tests inter-file move with remote-side add
func TestDetectInterFileMoves_RemoteAdd(t *testing.T) {
	body := "def helper():\n    return calculate_something()\n    with_multiple_lines()"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", body),
		}),
		makeAnalysis("newutils.py", []SynthesisConflict{
			makeRemoteAddConflictForFile("newutils.py", "helper", body),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 1 {
		t.Fatalf("expected 1 inter-file move, got %d", len(moves))
	}

	move := moves[0]
	if move.DestConflict.Remote == nil {
		t.Error("expected dest conflict to have Remote set")
	}
}

// TestDetectInterFileMoves_BoilerplateSafety tests that small bodies don't match
func TestDetectInterFileMoves_BoilerplateSafety(t *testing.T) {
	// Very small bodies that shouldn't be fuzzy matched
	analyses := []*SynthesisAnalysis{
		makeAnalysis("a.py", []SynthesisConflict{
			makeDeleteConflictForFile("a.py", "x", "x = 1"),
		}),
		makeAnalysis("b.py", []SynthesisConflict{
			makeAddConflictForFile("b.py", "y", "y = 2"),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	// Small bodies shouldn't match via fuzzy, and they're different so no exact match
	if len(moves) != 0 {
		t.Errorf("expected 0 moves (boilerplate safety), got %d", len(moves))
	}
}

// TestApplyInterFileMoves_UpdatesConflicts tests that ApplyInterFileMoves updates conflicts correctly
func TestApplyInterFileMoves_UpdatesConflicts(t *testing.T) {
	body := "def helper():\n    return something()\n    multiple_lines()"

	sourceConflict := makeDeleteConflictForFile("utils.py", "helper", body)
	destConflict := makeAddConflictForFile("newutils.py", "helper", body)

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{sourceConflict}),
		makeAnalysis("newutils.py", []SynthesisConflict{destConflict}),
	}

	moves := DetectInterFileMoves(analyses)
	ApplyInterFileMoves(analyses, moves)

	// Check source conflict was updated
	srcConflict := &analyses[0].Conflicts[0]
	if srcConflict.UIConflict.Status != "Can Auto-merge" {
		t.Errorf("source conflict status should be 'Can Auto-merge', got '%s'", srcConflict.UIConflict.Status)
	}
	if !strings.Contains(srcConflict.UIConflict.ConflictType, "Moved to newutils.py") {
		t.Errorf("source conflict type should mention dest file, got '%s'", srcConflict.UIConflict.ConflictType)
	}
	if !strings.Contains(srcConflict.UIConflict.ConflictType, "Exact Match") {
		t.Errorf("source conflict type should mention match type, got '%s'", srcConflict.UIConflict.ConflictType)
	}

	// Check dest conflict was updated
	dstConflict := &analyses[1].Conflicts[0]
	if dstConflict.UIConflict.Status != "Can Auto-merge" {
		t.Errorf("dest conflict status should be 'Can Auto-merge', got '%s'", dstConflict.UIConflict.Status)
	}
	if !strings.Contains(dstConflict.UIConflict.ConflictType, "Moved from utils.py") {
		t.Errorf("dest conflict type should mention source file, got '%s'", dstConflict.UIConflict.ConflictType)
	}
}

// TestApplyInterFileMoves_FuzzyMatchPercentage tests fuzzy match percentage in conflict type
func TestApplyInterFileMoves_FuzzyMatchPercentage(t *testing.T) {
	deleteBody := "def calculate_total(items):\n    total = 0\n    for item in items:\n        total += item.price\n    return total"
	addBody := "def calculate_total(items):\n    total = 0\n    for item in items:\n        total += item.price\n    return total * 1.1"

	sourceConflict := makeDeleteConflictForFile("utils.py", "calculate_total", deleteBody)
	destConflict := makeAddConflictForFile("pricing.py", "calculate_total", addBody)

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{sourceConflict}),
		makeAnalysis("pricing.py", []SynthesisConflict{destConflict}),
	}

	moves := DetectInterFileMoves(analyses)
	ApplyInterFileMoves(analyses, moves)

	// Check that percentage is shown for fuzzy match
	srcConflict := &analyses[0].Conflicts[0]
	if !strings.Contains(srcConflict.UIConflict.ConflictType, "% Match") {
		t.Errorf("fuzzy match should show percentage, got '%s'", srcConflict.UIConflict.ConflictType)
	}
}

// TestApplyInterFileMoves_EmptyMoves tests applying empty moves list
func TestApplyInterFileMoves_EmptyMoves(t *testing.T) {
	body := "def helper(): pass"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", body),
		}),
	}

	originalStatus := analyses[0].Conflicts[0].UIConflict.Status

	ApplyInterFileMoves(analyses, []InterFileMove{})

	// Conflicts should be unchanged
	if analyses[0].Conflicts[0].UIConflict.Status != originalStatus {
		t.Error("conflict should be unchanged when no moves applied")
	}
}

// TestDetectInterFileMoves_MixedConflictTypes tests with mix of orphans and non-orphans
func TestDetectInterFileMoves_MixedConflictTypes(t *testing.T) {
	moveBody := "def helper():\n    return calculate_something()\n    with_multiple_lines()"

	// Modified conflict (not an orphan)
	modifiedConflict := SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "utils.py",
			ConflictType: "Function 'other' Modified",
			Status:       "Needs Resolution",
		},
		Base:   &Definition{Name: "other", Kind: "function", Body: "old"},
		Local:  &Definition{Name: "other", Kind: "function", Body: "new1"},
		Remote: &Definition{Name: "other", Kind: "function", Body: "new2"},
	}

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			modifiedConflict,
			makeDeleteConflictForFile("utils.py", "helper", moveBody),
		}),
		makeAnalysis("newutils.py", []SynthesisConflict{
			makeAddConflictForFile("newutils.py", "helper", moveBody),
		}),
	}

	moves := DetectInterFileMoves(analyses)

	if len(moves) != 1 {
		t.Fatalf("expected 1 inter-file move, got %d", len(moves))
	}

	// Should only match the orphans, not the modified conflict
	if moves[0].SourceConflict.Base.Name != "helper" {
		t.Errorf("should match 'helper', not '%s'", moves[0].SourceConflict.Base.Name)
	}
}

// TestDetectInterFileMovesWithConfig_DisableExact tests disabling exact matching
func TestDetectInterFileMovesWithConfig_DisableExact(t *testing.T) {
	// Body needs >10 tokens for fuzzy matching to work
	body := "def helper():\n    result = calculate_something(input_param)\n    processed = transform_data(result, config)\n    validated = check_output(processed)\n    return finalize_result(validated)"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", body),
		}),
		makeAnalysis("newutils.py", []SynthesisConflict{
			makeAddConflictForFile("newutils.py", "helper", body),
		}),
	}

	config := DefaultMoveDetectionConfig()
	config.EnableExactMatch = false

	moves := DetectInterFileMovesWithConfig(analyses, config)

	// Should still match via fuzzy (identical bodies = 100% similarity)
	if len(moves) != 1 {
		t.Fatalf("expected 1 move via fuzzy, got %d", len(moves))
	}

	// Should be marked as fuzzy match
	if moves[0].MatchType == "Exact Match" {
		t.Error("should use fuzzy match when exact is disabled")
	}
}

// TestDetectInterFileMovesWithConfig_DisableFuzzy tests disabling fuzzy matching
func TestDetectInterFileMovesWithConfig_DisableFuzzy(t *testing.T) {
	deleteBody := "def helper():\n    return calculate_something()\n    with_multiple_lines()"
	addBody := "def helper():\n    return calculate_something_else()\n    with_different_lines()"

	analyses := []*SynthesisAnalysis{
		makeAnalysis("utils.py", []SynthesisConflict{
			makeDeleteConflictForFile("utils.py", "helper", deleteBody),
		}),
		makeAnalysis("newutils.py", []SynthesisConflict{
			makeAddConflictForFile("newutils.py", "helper", addBody),
		}),
	}

	config := DefaultMoveDetectionConfig()
	config.EnableFuzzyMatch = false

	moves := DetectInterFileMovesWithConfig(analyses, config)

	// Bodies are different so exact won't match, and fuzzy is disabled
	if len(moves) != 0 {
		t.Errorf("expected 0 moves (fuzzy disabled), got %d", len(moves))
	}
}

// BenchmarkDetectInterFileMoves benchmarks inter-file move detection
func BenchmarkDetectInterFileMoves(b *testing.B) {
	// Create 10 files with 5 orphan conflicts each
	var analyses []*SynthesisAnalysis

	for i := 0; i < 5; i++ {
		var conflicts []SynthesisConflict
		for j := 0; j < 5; j++ {
			body := strings.Repeat("def func_"+string(rune('A'+j))+"(): return value\n", 5)
			conflicts = append(conflicts, makeDeleteConflictForFile(
				"file"+string(rune('0'+i))+".py",
				"func_"+string(rune('A'+j)),
				body,
			))
		}
		analyses = append(analyses, makeAnalysis("file"+string(rune('0'+i))+".py", conflicts))
	}

	for i := 5; i < 10; i++ {
		var conflicts []SynthesisConflict
		for j := 0; j < 5; j++ {
			body := strings.Repeat("def func_"+string(rune('A'+j))+"(): return value\n", 5)
			conflicts = append(conflicts, makeAddConflictForFile(
				"file"+string(rune('0'+i))+".py",
				"func_"+string(rune('A'+j)),
				body,
			))
		}
		analyses = append(analyses, makeAnalysis("file"+string(rune('0'+i))+".py", conflicts))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectInterFileMoves(analyses)
	}
}
