package semantic

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/simonkoeck/g2/pkg/logging"
)

// ImportUpdate represents a required import change in a file
type ImportUpdate struct {
	File       string // File that needs import updated
	OldImport  string // Original import statement
	NewImport  string // Updated import statement
	LineNumber int    // Line number of the import (1-based)
	StartByte  int    // Start byte offset
	EndByte    int    // End byte offset
	Definition string // Name of the definition that moved
	FromModule string // Original module
	ToModule   string // New module
}

// ImportUpdateResult contains the outcome of applying import updates
type ImportUpdateResult struct {
	File         string
	UpdatesCount int
	Error        error
}

// FindImportUpdates scans the repository for files that need import updates
// based on detected inter-file moves
func FindImportUpdates(moves []InterFileMove, repoRoot string) ([]ImportUpdate, error) {
	if len(moves) == 0 {
		return nil, nil
	}

	// Build a map of what moved where
	// key: (sourceModule, defName), value: destModule
	moveMap := make(map[struct{ sourceModule, defName string }]string)

	for _, move := range moves {
		sourceModule := fileToModule(move.SourceFile)
		destModule := fileToModule(move.DestFile)
		defName := move.SourceConflict.Base.Name

		moveMap[struct{ sourceModule, defName string }{sourceModule, defName}] = destModule
	}

	// Find all source files to scan
	var filesToScan []string
	var walkErrors []error
	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logging.Debug("walk error", "path", path, "error", err)
			walkErrors = append(walkErrors, err)
			return nil // Skip errors but record them
		}
		if info.IsDir() {
			// Skip hidden directories and common non-source directories
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "venv" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		// Only scan supported languages
		if IsSemanticFile(path) {
			relPath, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				logging.Debug("failed to get relative path", "path", path, "error", relErr)
				return nil
			}
			filesToScan = append(filesToScan, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan repository: %w", err)
	}
	if len(walkErrors) > 0 {
		logging.Warn("encountered errors while scanning repository", "error_count", len(walkErrors))
	}

	var updates []ImportUpdate

	for _, file := range filesToScan {
		fullPath := filepath.Join(repoRoot, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			logging.Debug("failed to read file for import scanning", "file", file, "error", err)
			continue // Skip files we can't read
		}

		lang := DetectLanguage(file)
		var fileUpdates []ImportUpdate

		switch lang {
		case LangPython:
			fileUpdates = findPythonImportUpdates(file, content, moveMap)
		case LangJavaScript, LangTypeScript:
			fileUpdates = findJSImportUpdates(file, content, moveMap)
		}

		updates = append(updates, fileUpdates...)
	}

	return updates, nil
}

// fileToModule converts a file path to a module name
// e.g., "utils.py" -> "utils", "foo/bar.py" -> "foo.bar"
func fileToModule(filePath string) string {
	// Remove extension
	ext := filepath.Ext(filePath)
	module := strings.TrimSuffix(filePath, ext)

	// Convert path separators to dots for Python-style modules
	module = strings.ReplaceAll(module, string(filepath.Separator), ".")
	module = strings.ReplaceAll(module, "/", ".")

	return module
}

// moduleToFile converts a module name back to a file path (Python)
func moduleToFile(module string, lang Language) string {
	switch lang {
	case LangPython:
		return strings.ReplaceAll(module, ".", string(filepath.Separator)) + ".py"
	case LangJavaScript:
		return strings.ReplaceAll(module, ".", string(filepath.Separator)) + ".js"
	case LangTypeScript:
		return strings.ReplaceAll(module, ".", string(filepath.Separator)) + ".ts"
	default:
		return module
	}
}

// Python import patterns
var (
	// from module import name, name2, ...
	pyFromImportRe = regexp.MustCompile(`^(\s*from\s+)([\w.]+)(\s+import\s+)(.+)$`)
	// import module
	pyImportRe = regexp.MustCompile(`^(\s*import\s+)([\w.]+)(.*)$`)
)

// findPythonImportUpdates finds imports that need updating in a Python file
func findPythonImportUpdates(file string, content []byte, moveMap map[struct{ sourceModule, defName string }]string) []ImportUpdate {
	var updates []ImportUpdate

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	byteOffset := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lineBytes := len(scanner.Bytes()) + 1 // +1 for newline

		// Check "from X import Y" style
		if matches := pyFromImportRe.FindStringSubmatch(line); matches != nil {
			prefix := matches[1]   // "from "
			module := matches[2]   // module name
			importKw := matches[3] // " import "
			names := matches[4]    // imported names

			// Parse the imported names
			importedNames := parseImportNames(names)

			// Check if any imported name was moved
			var movedNames []string
			var remainingNames []string
			destModules := make(map[string][]string) // destModule -> names to import

			for _, name := range importedNames {
				key := struct{ sourceModule, defName string }{module, name}
				if destModule, moved := moveMap[key]; moved {
					movedNames = append(movedNames, name)
					destModules[destModule] = append(destModules[destModule], name)
				} else {
					remainingNames = append(remainingNames, name)
				}
			}

			if len(movedNames) > 0 {
				// Build the new import statement(s)
				var newImport string

				// Keep remaining names in original import
				if len(remainingNames) > 0 {
					newImport = prefix + module + importKw + strings.Join(remainingNames, ", ")
				}

				// Add new imports for moved names
				for destModule, names := range destModules {
					if newImport != "" {
						newImport += "\n"
					}
					newImport += "from " + destModule + " import " + strings.Join(names, ", ")
				}

				updates = append(updates, ImportUpdate{
					File:       file,
					OldImport:  line,
					NewImport:  newImport,
					LineNumber: lineNum,
					StartByte:  byteOffset,
					EndByte:    byteOffset + len(line),
					Definition: strings.Join(movedNames, ", "),
					FromModule: module,
					ToModule:   getFirstKey(destModules),
				})
			}
		}

		byteOffset += lineBytes
	}

	return updates
}

// parseImportNames parses "name1, name2 as alias, name3" into ["name1", "name2", "name3"]
func parseImportNames(names string) []string {
	var result []string
	parts := strings.Split(names, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Handle "name as alias" - extract just the name
		if idx := strings.Index(part, " as "); idx > 0 {
			part = part[:idx]
		}
		// Handle trailing comments
		if idx := strings.Index(part, "#"); idx > 0 {
			part = strings.TrimSpace(part[:idx])
		}
		if part != "" {
			result = append(result, strings.TrimSpace(part))
		}
	}
	return result
}

// JS/TS import patterns
var (
	// import { name, name2 } from 'module'
	jsNamedImportRe = regexp.MustCompile(`^(\s*import\s*\{)([^}]+)(\}\s*from\s*['"])([^'"]+)(['"].*)$`)
	// import name from 'module'
	jsDefaultImportRe = regexp.MustCompile(`^(\s*import\s+)(\w+)(\s+from\s*['"])([^'"]+)(['"].*)$`)
)

// findJSImportUpdates finds imports that need updating in a JS/TS file
func findJSImportUpdates(file string, content []byte, moveMap map[struct{ sourceModule, defName string }]string) []ImportUpdate {
	var updates []ImportUpdate

	// Get the directory of the current file for relative import resolution
	fileDir := filepath.Dir(file)

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	byteOffset := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lineBytes := len(scanner.Bytes()) + 1

		// Check named imports: import { x, y } from 'module'
		if matches := jsNamedImportRe.FindStringSubmatch(line); matches != nil {
			prefix := matches[1]     // "import {"
			names := matches[2]      // "name, name2"
			middle := matches[3]     // "} from '"
			modulePath := matches[4] // module path
			suffix := matches[5]     // "'" or "';"

			// Resolve the module path to a normalized form
			resolvedModule := resolveJSModulePath(fileDir, modulePath)

			// Parse the imported names
			importedNames := parseJSImportNames(names)

			// Check if any imported name was moved
			var movedNames []string
			var remainingNames []string
			destModules := make(map[string][]string)

			for _, name := range importedNames {
				key := struct{ sourceModule, defName string }{resolvedModule, name}
				if destModule, moved := moveMap[key]; moved {
					movedNames = append(movedNames, name)
					// Convert back to relative path for the import
					relPath := makeRelativeJSImport(fileDir, destModule)
					destModules[relPath] = append(destModules[relPath], name)
				} else {
					remainingNames = append(remainingNames, name)
				}
			}

			if len(movedNames) > 0 {
				var newImport string

				// Keep remaining names in original import
				if len(remainingNames) > 0 {
					newImport = prefix + " " + strings.Join(remainingNames, ", ") + " " + middle + modulePath + suffix
				}

				// Add new imports for moved names
				for destPath, names := range destModules {
					if newImport != "" {
						newImport += "\n"
					}
					newImport += "import { " + strings.Join(names, ", ") + " } from '" + destPath + "'"
				}

				updates = append(updates, ImportUpdate{
					File:       file,
					OldImport:  line,
					NewImport:  newImport,
					LineNumber: lineNum,
					StartByte:  byteOffset,
					EndByte:    byteOffset + len(line),
					Definition: strings.Join(movedNames, ", "),
					FromModule: modulePath,
					ToModule:   getFirstKey(destModules),
				})
			}
		}

		byteOffset += lineBytes
	}

	return updates
}

// parseJSImportNames parses "name, name2 as alias" into ["name", "name2"]
func parseJSImportNames(names string) []string {
	var result []string
	parts := strings.Split(names, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Handle "name as alias" - extract just the name
		if idx := strings.Index(part, " as "); idx > 0 {
			part = part[:idx]
		}
		if part != "" {
			result = append(result, strings.TrimSpace(part))
		}
	}
	return result
}

// resolveJSModulePath resolves a JS import path to a normalized module name
func resolveJSModulePath(fromDir, importPath string) string {
	// Handle relative imports
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		resolved := filepath.Join(fromDir, importPath)
		resolved = filepath.Clean(resolved)
		// Remove leading ./
		resolved = strings.TrimPrefix(resolved, "./")
		return fileToModule(resolved)
	}
	// Non-relative imports (node_modules, etc.) - return as-is
	return importPath
}

// makeRelativeJSImport creates a relative import path from one file to another
func makeRelativeJSImport(fromDir, toModule string) string {
	toPath := moduleToFile(toModule, LangJavaScript)
	toPath = strings.TrimSuffix(toPath, ".js") // JS imports often omit extension

	rel, err := filepath.Rel(fromDir, toPath)
	if err != nil {
		return "./" + toPath
	}

	// Ensure it starts with ./ or ../
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}

	return rel
}

// getFirstKey returns the first key from a map (for single-destination moves)
func getFirstKey(m map[string][]string) string {
	for k := range m {
		return k
	}
	return ""
}

// ApplyImportUpdates applies the import updates to files
func ApplyImportUpdates(updates []ImportUpdate, repoRoot string) []ImportUpdateResult {
	// Group updates by file
	updatesByFile := make(map[string][]ImportUpdate)
	for _, update := range updates {
		updatesByFile[update.File] = append(updatesByFile[update.File], update)
	}

	var results []ImportUpdateResult

	for file, fileUpdates := range updatesByFile {
		fullPath := filepath.Join(repoRoot, file)
		result := applyFileImportUpdates(fullPath, fileUpdates)
		result.File = file
		results = append(results, result)
	}

	return results
}

// applyFileImportUpdates applies import updates to a single file
func applyFileImportUpdates(filePath string, updates []ImportUpdate) ImportUpdateResult {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ImportUpdateResult{Error: fmt.Errorf("failed to read file: %w", err)}
	}

	lines := strings.Split(string(content), "\n")

	// Sort updates by line number descending (apply from bottom to top)
	// to avoid offset issues
	for i := len(updates) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			if updates[j].LineNumber < updates[j+1].LineNumber {
				updates[j], updates[j+1] = updates[j+1], updates[j]
			}
		}
	}

	// Apply updates
	for _, update := range updates {
		lineIdx := update.LineNumber - 1 // Convert to 0-based
		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}

		// Handle multi-line new imports
		newLines := strings.Split(update.NewImport, "\n")
		if len(newLines) == 1 {
			lines[lineIdx] = update.NewImport
		} else {
			// Replace single line with multiple lines
			newSlice := make([]string, 0, len(lines)+len(newLines)-1)
			newSlice = append(newSlice, lines[:lineIdx]...)
			newSlice = append(newSlice, newLines...)
			newSlice = append(newSlice, lines[lineIdx+1:]...)
			lines = newSlice
		}
	}

	// Write back
	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return ImportUpdateResult{Error: fmt.Errorf("failed to write file: %w", err)}
	}

	return ImportUpdateResult{UpdatesCount: len(updates)}
}

// FormatImportUpdateSummary returns a human-readable summary of import updates
func FormatImportUpdateSummary(updates []ImportUpdate) string {
	if len(updates) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d import(s) to update:\n", len(updates)))

	// Group by file
	byFile := make(map[string][]ImportUpdate)
	for _, u := range updates {
		byFile[u.File] = append(byFile[u.File], u)
	}

	for file, fileUpdates := range byFile {
		sb.WriteString(fmt.Sprintf("  %s:\n", file))
		for _, u := range fileUpdates {
			sb.WriteString(fmt.Sprintf("    Line %d: %s -> %s\n", u.LineNumber, u.FromModule, u.ToModule))
		}
	}

	return sb.String()
}
