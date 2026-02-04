// Package exitcode defines standard exit codes for G2.
package exitcode

const (
	// Success indicates all conflicts were auto-resolved.
	Success = 0

	// ConflictsRemain indicates unresolved conflicts exist.
	ConflictsRemain = 1

	// GitError indicates a git command failed.
	GitError = 2

	// TimeoutError indicates an operation timed out.
	TimeoutError = 3

	// NotGitRepo indicates the command was run outside a git repository.
	// This matches git's convention for this error.
	NotGitRepo = 128
)
