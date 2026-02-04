// Package output provides JSON output formatting for G2.
package output

import (
	"encoding/json"
	"io"
	"os"
)

// FileResult contains the merge result for a single file.
type FileResult struct {
	File           string `json:"file"`
	ConflictCount  int    `json:"conflict_count"`
	ResolvedCount  int    `json:"resolved_count"`
	AllAutoMerged  bool   `json:"all_auto_merged"`
	HasMarkers     bool   `json:"has_markers"`
	Error          string `json:"error,omitempty"`
}

// MergeResult contains the overall merge result.
type MergeResult struct {
	Success        bool         `json:"success"`
	TotalConflicts int          `json:"total_conflicts"`
	ResolvedCount  int          `json:"resolved_count"`
	Files          []FileResult `json:"files"`
	Error          string       `json:"error,omitempty"`
	DryRun         bool         `json:"dry_run,omitempty"`
}

// NewMergeResult creates a new empty MergeResult.
func NewMergeResult() *MergeResult {
	return &MergeResult{
		Files: make([]FileResult, 0),
	}
}

// AddFileResult adds a file result to the merge result.
func (r *MergeResult) AddFileResult(file FileResult) {
	r.Files = append(r.Files, file)
	r.TotalConflicts += file.ConflictCount
	r.ResolvedCount += file.ResolvedCount
}

// SetError sets the error message.
func (r *MergeResult) SetError(err error) {
	if err != nil {
		r.Error = err.Error()
	}
}

// Finalize calculates the final success state.
func (r *MergeResult) Finalize() {
	r.Success = r.Error == "" && r.TotalConflicts == r.ResolvedCount
}

// WriteJSON writes the merge result as JSON to the given writer.
func WriteJSON(w io.Writer, result *MergeResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// WriteJSONStdout writes the merge result as JSON to stdout.
func WriteJSONStdout(result *MergeResult) error {
	return WriteJSON(os.Stdout, result)
}

// MarshalJSON returns the JSON encoding of the merge result.
func (r *MergeResult) MarshalJSON() ([]byte, error) {
	type Alias MergeResult
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}
