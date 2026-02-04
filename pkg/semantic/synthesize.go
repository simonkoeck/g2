package semantic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/simonkoeck/g2/pkg/git"
	"github.com/simonkoeck/g2/pkg/logging"
	"github.com/simonkoeck/g2/pkg/ui"
)

// SynthesisConflict carries full definition data for synthesis
type SynthesisConflict struct {
	UIConflict ui.Conflict
	Local      *Definition // nil if deleted locally
	Remote     *Definition // nil if deleted remotely
	Base       *Definition // nil if added in both
}

// SynthesisAnalysis contains all data needed to synthesize a file
type SynthesisAnalysis struct {
	File         string
	Language     Language
	Conflicts    []SynthesisConflict
	LocalContent []byte // the "canvas" for synthesis
}

// SynthesisResult contains the outcome of synthesizing a file
type SynthesisResult struct {
	File           string
	Success        bool
	AllAutoMerged  bool
	Error          error
	ConflictCount  int
	AutoMergeCount int
}

// MergeConfig controls synthesis behavior
type MergeConfig struct {
	DryRun       bool          // If true, print proposed changes but don't write
	CreateBackup bool          // If true, create .orig backup (default: true)
	Verbose      bool          // If true, print detailed progress
	JSONOutput   bool          // If true, output JSON instead of human-readable text
	LogLevel     string        // Log level: debug, info, warn, error
	GitTimeout   time.Duration // Timeout for git operations (0 = use default)
	MaxFileSize  int64         // Maximum file size to process (0 = unlimited)
}

// DefaultMergeConfig returns safe defaults
func DefaultMergeConfig() MergeConfig {
	return MergeConfig{
		DryRun:       false,
		CreateBackup: true,
		Verbose:      false,
		JSONOutput:   false,
		LogLevel:     "warn",
		GitTimeout:   git.DefaultTimeout,
		MaxFileSize:  0,
	}
}

// RangeCollision represents overlapping conflicts
type RangeCollision struct {
	Outer *SynthesisConflict // The enclosing conflict (e.g., class)
	Inner *SynthesisConflict // The nested conflict (e.g., method inside class)
}

// AnalyzeConflictForSynthesis analyzes a conflicting file and returns full synthesis data
func AnalyzeConflictForSynthesis(file string) *SynthesisAnalysis {
	result := &SynthesisAnalysis{
		File:     file,
		Language: DetectLanguage(file),
	}

	// Get all three versions
	baseContent, baseErr := GetFileVersion(file, 1)
	localContent, localErr := GetFileVersion(file, 2)
	remoteContent, remoteErr := GetFileVersion(file, 3)

	// Store local content as the canvas
	if localErr == nil {
		result.LocalContent = localContent
	}

	// Handle binary files
	if (localErr == nil && IsBinaryFile(localContent)) ||
		(remoteErr == nil && IsBinaryFile(remoteContent)) {
		result.Conflicts = append(result.Conflicts, SynthesisConflict{
			UIConflict: ui.Conflict{
				File:         file,
				ConflictType: "Binary Conflict",
				Status:       "Needs Resolution",
			},
		})
		return result
	}

	// Parse all versions
	var baseAnalysis, localAnalysis, remoteAnalysis *FileAnalysis

	if baseErr == nil {
		baseAnalysis = ParseFile(baseContent, result.Language)
	} else {
		baseAnalysis = &FileAnalysis{} // Empty base (new file)
	}

	if localErr != nil {
		result.Conflicts = append(result.Conflicts, SynthesisConflict{
			UIConflict: ui.Conflict{
				File:         file,
				ConflictType: "File Missing (local)",
				Status:       "Needs Resolution",
			},
		})
		return result
	}
	localAnalysis = ParseFile(localContent, result.Language)

	if remoteErr != nil {
		result.Conflicts = append(result.Conflicts, SynthesisConflict{
			UIConflict: ui.Conflict{
				File:         file,
				ConflictType: "File Missing (remote)",
				Status:       "Needs Resolution",
			},
		})
		return result
	}
	remoteAnalysis = ParseFile(remoteContent, result.Language)

	// Check for parse errors
	if localAnalysis.ParseError != nil || remoteAnalysis.ParseError != nil {
		result.Conflicts = append(result.Conflicts, SynthesisConflict{
			UIConflict: ui.Conflict{
				File:         file,
				ConflictType: "Parse Error",
				Status:       "Needs Resolution",
			},
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

		conflict := analyzeSynthesisConflict(file, name, baseDef, localDef, remoteDef)
		if conflict != nil {
			result.Conflicts = append(result.Conflicts, *conflict)
		}
	}

	// Detect and consolidate move operations (delete + add of same definition)
	result.Conflicts = DetectMoves(result.Conflicts)

	return result
}

// analyzeSynthesisConflict determines conflict type and preserves definition data
func analyzeSynthesisConflict(file, name string, base, local, remote *Definition) *SynthesisConflict {
	// Determine the kind
	kind := "definition"
	if local != nil {
		kind = local.Kind
	} else if remote != nil {
		kind = remote.Kind
	} else if base != nil {
		kind = base.Kind
	}

	kindStr := capitalizeFirst(kind)

	// Normalize bodies for semantic comparison
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
			return &SynthesisConflict{
				UIConflict: ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s Added (identical)", kindStr),
					Status:       "Can Auto-merge",
				},
				Local:  local,
				Remote: remote,
				Base:   nil,
			}
		}
		return &SynthesisConflict{
			UIConflict: ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s '%s' Added (differs)", kindStr, name),
				Status:       "Needs Resolution",
			},
			Local:  local,
			Remote: remote,
			Base:   nil,
		}
	}

	// Case 1b: Added only in local (orphan add - for move detection)
	if base == nil && local != nil && remote == nil {
		return &SynthesisConflict{
			UIConflict: ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s '%s' Added (local)", kindStr, name),
				Status:       "Needs Resolution",
			},
			Local:  local,
			Remote: nil,
			Base:   nil,
		}
	}

	// Case 1c: Added only in remote (orphan add - for move detection)
	if base == nil && local == nil && remote != nil {
		return &SynthesisConflict{
			UIConflict: ui.Conflict{
				File:         file,
				ConflictType: fmt.Sprintf("%s '%s' Added (remote)", kindStr, name),
				Status:       "Needs Resolution",
			},
			Local:  nil,
			Remote: remote,
			Base:   nil,
		}
	}

	// Case 2: Removed in one branch, modified in other
	if base != nil {
		// Deleted on both branches (orphan delete - for move detection)
		if local == nil && remote == nil {
			return &SynthesisConflict{
				UIConflict: ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Deleted", kindStr, name),
					Status:       "Needs Resolution",
				},
				Local:  nil,
				Remote: nil,
				Base:   base,
			}
		}
		if local == nil && remote != nil && remoteNorm != baseNorm {
			return &SynthesisConflict{
				UIConflict: ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Delete/Modify", kindStr, name),
					Status:       "Needs Resolution",
				},
				Local:  nil,
				Remote: remote,
				Base:   base,
			}
		}
		if remote == nil && local != nil && localNorm != baseNorm {
			return &SynthesisConflict{
				UIConflict: ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Modify/Delete", kindStr, name),
					Status:       "Needs Resolution",
				},
				Local:  local,
				Remote: nil,
				Base:   base,
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
				return &SynthesisConflict{
					UIConflict: ui.Conflict{
						File:         file,
						ConflictType: fmt.Sprintf("%s '%s' Modified (same)", kindStr, name),
						Status:       "Can Auto-merge",
					},
					Local:  local,
					Remote: remote,
					Base:   base,
				}
			}
			// Check if semantically identical but different formatting
			if localNorm == remoteNorm {
				return &SynthesisConflict{
					UIConflict: ui.Conflict{
						File:         file,
						ConflictType: fmt.Sprintf("%s '%s' Formatted Change", kindStr, name),
						Status:       "Can Auto-merge",
					},
					Local:  local,
					Remote: remote,
					Base:   base,
				}
			}
			return &SynthesisConflict{
				UIConflict: ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Modified", kindStr, name),
					Status:       "Needs Resolution",
				},
				Local:  local,
				Remote: remote,
				Base:   base,
			}
		}

		// Only remote changed - auto-mergeable
		if remoteChanged && !localChanged {
			return &SynthesisConflict{
				UIConflict: ui.Conflict{
					File:         file,
					ConflictType: fmt.Sprintf("%s '%s' Updated (remote)", kindStr, name),
					Status:       "Can Auto-merge",
				},
				Local:  local,
				Remote: remote,
				Base:   base,
			}
		}
	}

	return nil
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// SynthesizeFile applies synthesis to rewrite the file on disk
func SynthesizeFile(analysis *SynthesisAnalysis, config MergeConfig) *SynthesisResult {
	result := &SynthesisResult{
		File:    analysis.File,
		Success: true,
	}

	if len(analysis.Conflicts) == 0 {
		result.AllAutoMerged = true
		return result
	}

	if len(analysis.LocalContent) == 0 {
		result.Success = false
		result.Error = fmt.Errorf("no local content available for synthesis")
		return result
	}

	// Check for range collisions before processing
	collisions := detectCollisions(analysis.Conflicts)
	workingConflicts := analysis.Conflicts
	if len(collisions) > 0 {
		if config.Verbose {
			ui.Warning(fmt.Sprintf("Detected %d range collision(s) in %s", len(collisions), analysis.File))
		}
		// Handle collisions by wrapping outer ranges and skipping inner conflicts
		workingConflicts = handleCollisions(analysis.Conflicts, collisions)
	}

	// Sort conflicts by start byte descending (process from end to beginning)
	sortedConflicts := make([]SynthesisConflict, len(workingConflicts))
	copy(sortedConflicts, workingConflicts)
	sort.Slice(sortedConflicts, func(i, j int) bool {
		return getConflictStartByte(&sortedConflicts[i]) > getConflictStartByte(&sortedConflicts[j])
	})

	canvas := make([]byte, len(analysis.LocalContent))
	copy(canvas, analysis.LocalContent)

	allAutoMerged := true

	for _, conflict := range sortedConflicts {
		result.ConflictCount++

		if conflict.UIConflict.Status == "Can Auto-merge" {
			result.AutoMergeCount++
			canvas = applyAutoMerge(canvas, &conflict)
		} else {
			allAutoMerged = false
			canvas = insertConflictMarkers(canvas, &conflict)
		}
	}

	result.AllAutoMerged = allAutoMerged

	// Dry-run mode: print diff but don't write
	if config.DryRun {
		printDryRunDiff(analysis.File, analysis.LocalContent, canvas)
		// In dry-run mode, don't mark as auto-merged since we didn't write anything
		result.AllAutoMerged = false
		return result
	}

	// Write result to file using atomic write with backup
	if err := atomicWriteWithBackup(analysis.File, canvas, config.CreateBackup); err != nil {
		result.Success = false
		result.Error = err
		return result
	}

	// If all conflicts were auto-merged, stage the file
	if allAutoMerged {
		if err := gitExec.Run(context.Background(), "add", analysis.File); err != nil {
			logging.Error("failed to stage file", "file", analysis.File, "error", err)
			result.Success = false
			result.Error = fmt.Errorf("failed to stage file: %w", err)
			return result
		}
	}

	return result
}

// getConflictStartByte returns the start byte position for a conflict
func getConflictStartByte(conflict *SynthesisConflict) uint32 {
	// Use local definition position if available (since we're editing local content)
	if conflict.Local != nil {
		return conflict.Local.StartByte
	}
	// Fall back to base position for deletions
	if conflict.Base != nil {
		return conflict.Base.StartByte
	}
	// Remote-only additions - append at end (return max value to sort first, but handle specially)
	return 0
}

// getConflictEndByte returns the end byte position for a conflict
func getConflictEndByte(conflict *SynthesisConflict) uint32 {
	if conflict.Local != nil {
		return conflict.Local.EndByte
	}
	if conflict.Base != nil {
		return conflict.Base.EndByte
	}
	return 0
}

// applyAutoMerge replaces the local definition with the remote one
func applyAutoMerge(canvas []byte, conflict *SynthesisConflict) []byte {
	// For identical additions or modifications where both sides are the same,
	// we keep local (no change needed)
	if conflict.Local != nil && conflict.Remote != nil {
		localNorm := normalize(conflict.Local.Body)
		remoteNorm := normalize(conflict.Remote.Body)
		if localNorm == remoteNorm {
			// Already identical, keep as-is
			return canvas
		}
	}

	// Remote-only change - replace local with remote
	if conflict.Local != nil && conflict.Remote != nil {
		startByte := conflict.Local.StartByte
		endByte := conflict.Local.EndByte

		newBody := []byte(conflict.Remote.Body)
		return replaceBytes(canvas, startByte, endByte, newBody)
	}

	// Move conflict: Local is nil (deleted), Remote has the renamed function
	// Append the remote content to the end of the file
	if conflict.Local == nil && conflict.Remote != nil {
		// Ensure we have a newline before appending
		newContent := conflict.Remote.Body
		if len(canvas) > 0 && canvas[len(canvas)-1] != '\n' {
			newContent = "\n" + newContent
		}
		// Add extra newline for separation
		if len(canvas) > 0 {
			newContent = "\n" + newContent
		}
		return append(canvas, []byte(newContent)...)
	}

	// Local-only addition (no remote) - keep local as-is
	if conflict.Local != nil && conflict.Remote == nil {
		return canvas
	}

	return canvas
}

// insertConflictMarkers inserts Git-style conflict markers
func insertConflictMarkers(canvas []byte, conflict *SynthesisConflict) []byte {
	var localBody, remoteBody string

	if conflict.Local != nil {
		localBody = conflict.Local.Body
	}
	if conflict.Remote != nil {
		remoteBody = conflict.Remote.Body
	}

	// Build conflict block
	conflictBlock := fmt.Sprintf("<<<<<<< LOCAL\n%s\n=======\n%s\n>>>>>>> REMOTE", localBody, remoteBody)

	// Determine position to insert
	var startByte, endByte uint32
	if conflict.Local != nil {
		startByte = conflict.Local.StartByte
		endByte = conflict.Local.EndByte
	} else if conflict.Base != nil {
		// Deleted locally - use base position (approximate insertion point)
		startByte = conflict.Base.StartByte
		endByte = conflict.Base.StartByte // Don't replace anything, just insert
	}

	return replaceBytes(canvas, startByte, endByte, []byte(conflictBlock))
}

// replaceBytes replaces canvas[start:end] with replacement
func replaceBytes(canvas []byte, start, end uint32, replacement []byte) []byte {
	// Bounds checking
	if start > uint32(len(canvas)) {
		start = uint32(len(canvas))
	}
	if end > uint32(len(canvas)) {
		end = uint32(len(canvas))
	}
	if start > end {
		start = end
	}

	result := make([]byte, 0, len(canvas)-int(end-start)+len(replacement))
	result = append(result, canvas[:start]...)
	result = append(result, replacement...)
	result = append(result, canvas[end:]...)
	return result
}

// detectCollisions checks for overlapping byte ranges
// Returns collisions if Conflict[i].EndByte > Conflict[i+1].StartByte
func detectCollisions(conflicts []SynthesisConflict) []RangeCollision {
	if len(conflicts) < 2 {
		return nil
	}

	// Sort by StartByte ascending for collision detection
	sorted := make([]SynthesisConflict, len(conflicts))
	copy(sorted, conflicts)
	sort.Slice(sorted, func(i, j int) bool {
		return getConflictStartByte(&sorted[i]) < getConflictStartByte(&sorted[j])
	})

	var collisions []RangeCollision
	for i := 0; i < len(sorted)-1; i++ {
		endByte := getConflictEndByte(&sorted[i])
		nextStart := getConflictStartByte(&sorted[i+1])

		if endByte > nextStart {
			collisions = append(collisions, RangeCollision{
				Outer: &sorted[i],
				Inner: &sorted[i+1],
			})
		}
	}
	return collisions
}

// handleCollisions processes detected collisions by marking inner conflicts for skipping
// and updating the outer conflict to indicate collision
func handleCollisions(conflicts []SynthesisConflict, collisions []RangeCollision) []SynthesisConflict {
	// Build sets for inner (to skip) and outer (to mark) conflict positions
	skipSet := make(map[uint32]bool)
	markSet := make(map[uint32]bool)
	for _, collision := range collisions {
		skipSet[getConflictStartByte(collision.Inner)] = true
		markSet[getConflictStartByte(collision.Outer)] = true
	}

	// Filter out inner conflicts and mark outer conflicts
	var result []SynthesisConflict
	for _, conflict := range conflicts {
		startByte := getConflictStartByte(&conflict)
		if skipSet[startByte] {
			// Skip inner conflicts that are subsumed
			continue
		}
		// Make a copy to avoid mutating the original
		resultConflict := conflict
		if markSet[startByte] {
			// Mark outer conflict as collision
			resultConflict.UIConflict.Status = "Needs Resolution"
			resultConflict.UIConflict.ConflictType += " (Collision Detected)"
		}
		result = append(result, resultConflict)
	}
	return result
}

// atomicWriteWithBackup safely writes content with backup
func atomicWriteWithBackup(filename string, content []byte, createBackup bool) error {
	// Step A: Create backup if requested
	if createBackup {
		backupPath := filename + ".orig"
		// Only create backup if original exists and backup doesn't
		if _, err := os.Stat(filename); err == nil {
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				original, err := os.ReadFile(filename)
				if err != nil {
					return formatWriteError(filename, err, "read original for backup")
				}
				if err := os.WriteFile(backupPath, original, 0644); err != nil {
					return fmt.Errorf("cannot create backup %s.orig: %w", filename, err)
				}
			}
		}
	}

	// Step B: Write to temp file in same directory (for atomic rename)
	dir := filepath.Dir(filename)
	tempFile, err := os.CreateTemp(dir, ".g2-merge-*")
	if err != nil {
		return formatWriteError(filename, err, "create temp file")
	}
	tempPath := tempFile.Name()

	// Clean up temp file on any error
	defer func() {
		if tempPath != "" {
			os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return formatWriteError(filename, err, "write temp file")
	}
	if err := tempFile.Close(); err != nil {
		return formatWriteError(filename, err, "close temp file")
	}

	// Step C: Atomic rename
	if err := os.Rename(tempPath, filename); err != nil {
		return formatWriteError(filename, err, "rename temp to target")
	}

	tempPath = "" // Prevent cleanup since rename succeeded
	return nil
}

// formatWriteError formats write errors with user-friendly messages
func formatWriteError(filename string, err error, operation string) error {
	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s: permission denied", filename)
	}
	// Check for disk full (ENOSPC)
	if strings.Contains(err.Error(), "no space left") {
		return fmt.Errorf("cannot write to %s: disk full", filename)
	}
	// Check for file locked
	if strings.Contains(err.Error(), "locked") || strings.Contains(err.Error(), "busy") {
		return fmt.Errorf("cannot write to %s: file is locked by another process", filename)
	}
	return fmt.Errorf("failed to %s for %s: %w", operation, filename, err)
}

// printDryRunDiff prints colored diff output for dry-run mode
func printDryRunDiff(filename string, original, proposed []byte) {
	fmt.Printf("\n=== Dry Run: %s ===\n", filename)

	// Simple line-by-line diff
	origLines := strings.Split(string(original), "\n")
	newLines := strings.Split(string(proposed), "\n")

	// Styles for diff output
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E07A7A"))
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7CB486"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7AA2F7")).Bold(true)

	// Find differences using a simple approach
	maxLines := len(origLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	fmt.Println(headerStyle.Render("Changes to be applied:"))
	fmt.Println()

	// Track if any changes found
	changesFound := false

	// Use a simple diff approach - show context around changes
	for i := 0; i < maxLines; i++ {
		var origLine, newLine string
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if origLine != newLine {
			changesFound = true
			if origLine != "" {
				fmt.Printf("%s\n", redStyle.Render(fmt.Sprintf("- %s", origLine)))
			}
			if newLine != "" {
				fmt.Printf("%s\n", greenStyle.Render(fmt.Sprintf("+ %s", newLine)))
			}
		}
	}

	if !changesFound {
		fmt.Println("  (no changes)")
	}

	fmt.Println()
}
