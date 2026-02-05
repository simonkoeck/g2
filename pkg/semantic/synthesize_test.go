package semantic

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simonkoeck/g2/pkg/ui"
)

// TestDefaultMergeConfig verifies safe defaults
func TestDefaultMergeConfig(t *testing.T) {
	config := DefaultMergeConfig()

	if config.DryRun != false {
		t.Errorf("DryRun should default to false, got %v", config.DryRun)
	}
	if config.CreateBackup != true {
		t.Errorf("CreateBackup should default to true, got %v", config.CreateBackup)
	}
	if config.Verbose != false {
		t.Errorf("Verbose should default to false, got %v", config.Verbose)
	}
}

// TestDetectCollisions tests collision detection between overlapping ranges
func TestDetectCollisions(t *testing.T) {
	tests := []struct {
		name              string
		conflicts         []SynthesisConflict
		expectedCollision int
	}{
		{
			name:              "empty conflicts",
			conflicts:         []SynthesisConflict{},
			expectedCollision: 0,
		},
		{
			name: "single conflict - no collision",
			conflicts: []SynthesisConflict{
				makeConflict("func1", 0, 50),
			},
			expectedCollision: 0,
		},
		{
			name: "two non-overlapping conflicts",
			conflicts: []SynthesisConflict{
				makeConflict("func1", 0, 50),
				makeConflict("func2", 60, 100),
			},
			expectedCollision: 0,
		},
		{
			name: "two adjacent conflicts (no overlap)",
			conflicts: []SynthesisConflict{
				makeConflict("func1", 0, 50),
				makeConflict("func2", 50, 100),
			},
			expectedCollision: 0,
		},
		{
			name: "two overlapping conflicts - class contains method",
			conflicts: []SynthesisConflict{
				makeConflict("MyClass", 0, 200),
				makeConflict("myMethod", 50, 100),
			},
			expectedCollision: 1,
		},
		{
			name: "three conflicts with one overlap",
			conflicts: []SynthesisConflict{
				makeConflict("func1", 0, 50),
				makeConflict("class1", 60, 200),
				makeConflict("method1", 80, 120),
			},
			expectedCollision: 1,
		},
		{
			name: "nested conflicts - multiple overlaps",
			conflicts: []SynthesisConflict{
				makeConflict("outer", 0, 300),
				makeConflict("middle", 50, 200),
				makeConflict("inner", 100, 150),
			},
			expectedCollision: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collisions := detectCollisions(tt.conflicts)
			if len(collisions) != tt.expectedCollision {
				t.Errorf("expected %d collisions, got %d", tt.expectedCollision, len(collisions))
			}
		})
	}
}

// TestHandleCollisions tests that collisions are properly marked and inner conflicts skipped
func TestHandleCollisions(t *testing.T) {
	// Create a class containing a method - both modified
	conflicts := []SynthesisConflict{
		makeConflict("MyClass", 0, 200),
		makeConflict("myMethod", 50, 100),
	}

	collisions := detectCollisions(conflicts)
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}

	result := handleCollisions(conflicts, collisions)

	// Should have only 1 conflict left (the outer one)
	if len(result) != 1 {
		t.Errorf("expected 1 conflict after handling, got %d", len(result))
	}

	// The outer conflict should be marked as collision
	if !strings.Contains(result[0].UIConflict.ConflictType, "Collision Detected") {
		t.Errorf("outer conflict should be marked as collision, got: %s", result[0].UIConflict.ConflictType)
	}

	// Status should be "Needs Resolution"
	if result[0].UIConflict.Status != "Needs Resolution" {
		t.Errorf("collision status should be 'Needs Resolution', got: %s", result[0].UIConflict.Status)
	}
}

// TestAtomicWriteWithBackup tests the atomic write and backup functionality
func TestAtomicWriteWithBackup(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "g2-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates backup and writes file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test1.py")
		originalContent := []byte("original content")
		newContent := []byte("new content")

		// Create original file
		if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Write with backup
		err := atomicWriteWithBackup(testFile, newContent, true)
		if err != nil {
			t.Fatalf("atomicWriteWithBackup failed: %v", err)
		}

		// Verify new content
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read test file: %v", err)
		}
		if string(content) != string(newContent) {
			t.Errorf("expected new content, got: %s", content)
		}

		// Verify backup was created
		backupFile := testFile + ".orig"
		backupContent, err := os.ReadFile(backupFile)
		if err != nil {
			t.Fatalf("backup file not created: %v", err)
		}
		if string(backupContent) != string(originalContent) {
			t.Errorf("backup content mismatch, got: %s", backupContent)
		}
	})

	t.Run("does not overwrite existing backup", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test2.py")
		originalBackup := []byte("original backup")
		originalContent := []byte("original content")
		newContent := []byte("new content")

		// Create original file and backup
		if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		backupFile := testFile + ".orig"
		if err := os.WriteFile(backupFile, originalBackup, 0644); err != nil {
			t.Fatalf("failed to create backup file: %v", err)
		}

		// Write with backup (should not overwrite existing backup)
		err := atomicWriteWithBackup(testFile, newContent, true)
		if err != nil {
			t.Fatalf("atomicWriteWithBackup failed: %v", err)
		}

		// Verify backup was NOT overwritten
		backupContent, err := os.ReadFile(backupFile)
		if err != nil {
			t.Fatalf("failed to read backup: %v", err)
		}
		if string(backupContent) != string(originalBackup) {
			t.Errorf("backup was overwritten, expected: %s, got: %s", originalBackup, backupContent)
		}
	})

	t.Run("works without backup", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test3.py")
		originalContent := []byte("original content")
		newContent := []byte("new content")

		// Create original file
		if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Write without backup
		err := atomicWriteWithBackup(testFile, newContent, false)
		if err != nil {
			t.Fatalf("atomicWriteWithBackup failed: %v", err)
		}

		// Verify no backup was created
		backupFile := testFile + ".orig"
		if _, err := os.Stat(backupFile); !os.IsNotExist(err) {
			t.Error("backup file should not exist when CreateBackup=false")
		}
	})

	t.Run("creates new file if doesn't exist", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "new_file.py")
		newContent := []byte("new content")

		// Write to non-existent file
		err := atomicWriteWithBackup(testFile, newContent, true)
		if err != nil {
			t.Fatalf("atomicWriteWithBackup failed: %v", err)
		}

		// Verify content
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read test file: %v", err)
		}
		if string(content) != string(newContent) {
			t.Errorf("expected new content, got: %s", content)
		}
	})

	t.Run("fails on read-only directory", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("test not applicable for root user")
		}

		readOnlyDir := filepath.Join(tmpDir, "readonly")
		if err := os.Mkdir(readOnlyDir, 0555); err != nil {
			t.Fatalf("failed to create read-only dir: %v", err)
		}
		defer os.Chmod(readOnlyDir, 0755) // Restore for cleanup

		testFile := filepath.Join(readOnlyDir, "test.py")
		err := atomicWriteWithBackup(testFile, []byte("content"), false)
		if err == nil {
			t.Error("expected error writing to read-only directory")
		}
	})
}

// TestFormatWriteError tests user-friendly error formatting
func TestFormatWriteError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		operation      string
		expectedSubstr string
	}{
		{
			name:           "permission denied",
			err:            os.ErrPermission,
			operation:      "write",
			expectedSubstr: "permission denied",
		},
		{
			name:           "generic error",
			err:            os.ErrNotExist,
			operation:      "read",
			expectedSubstr: "failed to read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := formatWriteError("/test/file.py", tt.err, tt.operation)
			if !strings.Contains(err.Error(), tt.expectedSubstr) {
				t.Errorf("expected error to contain %q, got: %s", tt.expectedSubstr, err.Error())
			}
		})
	}
}

// TestReplaceBytes tests the byte replacement function
func TestReplaceBytes(t *testing.T) {
	tests := []struct {
		name        string
		canvas      []byte
		start       uint32
		end         uint32
		replacement []byte
		expected    []byte
	}{
		{
			name:        "replace middle",
			canvas:      []byte("hello world"),
			start:       6,
			end:         11,
			replacement: []byte("golang"),
			expected:    []byte("hello golang"),
		},
		{
			name:        "insert at beginning",
			canvas:      []byte("world"),
			start:       0,
			end:         0,
			replacement: []byte("hello "),
			expected:    []byte("hello world"),
		},
		{
			name:        "append at end",
			canvas:      []byte("hello"),
			start:       5,
			end:         5,
			replacement: []byte(" world"),
			expected:    []byte("hello world"),
		},
		{
			name:        "delete (empty replacement)",
			canvas:      []byte("hello cruel world"),
			start:       6,
			end:         12,
			replacement: []byte(""),
			expected:    []byte("hello world"),
		},
		{
			name:        "bounds checking - start beyond length",
			canvas:      []byte("hello"),
			start:       100,
			end:         100,
			replacement: []byte(" world"),
			expected:    []byte("hello world"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceBytes(tt.canvas, tt.start, tt.end, tt.replacement)
			if string(result) != string(tt.expected) {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestGetConflictStartByte tests start byte extraction
func TestGetConflictStartByte(t *testing.T) {
	t.Run("uses local position when available", func(t *testing.T) {
		conflict := &SynthesisConflict{
			Local: &Definition{StartByte: 100},
			Base:  &Definition{StartByte: 50},
		}
		if getConflictStartByte(conflict) != 100 {
			t.Error("should use local position")
		}
	})

	t.Run("falls back to base when local nil", func(t *testing.T) {
		conflict := &SynthesisConflict{
			Local: nil,
			Base:  &Definition{StartByte: 50},
		}
		if getConflictStartByte(conflict) != 50 {
			t.Error("should fall back to base position")
		}
	})

	t.Run("returns 0 when both nil", func(t *testing.T) {
		conflict := &SynthesisConflict{
			Local: nil,
			Base:  nil,
		}
		if getConflictStartByte(conflict) != 0 {
			t.Error("should return 0 when no position available")
		}
	})
}

// TestGetConflictEndByte tests end byte extraction
func TestGetConflictEndByte(t *testing.T) {
	t.Run("uses local position when available", func(t *testing.T) {
		conflict := &SynthesisConflict{
			Local: &Definition{EndByte: 200},
			Base:  &Definition{EndByte: 150},
		}
		if getConflictEndByte(conflict) != 200 {
			t.Error("should use local position")
		}
	})

	t.Run("falls back to base when local nil", func(t *testing.T) {
		conflict := &SynthesisConflict{
			Local: nil,
			Base:  &Definition{EndByte: 150},
		}
		if getConflictEndByte(conflict) != 150 {
			t.Error("should fall back to base position")
		}
	})
}

// TestCapitalizeFirst tests string capitalization
func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"function", "Function"},
		{"class", "Class"},
		{"", ""},
		{"A", "A"},
		{"aBC", "ABC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := capitalizeFirst(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestPrintDryRunDiff verifies dry-run doesn't panic
func TestPrintDryRunDiff(t *testing.T) {
	// This test just verifies the function doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printDryRunDiff panicked: %v", r)
		}
	}()

	original := []byte("line1\nline2\nline3")
	proposed := []byte("line1\nmodified\nline3\nline4")

	// Capture stdout to avoid polluting test output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printDryRunDiff("test.py", original, proposed)

	w.Close()
	os.Stdout = oldStdout

	// Just verify it ran without panic
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "Dry Run") {
		t.Error("dry run output should contain header")
	}
}

// TestAnalyzeSynthesisConflict tests conflict type detection
func TestAnalyzeSynthesisConflict(t *testing.T) {
	t.Run("added in both - identical", func(t *testing.T) {
		local := &Definition{Kind: "function", Body: "def foo(): pass"}
		remote := &Definition{Kind: "function", Body: "def foo(): pass"}

		conflict := analyzeSynthesisConflict("test.py", "foo", nil, local, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("identical additions should be auto-mergeable, got: %s", conflict.UIConflict.Status)
		}
	})

	t.Run("added in both - differs", func(t *testing.T) {
		local := &Definition{Kind: "function", Body: "def foo(): return 1"}
		remote := &Definition{Kind: "function", Body: "def foo(): return 2"}

		conflict := analyzeSynthesisConflict("test.py", "foo", nil, local, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if conflict.UIConflict.Status != "Needs Resolution" {
			t.Errorf("different additions should need resolution, got: %s", conflict.UIConflict.Status)
		}
	})

	t.Run("modified in both - exactly same", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		local := &Definition{Kind: "function", Body: "def foo(): return 1"}
		remote := &Definition{Kind: "function", Body: "def foo(): return 1"}

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Modified (same)") {
			t.Errorf("should be 'Modified (same)', got: %s", conflict.UIConflict.ConflictType)
		}
	})

	t.Run("modified in both - formatted change", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		local := &Definition{Kind: "function", Body: "def foo():  return  1"} // extra spaces
		remote := &Definition{Kind: "function", Body: "def foo(): return 1"}  // normal

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Formatted Change") {
			t.Errorf("should be 'Formatted Change', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("formatted changes should be auto-mergeable")
		}
	})

	t.Run("delete/modify conflict", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		remote := &Definition{Kind: "function", Body: "def foo(): return 1"}

		conflict := analyzeSynthesisConflict("test.py", "foo", base, nil, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Delete/Modify") {
			t.Errorf("should be Delete/Modify conflict, got: %s", conflict.UIConflict.ConflictType)
		}
	})

	t.Run("remote only change - auto-mergeable", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		local := &Definition{Kind: "function", Body: "def foo(): pass"} // unchanged
		remote := &Definition{Kind: "function", Body: "def foo(): return 1"}

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Updated (remote)") {
			t.Errorf("should be 'Updated (remote)', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("remote-only changes should be auto-mergeable")
		}
	})

	// NEW TESTS for improved automerging

	t.Run("local only change - auto-mergeable", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		local := &Definition{Kind: "function", Body: "def foo(): return 1"}
		remote := &Definition{Kind: "function", Body: "def foo(): pass"} // unchanged

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Updated (local)") {
			t.Errorf("should be 'Updated (local)', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("local-only changes should be auto-mergeable")
		}
	})

	t.Run("deleted in both branches - auto-mergeable", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}

		conflict := analyzeSynthesisConflict("test.py", "foo", base, nil, nil, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Deleted (both)") {
			t.Errorf("should be 'Deleted (both)', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("deletion agreement should be auto-mergeable")
		}
	})

	t.Run("deleted locally, remote unchanged - auto-mergeable", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		remote := &Definition{Kind: "function", Body: "def foo(): pass"} // unchanged from base

		conflict := analyzeSynthesisConflict("test.py", "foo", base, nil, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Deleted (local)") {
			t.Errorf("should be 'Deleted (local)', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("local deletion with unchanged remote should be auto-mergeable")
		}
	})

	t.Run("deleted remotely, local unchanged - auto-mergeable", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		local := &Definition{Kind: "function", Body: "def foo(): pass"} // unchanged from base

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, nil, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Deleted (remote)") {
			t.Errorf("should be 'Deleted (remote)', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("remote deletion with unchanged local should be auto-mergeable")
		}
	})

	t.Run("modify/delete conflict - needs resolution", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		local := &Definition{Kind: "function", Body: "def foo(): return 1"} // modified

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, nil, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Modify/Delete") {
			t.Errorf("should be 'Modify/Delete', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Needs Resolution" {
			t.Errorf("modify/delete conflict should need resolution")
		}
	})

	t.Run("comment-only change - auto-mergeable", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo():\n    return 1"}
		local := &Definition{Kind: "function", Body: "def foo():\n    # local comment\n    return 1"}
		remote := &Definition{Kind: "function", Body: "def foo():\n    # remote comment\n    return 1"}

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, remote, LangPython)

		if conflict == nil {
			t.Fatal("expected conflict")
		}
		if !strings.Contains(conflict.UIConflict.ConflictType, "Comment Change") {
			t.Errorf("should be 'Comment Change', got: %s", conflict.UIConflict.ConflictType)
		}
		if conflict.UIConflict.Status != "Can Auto-merge" {
			t.Errorf("comment-only changes should be auto-mergeable")
		}
	})

	t.Run("no conflict - no change", func(t *testing.T) {
		base := &Definition{Kind: "function", Body: "def foo(): pass"}
		local := &Definition{Kind: "function", Body: "def foo(): pass"}
		remote := &Definition{Kind: "function", Body: "def foo(): pass"}

		conflict := analyzeSynthesisConflict("test.py", "foo", base, local, remote, LangPython)

		// No changes = no conflict
		if conflict != nil {
			t.Errorf("no changes should produce no conflict, got: %v", conflict)
		}
	})
}

// TestInsertConflictMarkers tests Git-style marker insertion
func TestInsertConflictMarkers(t *testing.T) {
	canvas := []byte("before\ndef foo(): pass\nafter")
	conflict := &SynthesisConflict{
		Local:  &Definition{Body: "def foo(): return 1", StartByte: 7, EndByte: 22},
		Remote: &Definition{Body: "def foo(): return 2"},
	}

	result := insertConflictMarkers(canvas, conflict)

	resultStr := string(result)
	if !strings.Contains(resultStr, "<<<<<<< LOCAL") {
		t.Error("should contain LOCAL marker")
	}
	if !strings.Contains(resultStr, "=======") {
		t.Error("should contain separator")
	}
	if !strings.Contains(resultStr, ">>>>>>> REMOTE") {
		t.Error("should contain REMOTE marker")
	}
	if !strings.Contains(resultStr, "def foo(): return 1") {
		t.Error("should contain local body")
	}
	if !strings.Contains(resultStr, "def foo(): return 2") {
		t.Error("should contain remote body")
	}
}

// TestApplyAutoMerge tests auto-merge application
func TestApplyAutoMerge(t *testing.T) {
	t.Run("identical content - no change", func(t *testing.T) {
		canvas := []byte("def foo(): return 1")
		conflict := &SynthesisConflict{
			Local:  &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
			Remote: &Definition{Body: "def foo(): return 1"},
		}

		result := applyAutoMerge(canvas, conflict)
		if string(result) != string(canvas) {
			t.Errorf("identical content should not change, got: %s", result)
		}
	})

	t.Run("remote change - applies remote", func(t *testing.T) {
		canvas := []byte("def foo(): pass")
		conflict := &SynthesisConflict{
			Local:  &Definition{Body: "def foo(): pass", StartByte: 0, EndByte: 15},
			Remote: &Definition{Body: "def foo(): return 42"},
		}

		result := applyAutoMerge(canvas, conflict)
		if string(result) != "def foo(): return 42" {
			t.Errorf("should apply remote change, got: %s", result)
		}
	})

	t.Run("deleted both - no change to canvas", func(t *testing.T) {
		// Canvas doesn't have the function (already deleted in local)
		canvas := []byte("def other(): pass")
		conflict := &SynthesisConflict{
			Base:   &Definition{Body: "def foo(): pass", StartByte: 0, EndByte: 15},
			Local:  nil, // deleted locally
			Remote: nil, // deleted remotely
		}

		result := applyAutoMerge(canvas, conflict)
		if string(result) != string(canvas) {
			t.Errorf("deleted both should not change canvas, got: %s", result)
		}
	})

	t.Run("deleted locally, remote unchanged - no change to canvas", func(t *testing.T) {
		// Canvas doesn't have the function (local deleted it)
		canvas := []byte("def other(): pass")
		conflict := &SynthesisConflict{
			Base:   &Definition{Body: "def foo(): pass"},
			Local:  nil,                                  // deleted locally
			Remote: &Definition{Body: "def foo(): pass"}, // unchanged from base
		}

		result := applyAutoMerge(canvas, conflict)
		if string(result) != string(canvas) {
			t.Errorf("deleted locally (remote unchanged) should not change canvas, got: %s", result)
		}
	})

	t.Run("deleted remotely, local unchanged - removes from canvas", func(t *testing.T) {
		// Canvas has the function (local kept it unchanged)
		canvas := []byte("def foo(): pass\ndef bar(): pass")
		conflict := &SynthesisConflict{
			Base:   &Definition{Body: "def foo(): pass"},
			Local:  &Definition{Body: "def foo(): pass", StartByte: 0, EndByte: 15}, // unchanged from base
			Remote: nil,                                                             // deleted remotely
		}

		result := applyAutoMerge(canvas, conflict)
		// Should remove def foo() from canvas
		if strings.Contains(string(result), "def foo()") {
			t.Errorf("deleted remotely should remove from canvas, got: %s", result)
		}
		if !strings.Contains(string(result), "def bar()") {
			t.Errorf("should preserve other content, got: %s", result)
		}
	})

	t.Run("local only addition - keeps local", func(t *testing.T) {
		canvas := []byte("def foo(): return 1")
		conflict := &SynthesisConflict{
			Base:   nil,
			Local:  &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
			Remote: nil,
		}

		result := applyAutoMerge(canvas, conflict)
		if string(result) != string(canvas) {
			t.Errorf("local only addition should keep local, got: %s", result)
		}
	})

	t.Run("remote only addition - appends remote", func(t *testing.T) {
		canvas := []byte("# existing content")
		conflict := &SynthesisConflict{
			Base:   nil,
			Local:  nil,
			Remote: &Definition{Body: "def new_func(): pass"},
		}

		result := applyAutoMerge(canvas, conflict)
		if !strings.Contains(string(result), "# existing content") {
			t.Errorf("should preserve existing content, got: %s", result)
		}
		if !strings.Contains(string(result), "def new_func()") {
			t.Errorf("should append remote content, got: %s", result)
		}
	})
}

// Helper function to create test conflicts
func makeConflict(name string, start, end uint32) SynthesisConflict {
	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "test.py",
			ConflictType: "Function '" + name + "' Modified",
			Status:       "Needs Resolution",
		},
		Local: &Definition{
			Name:      name,
			Kind:      "function",
			StartByte: start,
			EndByte:   end,
		},
	}
}

// =============================================================================
// Inter-File Move Synthesis Tests
// =============================================================================

// TestApplyAutoMerge_InterFileMoveSourceDelete tests synthesis for the SOURCE side
// of an inter-file move (the delete). The definition was deleted from the source file
// and moved to another file. Synthesis should be a no-op (keep canvas unchanged).
func TestApplyAutoMerge_InterFileMoveSourceDelete(t *testing.T) {
	// Source file content after the definition was deleted locally
	canvas := []byte("def other_func():\n    return 'other'\n")

	// The inter-file move source conflict: Base is set (original location),
	// but Local and Remote are nil (deleted in both branches)
	conflict := &SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "utils.py",
			ConflictType: "Function 'helper' Moved to newutils.py (Exact Match)",
			Status:       "Can Auto-merge",
		},
		Base: &Definition{
			Name:      "helper",
			Kind:      "function",
			Body:      "def helper():\n    return 'helping'\n",
			StartByte: 0, // Was at beginning before deletion
			EndByte:   35,
		},
		Local:  nil, // Deleted locally
		Remote: nil, // Deleted remotely
	}

	result := applyAutoMerge(canvas, conflict)

	// Canvas should be unchanged - the delete is already in effect
	if string(result) != string(canvas) {
		t.Errorf("inter-file move source delete should not modify canvas\nexpected: %q\ngot: %q", canvas, result)
	}
}

// TestApplyAutoMerge_InterFileMoveDestAddLocal tests synthesis for the DEST side
// of an inter-file move where the definition was added in LOCAL branch.
func TestApplyAutoMerge_InterFileMoveDestAddLocal(t *testing.T) {
	// Dest file content with the definition added locally
	canvas := []byte("def helper():\n    return 'helping'\n")

	// The inter-file move dest conflict: Base is nil (didn't exist here before),
	// Local is set (added locally)
	conflict := &SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "newutils.py",
			ConflictType: "Function 'helper' Moved from utils.py (Exact Match)",
			Status:       "Can Auto-merge",
		},
		Base: nil, // Didn't exist in base
		Local: &Definition{
			Name:      "helper",
			Kind:      "function",
			Body:      "def helper():\n    return 'helping'\n",
			StartByte: 0,
			EndByte:   35,
		},
		Remote: nil, // Not added on remote in this scenario
	}

	result := applyAutoMerge(canvas, conflict)

	// Canvas should be unchanged - local addition is already in place
	if string(result) != string(canvas) {
		t.Errorf("inter-file move dest add (local) should keep local content\nexpected: %q\ngot: %q", canvas, result)
	}
}

// TestApplyAutoMerge_InterFileMoveDestAddRemote tests synthesis for the DEST side
// of an inter-file move where the definition was added in REMOTE branch only.
func TestApplyAutoMerge_InterFileMoveDestAddRemote(t *testing.T) {
	// Dest file content - empty or has other content, but no helper yet
	canvas := []byte("# newutils.py\n")

	// The inter-file move dest conflict: Base is nil, Remote is set
	conflict := &SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "newutils.py",
			ConflictType: "Function 'helper' Moved from utils.py (Exact Match)",
			Status:       "Can Auto-merge",
		},
		Base:  nil, // Didn't exist in base
		Local: nil, // Not in local yet
		Remote: &Definition{
			Name: "helper",
			Kind: "function",
			Body: "def helper():\n    return 'helping'\n",
		},
	}

	result := applyAutoMerge(canvas, conflict)

	// Should append remote content
	if !strings.Contains(string(result), "def helper()") {
		t.Errorf("inter-file move dest add (remote) should append remote content\ngot: %q", result)
	}
	if !strings.Contains(string(result), "# newutils.py") {
		t.Errorf("should preserve original canvas content\ngot: %q", result)
	}
}

// TestApplyAutoMerge_InterFileMoveDestAddBothIdentical tests synthesis when both
// branches added the definition to the dest file (identical content).
func TestApplyAutoMerge_InterFileMoveDestAddBothIdentical(t *testing.T) {
	canvas := []byte("def helper():\n    return 'helping'\n")

	// Both branches added the same definition
	conflict := &SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         "newutils.py",
			ConflictType: "Function 'helper' Moved from utils.py (Exact Match)",
			Status:       "Can Auto-merge",
		},
		Base: nil,
		Local: &Definition{
			Name:      "helper",
			Kind:      "function",
			Body:      "def helper():\n    return 'helping'\n",
			StartByte: 0,
			EndByte:   35,
		},
		Remote: &Definition{
			Name: "helper",
			Kind: "function",
			Body: "def helper():\n    return 'helping'\n",
		},
	}

	result := applyAutoMerge(canvas, conflict)

	// Should keep local (identical, no change needed)
	if string(result) != string(canvas) {
		t.Errorf("identical additions should not change canvas\nexpected: %q\ngot: %q", canvas, result)
	}
}

// TestSynthesizeFile_InterFileMoveSource tests full synthesis for source file of inter-file move
func TestSynthesizeFile_InterFileMoveSource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-interfile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo and change to it (needed because SynthesizeFile runs git add)
	cleanup := initGitRepo(t, tmpDir)
	defer cleanup()

	// Use relative path since we're now in tmpDir
	testFile := "utils.py"
	// Content after helper() was deleted - only other_func remains
	localContent := []byte("def other_func():\n    return 'other'\n")

	if err := os.WriteFile(testFile, localContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	analysis := &SynthesisAnalysis{
		File: testFile,
		Conflicts: []SynthesisConflict{
			{
				UIConflict: ui.Conflict{
					File:         testFile,
					ConflictType: "Function 'helper' Moved to newutils.py (Exact Match)",
					Status:       "Can Auto-merge",
				},
				Base: &Definition{
					Name:      "helper",
					Kind:      "function",
					Body:      "def helper():\n    return 'helping'\n",
					StartByte: 0,
					EndByte:   35,
				},
				Local:  nil,
				Remote: nil,
			},
		},
		LocalContent: localContent,
	}

	config := DefaultMergeConfig()
	config.CreateBackup = false

	result := SynthesizeFile(analysis, config)

	if !result.Success {
		t.Errorf("synthesis should succeed: %v", result.Error)
	}
	if !result.AllAutoMerged {
		t.Error("inter-file move source should be auto-merged")
	}

	// File content should be unchanged (delete already in effect)
	content, _ := os.ReadFile(testFile)
	if string(content) != string(localContent) {
		t.Errorf("source file should be unchanged\nexpected: %q\ngot: %q", localContent, content)
	}
}

// TestSynthesizeFile_InterFileMoveDestRemote tests full synthesis for dest file
// when the definition was added on the remote branch
func TestSynthesizeFile_InterFileMoveDestRemote(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-interfile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo and change to it (needed because SynthesizeFile runs git add)
	cleanup := initGitRepo(t, tmpDir)
	defer cleanup()

	// Use relative path since we're now in tmpDir
	testFile := "newutils.py"
	// Local content - file exists but doesn't have the helper function yet
	localContent := []byte("# New utilities module\n")

	if err := os.WriteFile(testFile, localContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	analysis := &SynthesisAnalysis{
		File: testFile,
		Conflicts: []SynthesisConflict{
			{
				UIConflict: ui.Conflict{
					File:         testFile,
					ConflictType: "Function 'helper' Moved from utils.py (Exact Match)",
					Status:       "Can Auto-merge",
				},
				Base:  nil,
				Local: nil,
				Remote: &Definition{
					Name: "helper",
					Kind: "function",
					Body: "def helper():\n    return 'helping'",
				},
			},
		},
		LocalContent: localContent,
	}

	config := DefaultMergeConfig()
	config.CreateBackup = false

	result := SynthesizeFile(analysis, config)

	if !result.Success {
		t.Errorf("synthesis should succeed: %v", result.Error)
	}
	if !result.AllAutoMerged {
		t.Error("inter-file move dest should be auto-merged")
	}

	// File content should now include the helper function
	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "def helper()") {
		t.Errorf("dest file should contain helper function\ngot: %q", content)
	}
	if !strings.Contains(string(content), "# New utilities module") {
		t.Errorf("dest file should preserve original content\ngot: %q", content)
	}
}

// initGitRepo initializes a git repo in the given directory and changes to it
// Returns a cleanup function that restores the original directory
func initGitRepo(t *testing.T, dir string) func() {
	t.Helper()

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}

	// Change to temp dir (git add needs to run from within the repo)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		os.Chdir(origDir)
		t.Fatalf("failed to init git repo: %v", err)
	}
	// Configure git user for commits
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test").Run()

	return func() {
		os.Chdir(origDir)
	}
}

// TestSynthesizeToBytes tests the byte-level synthesis function
func TestSynthesizeToBytes(t *testing.T) {
	t.Run("empty conflicts returns local content", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File:         "test.py",
			Conflicts:    []SynthesisConflict{},
			LocalContent: []byte("original content"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allMerged {
			t.Error("empty conflicts should be all merged")
		}
		if string(result) != "original content" {
			t.Errorf("expected original content, got: %s", result)
		}
	})

	t.Run("no local content returns error", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				makeConflict("foo", 0, 10),
			},
			LocalContent: []byte{},
		}

		_, _, err := SynthesizeToBytes(analysis)

		if err == nil {
			t.Error("expected error for empty local content")
		}
	})

	t.Run("auto-merge conflicts are resolved", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Can Auto-merge"},
					Local:      &Definition{Body: "def foo(): pass", StartByte: 0, EndByte: 15},
					Remote:     &Definition{Body: "def foo(): return 1"},
				},
			},
			LocalContent: []byte("def foo(): pass"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allMerged {
			t.Error("auto-merge conflicts should all be merged")
		}
		if string(result) != "def foo(): return 1" {
			t.Errorf("expected remote content, got: %s", result)
		}
	})

	t.Run("manual conflicts get markers", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Needs Resolution"},
					Local:      &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
					Remote:     &Definition{Body: "def foo(): return 2"},
				},
			},
			LocalContent: []byte("def foo(): return 1"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allMerged {
			t.Error("manual conflicts should not be all merged")
		}
		if !strings.Contains(string(result), "<<<<<<< LOCAL") {
			t.Error("should contain conflict markers")
		}
	})

	t.Run("user resolution skip leaves markers", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict:     ui.Conflict{Status: "Needs Resolution"},
					Local:          &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
					Remote:         &Definition{Body: "def foo(): return 2"},
					UserResolution: UserResolutionSkip,
				},
			},
			LocalContent: []byte("def foo(): return 1"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allMerged {
			t.Error("skip resolution should not be all merged")
		}
		if !strings.Contains(string(result), "<<<<<<< LOCAL") {
			t.Error("skip should leave conflict markers")
		}
	})

	t.Run("user resolution local keeps local", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict:     ui.Conflict{Status: "Needs Resolution"},
					Local:          &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
					Remote:         &Definition{Body: "def foo(): return 2"},
					UserResolution: UserResolutionLocal,
				},
			},
			LocalContent: []byte("def foo(): return 1"),
		}

		result, _, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result) != "def foo(): return 1" {
			t.Errorf("expected local content, got: %s", result)
		}
	})

	t.Run("user resolution remote applies remote", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict:     ui.Conflict{Status: "Needs Resolution"},
					Local:          &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
					Remote:         &Definition{Body: "def foo(): return 2"},
					UserResolution: UserResolutionRemote,
				},
			},
			LocalContent: []byte("def foo(): return 1"),
		}

		result, _, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result) != "def foo(): return 2" {
			t.Errorf("expected remote content, got: %s", result)
		}
	})

	t.Run("user resolution both keeps both", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict:     ui.Conflict{Status: "Needs Resolution"},
					Local:          &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
					Remote:         &Definition{Body: "def bar(): return 2"},
					UserResolution: UserResolutionBoth,
				},
			},
			LocalContent: []byte("def foo(): return 1"),
		}

		result, _, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(result), "def foo()") {
			t.Error("should contain local content")
		}
		if !strings.Contains(string(result), "def bar()") {
			t.Error("should contain remote content")
		}
	})
}

// TestSynthesizeToBytes_NewAutoMergeCases tests new auto-merge scenarios
func TestSynthesizeToBytes_NewAutoMergeCases(t *testing.T) {
	t.Run("deleted both - canvas unchanged", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Can Auto-merge", ConflictType: "Deleted (both)"},
					Base:       &Definition{Body: "def deleted(): pass"},
					Local:      nil,
					Remote:     nil,
				},
			},
			LocalContent: []byte("def remaining(): pass"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allMerged {
			t.Error("deleted both should be auto-merged")
		}
		if string(result) != "def remaining(): pass" {
			t.Errorf("canvas should be unchanged, got: %s", result)
		}
	})

	t.Run("deleted remotely, local unchanged - removes from canvas", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Can Auto-merge", ConflictType: "Deleted (remote)"},
					Base:       &Definition{Body: "def foo(): pass"},
					Local:      &Definition{Body: "def foo(): pass", StartByte: 0, EndByte: 15},
					Remote:     nil,
				},
			},
			LocalContent: []byte("def foo(): pass\ndef bar(): pass"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allMerged {
			t.Error("deleted remote should be auto-merged")
		}
		if strings.Contains(string(result), "def foo()") {
			t.Errorf("should remove deleted function, got: %s", result)
		}
		if !strings.Contains(string(result), "def bar()") {
			t.Errorf("should preserve other functions, got: %s", result)
		}
	})

	t.Run("deleted locally, remote unchanged - keeps canvas", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Can Auto-merge", ConflictType: "Deleted (local)"},
					Base:       &Definition{Body: "def deleted(): pass"},
					Local:      nil,
					Remote:     &Definition{Body: "def deleted(): pass"},
				},
			},
			LocalContent: []byte("def remaining(): pass"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allMerged {
			t.Error("deleted local should be auto-merged")
		}
		if string(result) != "def remaining(): pass" {
			t.Errorf("canvas should be unchanged, got: %s", result)
		}
	})

	t.Run("local only update - keeps local", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: "test.py",
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Can Auto-merge", ConflictType: "Updated (local)"},
					Base:       &Definition{Body: "def foo(): pass"},
					Local:      &Definition{Body: "def foo(): return 1", StartByte: 0, EndByte: 19},
					Remote:     &Definition{Body: "def foo(): pass"},
				},
			},
			LocalContent: []byte("def foo(): return 1"),
		}

		result, allMerged, err := SynthesizeToBytes(analysis)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allMerged {
			t.Error("local update should be auto-merged")
		}
		if string(result) != "def foo(): return 1" {
			t.Errorf("should keep local changes, got: %s", result)
		}
	})
}

// TestSynthesizeFileIntegration tests the full synthesis flow
func TestSynthesizeFileIntegration(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "g2-synth-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("empty conflicts - auto-merged", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File:         filepath.Join(tmpDir, "empty.py"),
			Conflicts:    []SynthesisConflict{},
			LocalContent: []byte("content"),
		}

		result := SynthesizeFile(analysis, DefaultMergeConfig())

		if !result.Success {
			t.Errorf("should succeed with empty conflicts")
		}
		if !result.AllAutoMerged {
			t.Errorf("empty conflicts should be auto-merged")
		}
	})

	t.Run("no local content - fails", func(t *testing.T) {
		analysis := &SynthesisAnalysis{
			File: filepath.Join(tmpDir, "no_content.py"),
			Conflicts: []SynthesisConflict{
				makeConflict("foo", 0, 10),
			},
			LocalContent: []byte{},
		}

		result := SynthesizeFile(analysis, DefaultMergeConfig())

		if result.Success {
			t.Errorf("should fail with no local content")
		}
		if result.Error == nil {
			t.Errorf("should have error")
		}
	})

	t.Run("dry-run mode - no file written", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "dryrun.py")
		originalContent := []byte("def foo(): pass")

		// Create original file
		if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		analysis := &SynthesisAnalysis{
			File: testFile,
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Can Auto-merge"},
					Local:      &Definition{Body: "def foo(): pass", StartByte: 0, EndByte: 15},
					Remote:     &Definition{Body: "def foo(): return 1"},
				},
			},
			LocalContent: originalContent,
		}

		config := DefaultMergeConfig()
		config.DryRun = true

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		result := SynthesizeFile(analysis, config)

		w.Close()
		os.Stdout = oldStdout
		buf := make([]byte, 4096)
		r.Read(buf)

		// File should be unchanged
		content, _ := os.ReadFile(testFile)
		if string(content) != string(originalContent) {
			t.Errorf("dry-run should not modify file, got: %s", content)
		}

		// Result should not be marked as auto-merged
		if result.AllAutoMerged {
			t.Error("dry-run should not mark as auto-merged")
		}
	})

	t.Run("backup created on write", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "backup_test.py")
		originalContent := []byte("def foo(): pass")

		// Create original file
		if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		analysis := &SynthesisAnalysis{
			File: testFile,
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{Status: "Needs Resolution"},
					Local:      &Definition{Body: "def foo(): pass", StartByte: 0, EndByte: 15},
					Remote:     &Definition{Body: "def foo(): return 1"},
				},
			},
			LocalContent: originalContent,
		}

		result := SynthesizeFile(analysis, DefaultMergeConfig())

		if !result.Success {
			t.Errorf("should succeed: %v", result.Error)
		}

		// Verify backup exists
		backupFile := testFile + ".orig"
		backupContent, err := os.ReadFile(backupFile)
		if err != nil {
			t.Errorf("backup should be created: %v", err)
		}
		if string(backupContent) != string(originalContent) {
			t.Errorf("backup content mismatch")
		}
	})

	t.Run("collision detection in synthesis", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "collision_test.py")
		originalContent := []byte("class Foo:\n    def bar(self): pass")

		// Create original file
		if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create overlapping conflicts (class contains method)
		analysis := &SynthesisAnalysis{
			File: testFile,
			Conflicts: []SynthesisConflict{
				{
					UIConflict: ui.Conflict{
						ConflictType: "Class 'Foo' Modified",
						Status:       "Needs Resolution",
					},
					Local: &Definition{
						Name:      "Foo",
						Body:      "class Foo:\n    def bar(self): pass",
						StartByte: 0,
						EndByte:   35,
					},
					Remote: &Definition{
						Body: "class Foo:\n    def bar(self): return 1",
					},
				},
				{
					UIConflict: ui.Conflict{
						ConflictType: "Function 'bar' Modified",
						Status:       "Needs Resolution",
					},
					Local: &Definition{
						Name:      "bar",
						Body:      "def bar(self): pass",
						StartByte: 15,
						EndByte:   34,
					},
					Remote: &Definition{
						Body: "def bar(self): return 2",
					},
				},
			},
			LocalContent: originalContent,
		}

		config := DefaultMergeConfig()
		config.Verbose = true

		result := SynthesizeFile(analysis, config)

		if !result.Success {
			t.Errorf("should succeed even with collisions: %v", result.Error)
		}

		// The collision should result in only 1 conflict being processed
		// (the inner one should be skipped)
		if result.ConflictCount != 1 {
			t.Errorf("expected 1 conflict after collision handling, got %d", result.ConflictCount)
		}
	})
}

func TestApplyAutoMerge_OrphanAddAppend(t *testing.T) {
	canvas := []byte("existing content\n")
	
	conflict := &SynthesisConflict{
		UIConflict: ui.Conflict{
			ConflictType: "Added (remote)",
			Status:       "Can Auto-merge",
		},
		Local:  nil,
		Remote: &Definition{
			Name: "newKey",
			Kind: "key",
			Body: "newKey: value\n",
		},
		Base: nil,
	}

	result := applyAutoMerge(canvas, conflict)
	
	if !strings.Contains(string(result), "newKey: value") {
		t.Errorf("expected result to contain appended content, got: %s", result)
	}
	if !strings.Contains(string(result), "existing content") {
		t.Errorf("expected result to preserve existing content, got: %s", result)
	}
}

// TestSynthesizeToBytes_MultipleConflictsWithOrphanAdd tests the scenario from YAML test
func TestSynthesizeToBytes_MultipleConflictsWithOrphanAdd(t *testing.T) {
	// Simulate YAML config scenario:
	// - database: exists in base, local updated (added pool_size)
	// - logging: new in remote (orphan add)
	// - server: unchanged in all versions

	localContent := `database:
  host: localhost
  port: 5432
  pool_size: 10

server:
  host: 0.0.0.0
  port: 8080
`

	analysis := &SynthesisAnalysis{
		File:         "config.yaml",
		Language:     LangYAML,
		LocalContent: []byte(localContent),
		Conflicts: []SynthesisConflict{
			{
				UIConflict: ui.Conflict{
					ConflictType: "Key 'database' Updated (local)",
					Status:       "Can Auto-merge",
				},
				Local: &Definition{
					Name:      "database",
					Kind:      "key",
					Body:      "database:\n  host: localhost\n  port: 5432\n  pool_size: 10\n",
					StartByte: 0,
					EndByte:   55,
				},
				Remote: &Definition{
					Name:      "database",
					Kind:      "key",
					Body:      "database:\n  host: localhost\n  port: 5432\n",
					StartByte: 0,
					EndByte:   42,
				},
				Base: &Definition{
					Name:      "database",
					Kind:      "key",
					Body:      "database:\n  host: localhost\n  port: 5432\n",
					StartByte: 0,
					EndByte:   42,
				},
			},
			{
				UIConflict: ui.Conflict{
					ConflictType: "Key 'logging' Added (remote)",
					Status:       "Can Auto-merge",
				},
				Local:  nil, // Doesn't exist locally
				Remote: &Definition{
					Name:      "logging",
					Kind:      "key",
					Body:      "logging:\n  level: info\n  format: json\n",
					StartByte: 0, // Doesn't matter for orphan adds
					EndByte:   0,
				},
				Base: nil, // Doesn't exist in base
			},
		},
	}

	result, allAutoMerged, err := SynthesizeToBytes(analysis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("allAutoMerged: %v", allAutoMerged)
	t.Logf("Result:\n%s", string(result))

	// Check that local changes are preserved
	if !strings.Contains(string(result), "pool_size: 10") {
		t.Error("expected result to contain local pool_size change")
	}

	// Check that orphan add was appended
	if !strings.Contains(string(result), "logging:") {
		t.Error("expected result to contain remote logging section")
	}

	if !strings.Contains(string(result), "level: info") {
		t.Error("expected result to contain logging level")
	}
}
