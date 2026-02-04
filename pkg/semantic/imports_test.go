package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileToModule tests conversion of file paths to module names
func TestFileToModule(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"utils.py", "utils"},
		{"foo/bar.py", "foo.bar"},
		{"src/utils/helpers.py", "src.utils.helpers"},
		{"utils.js", "utils"},
		{"src/components/Button.tsx", "src.components.Button"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := fileToModule(tt.input)
			if result != tt.expected {
				t.Errorf("fileToModule(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseImportNames tests parsing of import name lists
func TestParseImportNames(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"helper", []string{"helper"}},
		{"helper, utils", []string{"helper", "utils"}},
		{"helper, utils, foo", []string{"helper", "utils", "foo"}},
		{"helper as h, utils", []string{"helper", "utils"}},
		{"helper  ,  utils", []string{"helper", "utils"}},
		{"helper # comment", []string{"helper"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseImportNames(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseImportNames(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, name := range result {
				if name != tt.expected[i] {
					t.Errorf("parseImportNames(%q)[%d] = %q, want %q", tt.input, i, name, tt.expected[i])
				}
			}
		})
	}
}

// TestFindPythonImportUpdates tests Python import detection
func TestFindPythonImportUpdates(t *testing.T) {
	moveMap := map[struct{ sourceModule, defName string }]string{
		{"utils", "helper"}: "newutils",
	}

	t.Run("simple from import", func(t *testing.T) {
		content := []byte("from utils import helper\n")
		updates := findPythonImportUpdates("test.py", content, moveMap)

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}

		u := updates[0]
		if u.FromModule != "utils" {
			t.Errorf("FromModule = %q, want %q", u.FromModule, "utils")
		}
		if u.ToModule != "newutils" {
			t.Errorf("ToModule = %q, want %q", u.ToModule, "newutils")
		}
		if !strings.Contains(u.NewImport, "from newutils import helper") {
			t.Errorf("NewImport = %q, should contain 'from newutils import helper'", u.NewImport)
		}
	})

	t.Run("multiple imports - one moves", func(t *testing.T) {
		content := []byte("from utils import helper, other_func\n")
		updates := findPythonImportUpdates("test.py", content, moveMap)

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}

		u := updates[0]
		// Should keep other_func in utils, add new import for helper
		if !strings.Contains(u.NewImport, "from utils import other_func") {
			t.Errorf("NewImport should keep 'from utils import other_func', got: %q", u.NewImport)
		}
		if !strings.Contains(u.NewImport, "from newutils import helper") {
			t.Errorf("NewImport should add 'from newutils import helper', got: %q", u.NewImport)
		}
	})

	t.Run("no matching imports", func(t *testing.T) {
		content := []byte("from other import something\n")
		updates := findPythonImportUpdates("test.py", content, moveMap)

		if len(updates) != 0 {
			t.Errorf("expected 0 updates, got %d", len(updates))
		}
	})

	t.Run("import with alias", func(t *testing.T) {
		content := []byte("from utils import helper as h\n")
		updates := findPythonImportUpdates("test.py", content, moveMap)

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}

		u := updates[0]
		if !strings.Contains(u.NewImport, "from newutils import helper") {
			t.Errorf("NewImport = %q, should handle alias", u.NewImport)
		}
	})

	t.Run("nested module path", func(t *testing.T) {
		moveMap := map[struct{ sourceModule, defName string }]string{
			{"foo.bar.utils", "helper"}: "foo.bar.newutils",
		}
		content := []byte("from foo.bar.utils import helper\n")
		updates := findPythonImportUpdates("test.py", content, moveMap)

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}

		if !strings.Contains(updates[0].NewImport, "from foo.bar.newutils import helper") {
			t.Errorf("should handle nested modules, got: %q", updates[0].NewImport)
		}
	})
}

// TestFindJSImportUpdates tests JavaScript/TypeScript import detection
func TestFindJSImportUpdates(t *testing.T) {
	// Note: moveMap keys use module paths (file paths converted to dot notation)
	// For src/app.js importing from './utils', the resolved module is 'src.utils'
	moveMap := map[struct{ sourceModule, defName string }]string{
		{"src.utils", "helper"}: "src.newutils",
	}

	t.Run("named import", func(t *testing.T) {
		content := []byte("import { helper } from './utils'\n")
		updates := findJSImportUpdates("src/app.js", content, moveMap)

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}

		u := updates[0]
		if !strings.Contains(u.NewImport, "helper") {
			t.Errorf("NewImport should contain helper, got: %q", u.NewImport)
		}
		if !strings.Contains(u.NewImport, "newutils") {
			t.Errorf("NewImport should reference newutils, got: %q", u.NewImport)
		}
	})

	t.Run("multiple named imports - one moves", func(t *testing.T) {
		content := []byte("import { helper, otherFunc } from './utils'\n")
		updates := findJSImportUpdates("src/app.js", content, moveMap)

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}

		u := updates[0]
		// Should keep otherFunc in original, add new import for helper
		if !strings.Contains(u.NewImport, "otherFunc") {
			t.Errorf("NewImport should keep otherFunc, got: %q", u.NewImport)
		}
		// The new import path should reference newutils
		if !strings.Contains(u.NewImport, "newutils") {
			t.Errorf("NewImport should add newutils import, got: %q", u.NewImport)
		}
	})

	t.Run("root level import", func(t *testing.T) {
		// For files at root level, ./utils resolves to just 'utils'
		rootMoveMap := map[struct{ sourceModule, defName string }]string{
			{"utils", "helper"}: "newutils",
		}
		content := []byte("import { helper } from './utils'\n")
		updates := findJSImportUpdates("app.js", content, rootMoveMap)

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}
		if !strings.Contains(updates[0].NewImport, "newutils") {
			t.Errorf("NewImport should reference newutils, got: %q", updates[0].NewImport)
		}
	})

	t.Run("no matching imports", func(t *testing.T) {
		content := []byte("import { something } from './other'\n")
		updates := findJSImportUpdates("src/app.js", content, moveMap)

		if len(updates) != 0 {
			t.Errorf("expected 0 updates, got %d", len(updates))
		}
	})
}

// TestApplyImportUpdates tests applying updates to files
func TestApplyImportUpdates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-import-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("applies single update", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test1.py")
		content := "from utils import helper\n\ndef main():\n    helper()\n"
		os.WriteFile(testFile, []byte(content), 0644)

		updates := []ImportUpdate{
			{
				File:       "test1.py",
				OldImport:  "from utils import helper",
				NewImport:  "from newutils import helper",
				LineNumber: 1,
			},
		}

		results := ApplyImportUpdates(updates, tmpDir)

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Error != nil {
			t.Errorf("unexpected error: %v", results[0].Error)
		}

		newContent, _ := os.ReadFile(testFile)
		if !strings.Contains(string(newContent), "from newutils import helper") {
			t.Errorf("file should contain updated import, got:\n%s", newContent)
		}
	})

	t.Run("applies multi-line update", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test2.py")
		content := "from utils import helper, other\n\ndef main():\n    pass\n"
		os.WriteFile(testFile, []byte(content), 0644)

		updates := []ImportUpdate{
			{
				File:       "test2.py",
				OldImport:  "from utils import helper, other",
				NewImport:  "from utils import other\nfrom newutils import helper",
				LineNumber: 1,
			},
		}

		results := ApplyImportUpdates(updates, tmpDir)

		if results[0].Error != nil {
			t.Errorf("unexpected error: %v", results[0].Error)
		}

		newContent, _ := os.ReadFile(testFile)
		if !strings.Contains(string(newContent), "from utils import other") {
			t.Errorf("file should keep original import, got:\n%s", newContent)
		}
		if !strings.Contains(string(newContent), "from newutils import helper") {
			t.Errorf("file should have new import, got:\n%s", newContent)
		}
	})
}

// TestFindImportUpdates_Integration tests the full flow
func TestFindImportUpdates_Integration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-import-int-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source files
	os.WriteFile(filepath.Join(tmpDir, "utils.py"), []byte("def helper(): pass\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "newutils.py"), []byte("def helper(): pass\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("from utils import helper\n\nhelper()\n"), 0644)

	// Create a move
	moves := []InterFileMove{
		{
			SourceFile: "utils.py",
			DestFile:   "newutils.py",
			SourceConflict: &SynthesisConflict{
				Base: &Definition{Name: "helper", Kind: "function"},
			},
			DestConflict: &SynthesisConflict{},
			MatchType:    "Exact Match",
			Similarity:   1.0,
		},
	}

	updates, err := FindImportUpdates(moves, tmpDir)
	if err != nil {
		t.Fatalf("FindImportUpdates failed: %v", err)
	}

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	u := updates[0]
	if u.File != "main.py" {
		t.Errorf("File = %q, want %q", u.File, "main.py")
	}
	if u.FromModule != "utils" {
		t.Errorf("FromModule = %q, want %q", u.FromModule, "utils")
	}
	if u.ToModule != "newutils" {
		t.Errorf("ToModule = %q, want %q", u.ToModule, "newutils")
	}
}

// TestFormatImportUpdateSummary tests summary formatting
func TestFormatImportUpdateSummary(t *testing.T) {
	t.Run("empty updates", func(t *testing.T) {
		result := FormatImportUpdateSummary([]ImportUpdate{})
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("single update", func(t *testing.T) {
		updates := []ImportUpdate{
			{
				File:       "main.py",
				LineNumber: 1,
				FromModule: "utils",
				ToModule:   "newutils",
			},
		}
		result := FormatImportUpdateSummary(updates)
		if !strings.Contains(result, "main.py") {
			t.Errorf("should contain file name, got: %q", result)
		}
		if !strings.Contains(result, "utils -> newutils") {
			t.Errorf("should show module change, got: %q", result)
		}
	})
}

// BenchmarkFindPythonImportUpdates benchmarks Python import scanning
func BenchmarkFindPythonImportUpdates(b *testing.B) {
	// Create a file with many imports
	var content strings.Builder
	for i := 0; i < 50; i++ {
		content.WriteString("from module" + string(rune('a'+i%26)) + " import func" + string(rune('0'+i%10)) + "\n")
	}
	contentBytes := []byte(content.String())

	moveMap := map[struct{ sourceModule, defName string }]string{
		{"modulea", "func0"}: "newmodule",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findPythonImportUpdates("test.py", contentBytes, moveMap)
	}
}
