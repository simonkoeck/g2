package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

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

// Common programming keywords that should have lower weight in similarity matching
var commonKeywords = map[string]bool{
	// Control flow
	"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "break": true, "continue": true, "return": true,
	"try": true, "catch": true, "finally": true, "throw": true, "throws": true,
	// Declarations
	"var": true, "let": true, "const": true, "function": true, "class": true,
	"interface": true, "struct": true, "enum": true, "type": true, "def": true,
	"fn": true, "func": true, "pub": true, "private": true, "public": true,
	"protected": true, "static": true, "final": true, "abstract": true,
	// Types
	"int": true, "string": true, "bool": true, "float": true, "double": true,
	"void": true, "null": true, "nil": true, "true": true, "false": true,
	"None": true, "True": true, "False": true, "undefined": true,
	// Other common
	"import": true, "from": true, "export": true, "package": true, "module": true,
	"new": true, "this": true, "self": true, "super": true, "async": true, "await": true,
	"err": true, "error": true, "ctx": true, "ok": true, "i": true, "j": true, "k": true,
}

// calculateWeightedJaccard computes weighted Jaccard similarity
// Unique/rare tokens get higher weight than common keywords
func calculateWeightedJaccard(tokens1, tokens2 []string) float64 {
	if len(tokens1) == 0 && len(tokens2) == 0 {
		return 1.0
	}
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0
	}

	// Build frequency maps
	freq1 := make(map[string]int)
	for _, t := range tokens1 {
		freq1[t]++
	}

	freq2 := make(map[string]int)
	for _, t := range tokens2 {
		freq2[t]++
	}

	// Calculate weighted intersection and union
	var intersectionWeight, unionWeight float64

	// Process all unique tokens
	allTokens := make(map[string]bool)
	for t := range freq1 {
		allTokens[t] = true
	}
	for t := range freq2 {
		allTokens[t] = true
	}

	for token := range allTokens {
		weight := 1.0
		// Common keywords get lower weight
		if commonKeywords[token] {
			weight = 0.3
		}
		// Single-character tokens get lower weight
		if len(token) == 1 {
			weight = 0.2
		}

		count1 := freq1[token]
		count2 := freq2[token]

		// Min for intersection, max for union (weighted min-max similarity)
		minCount := count1
		if count2 < minCount {
			minCount = count2
		}
		maxCount := count1
		if count2 > maxCount {
			maxCount = count2
		}

		intersectionWeight += float64(minCount) * weight
		unionWeight += float64(maxCount) * weight
	}

	if unionWeight == 0 {
		return 0.0
	}

	return intersectionWeight / unionWeight
}

// Regex patterns for comment stripping
var (
	// C-style single line comments: // ...
	cStyleSingleLineRe = regexp.MustCompile(`//[^\n]*`)
	// C-style multi-line comments: /* ... */
	cStyleMultiLineRe = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	// Python single line comments: # ...
	pythonCommentRe = regexp.MustCompile(`#[^\n]*`)
)

// stripComments removes comments from code based on language
func stripComments(body string, lang Language) string {
	switch lang {
	case LangPython:
		return stripPythonComments(body)
	case LangGo, LangRust, LangJavaScript, LangTypeScript:
		return stripCStyleComments(body)
	default:
		return body
	}
}

// stripPythonComments removes Python # comments
// Note: This is a simple implementation that doesn't handle # inside strings
func stripPythonComments(body string) string {
	return pythonCommentRe.ReplaceAllString(body, "")
}

// stripCStyleComments removes // and /* */ comments
// Note: This is a simple implementation that doesn't handle comments inside strings
func stripCStyleComments(body string) string {
	// First remove multi-line comments
	result := cStyleMultiLineRe.ReplaceAllString(body, "")
	// Then remove single-line comments
	result = cStyleSingleLineRe.ReplaceAllString(result, "")
	return result
}

// normalizeForLanguage provides enhanced normalization for semantic comparison
// It strips comments and normalizes whitespace for language-aware comparison
func normalizeForLanguage(body string, lang Language) string {
	// Strip comments first
	stripped := stripComments(body, lang)
	// Normalize whitespace
	normalized := normalize(stripped)
	// Remove optional semicolons (helps with JS/TS)
	normalized = strings.ReplaceAll(normalized, ";", "")
	return normalized
}

// normalize collapses all whitespace to single spaces for semantic comparison
// This is the basic normalizer - use normalizeForLanguage for enhanced comparison
func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
