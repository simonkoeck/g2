package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"github.com/simonkoeck/g2/pkg/ui"
)

// MoveDetectionConfig controls move detection behavior
type MoveDetectionConfig struct {
	MinTokenCount    int     // Minimum tokens to consider for fuzzy matching (default: 10)
	FuzzyThreshold   float64 // Minimum Jaccard similarity for fuzzy match (default: 0.75)
	EnableExactMatch bool    // Enable exact body matching (default: true)
	EnableFuzzyMatch bool    // Enable fuzzy body matching (default: true)
}

// DefaultMoveDetectionConfig returns default configuration
func DefaultMoveDetectionConfig() MoveDetectionConfig {
	return MoveDetectionConfig{
		MinTokenCount:    10,
		FuzzyThreshold:   0.75,
		EnableExactMatch: true,
		EnableFuzzyMatch: true,
	}
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
	var orphanDeletes []*SynthesisConflict
	var orphanAdds []*SynthesisConflict
	var otherConflicts []SynthesisConflict

	for i := range conflicts {
		c := &conflicts[i]
		if isOrphanDelete(c) {
			orphanDeletes = append(orphanDeletes, c)
		} else if isOrphanAdd(c) {
			orphanAdds = append(orphanAdds, c)
		} else {
			otherConflicts = append(otherConflicts, *c)
		}
	}

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
		// Build hash index of orphan adds
		addHashIndex := make(map[string][]int) // hash -> indices in orphanAdds
		for i, add := range orphanAdds {
			body := getAddBody(add)
			if body == "" {
				continue
			}
			hash := hashBody(body)
			addHashIndex[hash] = append(addHashIndex[hash], i)
		}

		// Match deletes to adds by hash
		for delIdx, del := range orphanDeletes {
			if matchedDeletes[delIdx] {
				continue
			}

			body := normalize(del.Base.Body)
			hash := hashBody(body)

			// Find matching add
			for _, addIdx := range addHashIndex[hash] {
				if matchedAdds[addIdx] {
					continue
				}

				add := orphanAdds[addIdx]

				// Verify kinds match
				if del.Base.Kind != getAddKind(add) {
					continue
				}

				// Create move conflict
				moveConflict := createMoveConflict(del, add, "Exact Match", 1.0)
				moveConflicts = append(moveConflicts, moveConflict)

				matchedDeletes[delIdx] = true
				matchedAdds[addIdx] = true
				break
			}
		}
	}

	// Pass 2: Fuzzy Match
	if config.EnableFuzzyMatch {
		for delIdx, del := range orphanDeletes {
			if matchedDeletes[delIdx] {
				continue
			}

			delBody := normalize(del.Base.Body)
			delTokens := tokenize(delBody)

			// Skip small bodies (boilerplate guard)
			if len(delTokens) < config.MinTokenCount {
				continue
			}

			var bestMatch struct {
				addIdx     int
				similarity float64
			}
			bestMatch.addIdx = -1

			for addIdx, add := range orphanAdds {
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

				similarity := calculateJaccard(delTokens, addTokens)
				if similarity >= config.FuzzyThreshold && similarity > bestMatch.similarity {
					bestMatch.addIdx = addIdx
					bestMatch.similarity = similarity
				}
			}

			if bestMatch.addIdx >= 0 {
				add := orphanAdds[bestMatch.addIdx]
				moveConflict := createMoveConflict(del, add, "Fuzzy Match", bestMatch.similarity)
				moveConflicts = append(moveConflicts, moveConflict)

				matchedDeletes[delIdx] = true
				matchedAdds[bestMatch.addIdx] = true
			}
		}
	}

	// Build result: other conflicts + move conflicts + unmatched orphans
	result := make([]SynthesisConflict, 0, len(conflicts))
	result = append(result, otherConflicts...)
	result = append(result, moveConflicts...)

	// Add unmatched orphan deletes
	for i, del := range orphanDeletes {
		if !matchedDeletes[i] {
			result = append(result, *del)
		}
	}

	// Add unmatched orphan adds
	for i, add := range orphanAdds {
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

// hashBody returns SHA-256 hash of normalized body for exact matching
func hashBody(body string) string {
	normalized := normalize(body)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// tokenize splits a string into alphanumeric token sequences
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}

	// Don't forget the last token
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// calculateJaccard computes Jaccard similarity coefficient between two token sets
func calculateJaccard(tokens1, tokens2 []string) float64 {
	if len(tokens1) == 0 && len(tokens2) == 0 {
		return 1.0
	}
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0
	}

	// Build sets
	set1 := make(map[string]bool)
	for _, t := range tokens1 {
		set1[t] = true
	}

	set2 := make(map[string]bool)
	for _, t := range tokens2 {
		set2[t] = true
	}

	// Calculate intersection
	intersection := 0
	for t := range set1 {
		if set2[t] {
			intersection++
		}
	}

	// Calculate union
	union := len(set1)
	for t := range set2 {
		if !set1[t] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// createMoveConflict creates a merged conflict representing a move operation
func createMoveConflict(del, add *SynthesisConflict, matchType string, similarity float64) SynthesisConflict {
	deleteName := del.Base.Name
	addName := getAddName(add)
	kind := capitalizeFirst(del.Base.Kind)

	var conflictType string
	if deleteName == addName {
		// Same name - simple move
		if matchType == "Exact Match" {
			conflictType = fmt.Sprintf("%s '%s' Moved (Exact Match)", kind, deleteName)
		} else {
			conflictType = fmt.Sprintf("%s '%s' Moved (%.0f%% Match)", kind, deleteName, similarity*100)
		}
	} else {
		// Different names - rename + move
		if matchType == "Exact Match" {
			conflictType = fmt.Sprintf("%s '%s' Renamed+Moved to '%s' (Exact Match)", kind, deleteName, addName)
		} else {
			conflictType = fmt.Sprintf("%s '%s' Renamed+Moved to '%s' (%.0f%% Match)", kind, deleteName, addName, similarity*100)
		}
	}

	return SynthesisConflict{
		UIConflict: ui.Conflict{
			File:         del.UIConflict.File,
			ConflictType: conflictType,
			Status:       "Can Auto-merge",
		},
		Base:   del.Base,   // Source location (the delete)
		Local:  add.Local,  // Destination (the add)
		Remote: add.Remote, // Destination (the add)
	}
}
