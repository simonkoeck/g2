package semantic

import (
	"fmt"

	"github.com/simonkoeck/g2/pkg/ui"
)

// MoveDetectionConfig controls move detection behavior
type MoveDetectionConfig struct {
	MinTokenCount    int     // Minimum tokens to consider for fuzzy matching (default: 10)
	FuzzyThreshold   float64 // Minimum Jaccard similarity for fuzzy match (default: 0.75)
	EnableExactMatch bool    // Enable exact body matching (default: true)
	EnableFuzzyMatch bool    // Enable fuzzy body matching (default: true)

	// Tiered thresholds based on function size
	SmallBodyTokens    int     // Bodies below this use stricter threshold (default: 20)
	SmallBodyThreshold float64 // Stricter threshold for small bodies (default: 0.85)
	LargeBodyTokens    int     // Bodies above this use lenient threshold (default: 100)
	LargeBodyThreshold float64 // Lenient threshold for large bodies (default: 0.65)
}

// DefaultMoveDetectionConfig returns default configuration
func DefaultMoveDetectionConfig() MoveDetectionConfig {
	return MoveDetectionConfig{
		MinTokenCount:      10,
		FuzzyThreshold:     0.75,
		EnableExactMatch:   true,
		EnableFuzzyMatch:   true,
		SmallBodyTokens:    20,
		SmallBodyThreshold: 0.85,
		LargeBodyTokens:    100,
		LargeBodyThreshold: 0.65,
	}
}

// getThresholdForSize returns the appropriate threshold based on token count
func getThresholdForSize(tokenCount int, config MoveDetectionConfig) float64 {
	if tokenCount < config.SmallBodyTokens {
		return config.SmallBodyThreshold
	}
	if tokenCount > config.LargeBodyTokens {
		return config.LargeBodyThreshold
	}
	return config.FuzzyThreshold
}

// DetectMoves identifies move operations among conflicts and consolidates them
// It transforms separate Delete/Add conflicts into single "Moved" conflicts
func DetectMoves(conflicts []SynthesisConflict) []SynthesisConflict {
	return DetectMovesWithConfig(conflicts, DefaultMoveDetectionConfig())
}

// DetectMovesWithConfig identifies moves with custom configuration
func DetectMovesWithConfig(conflicts []SynthesisConflict, config MoveDetectionConfig) []SynthesisConflict {
	if len(conflicts) < 2 {
		return conflicts
	}

	// Separate orphan deletes and adds from other conflicts
	orphanDeletes, orphanAdds, otherConflicts := classifyConflicts(conflicts)

	// No potential moves if we don't have both deletes and adds
	if len(orphanDeletes) == 0 || len(orphanAdds) == 0 {
		return conflicts
	}

	// Track which conflicts have been matched
	matchedDeletes := make(map[int]bool)
	matchedAdds := make(map[int]bool)
	var moveConflicts []SynthesisConflict

	// Pass 1: Exact Match
	if config.EnableExactMatch {
		exactMatches := findExactMatches(orphanDeletes, orphanAdds, matchedDeletes, matchedAdds)
		moveConflicts = append(moveConflicts, exactMatches...)
	}

	// Pass 2: Fuzzy Match
	if config.EnableFuzzyMatch {
		fuzzyMatches := findFuzzyMatches(orphanDeletes, orphanAdds, matchedDeletes, matchedAdds, config)
		moveConflicts = append(moveConflicts, fuzzyMatches...)
	}

	// Build result: other conflicts + move conflicts + unmatched orphans
	return assembleResult(otherConflicts, moveConflicts, orphanDeletes, orphanAdds, matchedDeletes, matchedAdds)
}

// classifyConflicts separates conflicts into orphan deletes, orphan adds, and others
func classifyConflicts(conflicts []SynthesisConflict) (deletes, adds []*SynthesisConflict, others []SynthesisConflict) {
	for i := range conflicts {
		c := &conflicts[i]
		if isOrphanDelete(c) {
			deletes = append(deletes, c)
		} else if isOrphanAdd(c) {
			adds = append(adds, c)
		} else {
			others = append(others, *c)
		}
	}
	return
}

// findExactMatches finds conflicts with identical bodies using hash comparison
func findExactMatches(deletes, adds []*SynthesisConflict, matchedDeletes, matchedAdds map[int]bool) []SynthesisConflict {
	var matches []SynthesisConflict

	// Build hash index of orphan adds
	addHashIndex := make(map[string][]int)
	for i, add := range adds {
		body := getAddBody(add)
		if body == "" {
			continue
		}
		hash := hashBody(body)
		addHashIndex[hash] = append(addHashIndex[hash], i)
	}

	// Match deletes to adds by hash
	for delIdx, del := range deletes {
		if matchedDeletes[delIdx] {
			continue
		}

		body := normalize(del.Base.Body)
		hash := hashBody(body)

		for _, addIdx := range addHashIndex[hash] {
			if matchedAdds[addIdx] {
				continue
			}

			add := adds[addIdx]

			// Verify kinds match
			if del.Base.Kind != getAddKind(add) {
				continue
			}

			matches = append(matches, createMoveConflict(del, add, "Exact Match", 1.0))
			matchedDeletes[delIdx] = true
			matchedAdds[addIdx] = true
			break
		}
	}

	return matches
}

// findFuzzyMatches finds conflicts with similar bodies using Jaccard similarity
func findFuzzyMatches(deletes, adds []*SynthesisConflict, matchedDeletes, matchedAdds map[int]bool, config MoveDetectionConfig) []SynthesisConflict {
	var matches []SynthesisConflict

	for delIdx, del := range deletes {
		if matchedDeletes[delIdx] {
			continue
		}

		delBody := normalize(del.Base.Body)
		delTokens := tokenize(delBody)

		// Skip small bodies (boilerplate guard)
		if len(delTokens) < config.MinTokenCount {
			continue
		}

		bestIdx, bestSimilarity := findBestFuzzyMatch(del, adds, delTokens, matchedAdds, config)

		if bestIdx >= 0 {
			add := adds[bestIdx]
			matches = append(matches, createMoveConflict(del, add, "Fuzzy Match", bestSimilarity))
			matchedDeletes[delIdx] = true
			matchedAdds[bestIdx] = true
		}
	}

	return matches
}

// findBestFuzzyMatch finds the best fuzzy match for a delete among adds
func findBestFuzzyMatch(del *SynthesisConflict, adds []*SynthesisConflict, delTokens []string, matchedAdds map[int]bool, config MoveDetectionConfig) (int, float64) {
	bestIdx := -1
	bestSimilarity := 0.0

	// Get size-appropriate threshold for the delete body
	delThreshold := getThresholdForSize(len(delTokens), config)

	for addIdx, add := range adds {
		if matchedAdds[addIdx] {
			continue
		}

		// Verify kinds match
		if del.Base.Kind != getAddKind(add) {
			continue
		}

		addBody := getAddBody(add)
		addTokens := tokenize(addBody)

		// Skip small bodies
		if len(addTokens) < config.MinTokenCount {
			continue
		}

		// Use the more restrictive threshold of the two bodies
		addThreshold := getThresholdForSize(len(addTokens), config)
		threshold := delThreshold
		if addThreshold > threshold {
			threshold = addThreshold
		}

		similarity := calculateJaccard(delTokens, addTokens)
		if similarity >= threshold && similarity > bestSimilarity {
			bestIdx = addIdx
			bestSimilarity = similarity
		}
	}

	return bestIdx, bestSimilarity
}

// assembleResult combines all conflict types into the final result
func assembleResult(others, moves []SynthesisConflict, deletes, adds []*SynthesisConflict, matchedDeletes, matchedAdds map[int]bool) []SynthesisConflict {
	result := make([]SynthesisConflict, 0, len(others)+len(moves)+len(deletes)+len(adds))
	result = append(result, others...)
	result = append(result, moves...)

	// Add unmatched orphan deletes
	for i, del := range deletes {
		if !matchedDeletes[i] {
			result = append(result, *del)
		}
	}

	// Add unmatched orphan adds
	for i, add := range adds {
		if !matchedAdds[i] {
			result = append(result, *add)
		}
	}

	return result
}

// isOrphanDelete checks if a conflict represents an orphan deletion
// (existed in base, deleted in both local and remote)
func isOrphanDelete(c *SynthesisConflict) bool {
	return c.Base != nil && c.Local == nil && c.Remote == nil
}

// isOrphanAdd checks if a conflict represents an orphan addition
// (didn't exist in base, added in local or remote)
func isOrphanAdd(c *SynthesisConflict) bool {
	return c.Base == nil && (c.Local != nil || c.Remote != nil)
}

// getAddBody returns the normalized body from an orphan add conflict
func getAddBody(c *SynthesisConflict) string {
	if c.Local != nil {
		return normalize(c.Local.Body)
	}
	if c.Remote != nil {
		return normalize(c.Remote.Body)
	}
	return ""
}

// getAddKind returns the kind from an orphan add conflict
func getAddKind(c *SynthesisConflict) string {
	if c.Local != nil {
		return c.Local.Kind
	}
	if c.Remote != nil {
		return c.Remote.Kind
	}
	return ""
}

// getAddName returns the name from an orphan add conflict
func getAddName(c *SynthesisConflict) string {
	if c.Local != nil {
		return c.Local.Name
	}
	if c.Remote != nil {
		return c.Remote.Name
	}
	return ""
}

// createMoveConflict creates a merged conflict representing a move operation
func createMoveConflict(del, add *SynthesisConflict, matchType string, similarity float64) SynthesisConflict {
	deleteName := del.Base.Name
	addName := getAddName(add)
	kind := capitalizeFirst(del.Base.Kind)

	conflictType := formatMoveConflictType(kind, deleteName, addName, matchType, similarity)

	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         del.UIConflict.File,
			ConflictType: conflictType,
			Status:       "Can Auto-merge",
		},
		Base:   del.Base,
		Local:  add.Local,
		Remote: add.Remote,
	}
}

// formatMoveConflictType formats the conflict type string for a move
func formatMoveConflictType(kind, deleteName, addName, matchType string, similarity float64) string {
	if deleteName == addName {
		if matchType == "Exact Match" {
			return fmt.Sprintf("%s '%s' Moved (Exact Match)", kind, deleteName)
		}
		return fmt.Sprintf("%s '%s' Moved (%.0f%% Match)", kind, deleteName, similarity*100)
	}

	// Different names - rename + move
	if matchType == "Exact Match" {
		return fmt.Sprintf("%s '%s' Renamed+Moved to '%s' (Exact Match)", kind, deleteName, addName)
	}
	return fmt.Sprintf("%s '%s' Renamed+Moved to '%s' (%.0f%% Match)", kind, deleteName, addName, similarity*100)
}

// InterFileMove represents a definition moved between files
type InterFileMove struct {
	SourceFile     string             // File where definition was deleted
	DestFile       string             // File where definition was added
	SourceConflict *SynthesisConflict // The orphan delete
	DestConflict   *SynthesisConflict // The orphan add
	MatchType      string             // "Exact Match" or "Fuzzy Match"
	Similarity     float64            // 1.0 for exact, 0.0-1.0 for fuzzy
}

// CrossFileOrphan represents an orphan conflict from a specific file
type CrossFileOrphan struct {
	File     string
	Conflict *SynthesisConflict
}

// DetectInterFileMoves identifies definitions moved between files
// It looks for orphan deletes in one file that match orphan adds in another file
func DetectInterFileMoves(analyses []*SynthesisAnalysis) []InterFileMove {
	return DetectInterFileMovesWithConfig(analyses, DefaultMoveDetectionConfig())
}

// DetectInterFileMovesWithConfig identifies inter-file moves with custom configuration
func DetectInterFileMovesWithConfig(analyses []*SynthesisAnalysis, config MoveDetectionConfig) []InterFileMove {
	// Collect all orphan deletes and adds across files
	var orphanDeletes, orphanAdds []CrossFileOrphan

	for _, analysis := range analyses {
		for i := range analysis.Conflicts {
			c := &analysis.Conflicts[i]
			if isOrphanDelete(c) {
				orphanDeletes = append(orphanDeletes, CrossFileOrphan{
					File:     analysis.File,
					Conflict: c,
				})
			} else if isOrphanAdd(c) {
				orphanAdds = append(orphanAdds, CrossFileOrphan{
					File:     analysis.File,
					Conflict: c,
				})
			}
		}
	}

	// No potential inter-file moves if we don't have both deletes and adds
	if len(orphanDeletes) == 0 || len(orphanAdds) == 0 {
		return nil
	}

	// Track which conflicts have been matched
	matchedDeletes := make(map[int]bool)
	matchedAdds := make(map[int]bool)
	var moves []InterFileMove

	// Pass 1: Exact Match (across different files only)
	if config.EnableExactMatch {
		exactMoves := findExactInterFileMoves(orphanDeletes, orphanAdds, matchedDeletes, matchedAdds)
		moves = append(moves, exactMoves...)
	}

	// Pass 2: Fuzzy Match (across different files only)
	if config.EnableFuzzyMatch {
		fuzzyMoves := findFuzzyInterFileMoves(orphanDeletes, orphanAdds, matchedDeletes, matchedAdds, config)
		moves = append(moves, fuzzyMoves...)
	}

	return moves
}

// findExactInterFileMoves finds inter-file moves with identical bodies
func findExactInterFileMoves(deletes, adds []CrossFileOrphan, matchedDeletes, matchedAdds map[int]bool) []InterFileMove {
	var moves []InterFileMove

	// Build hash index of orphan adds
	addHashIndex := make(map[string][]int)
	for i, add := range adds {
		body := getAddBody(add.Conflict)
		if body == "" {
			continue
		}
		hash := hashBody(body)
		addHashIndex[hash] = append(addHashIndex[hash], i)
	}

	// Match deletes to adds by hash (different files only)
	for delIdx, del := range deletes {
		if matchedDeletes[delIdx] {
			continue
		}

		body := normalize(del.Conflict.Base.Body)
		hash := hashBody(body)

		for _, addIdx := range addHashIndex[hash] {
			if matchedAdds[addIdx] {
				continue
			}

			add := adds[addIdx]

			// Only match across different files
			if del.File == add.File {
				continue
			}

			// Verify kinds match
			if del.Conflict.Base.Kind != getAddKind(add.Conflict) {
				continue
			}

			moves = append(moves, InterFileMove{
				SourceFile:     del.File,
				DestFile:       add.File,
				SourceConflict: del.Conflict,
				DestConflict:   add.Conflict,
				MatchType:      "Exact Match",
				Similarity:     1.0,
			})
			matchedDeletes[delIdx] = true
			matchedAdds[addIdx] = true
			break
		}
	}

	return moves
}

// findFuzzyInterFileMoves finds inter-file moves with similar bodies
func findFuzzyInterFileMoves(deletes, adds []CrossFileOrphan, matchedDeletes, matchedAdds map[int]bool, config MoveDetectionConfig) []InterFileMove {
	var moves []InterFileMove

	for delIdx, del := range deletes {
		if matchedDeletes[delIdx] {
			continue
		}

		delBody := normalize(del.Conflict.Base.Body)
		delTokens := tokenize(delBody)

		// Skip small bodies
		if len(delTokens) < config.MinTokenCount {
			continue
		}

		bestIdx := -1
		bestSimilarity := 0.0

		for addIdx, add := range adds {
			if matchedAdds[addIdx] {
				continue
			}

			// Only match across different files
			if del.File == add.File {
				continue
			}

			// Verify kinds match
			if del.Conflict.Base.Kind != getAddKind(add.Conflict) {
				continue
			}

			addBody := getAddBody(add.Conflict)
			addTokens := tokenize(addBody)

			// Skip small bodies
			if len(addTokens) < config.MinTokenCount {
				continue
			}

			similarity := calculateJaccard(delTokens, addTokens)
			if similarity >= config.FuzzyThreshold && similarity > bestSimilarity {
				bestIdx = addIdx
				bestSimilarity = similarity
			}
		}

		if bestIdx >= 0 {
			add := adds[bestIdx]
			moves = append(moves, InterFileMove{
				SourceFile:     del.File,
				DestFile:       add.File,
				SourceConflict: del.Conflict,
				DestConflict:   add.Conflict,
				MatchType:      "Fuzzy Match",
				Similarity:     bestSimilarity,
			})
			matchedDeletes[delIdx] = true
			matchedAdds[bestIdx] = true
		}
	}

	return moves
}

// ApplyInterFileMoves updates conflicts to reflect detected inter-file moves
func ApplyInterFileMoves(analyses []*SynthesisAnalysis, moves []InterFileMove) {
	for _, move := range moves {
		kind := capitalizeFirst(move.SourceConflict.Base.Kind)
		name := move.SourceConflict.Base.Name

		// Format the match suffix
		var matchSuffix string
		if move.MatchType == "Exact Match" {
			matchSuffix = "Exact Match"
		} else {
			matchSuffix = fmt.Sprintf("%.0f%% Match", move.Similarity*100)
		}

		// Update source conflict (the delete)
		move.SourceConflict.UIConflict.Status = "Can Auto-merge"
		move.SourceConflict.UIConflict.ConflictType = fmt.Sprintf(
			"%s '%s' Moved to %s (%s)",
			kind, name, move.DestFile, matchSuffix,
		)

		// Update dest conflict (the add)
		move.DestConflict.UIConflict.Status = "Can Auto-merge"
		move.DestConflict.UIConflict.ConflictType = fmt.Sprintf(
			"%s '%s' Moved from %s (%s)",
			kind, name, move.SourceFile, matchSuffix,
		)
	}
}
