package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/simonkoeck/g2/pkg/exitcode"
	"github.com/simonkoeck/g2/pkg/git"
	"github.com/simonkoeck/g2/pkg/output"
)

// mockExitError is a mock error that mimics exec.ExitError with a specific exit code
type mockExitError struct {
	code int
}

func (e *mockExitError) Error() string {
	return "exit status " + string(rune('0'+e.code))
}

func (e *mockExitError) ExitCode() int {
	return e.code
}

// errWithExitCode creates an error that has an ExitCode() method
func errWithExitCode(code int) error {
	return &mockExitError{code: code}
}

// errConflict represents a merge conflict (exit code 1)
var errConflict = errors.New("merge conflict")

func TestSmartMerge_NotGitRepo(t *testing.T) {
	mock := git.NewMockExecutor()
	// Configure mock to fail rev-parse (not a git repo)
	mock.OnRun = func(ctx context.Context, args []string) error {
		if len(args) > 0 && args[0] == "rev-parse" {
			return git.ErrNotGitRepo
		}
		return nil
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	if exitCode != exitcode.NotGitRepo {
		t.Errorf("expected exit code %d, got %d", exitcode.NotGitRepo, exitCode)
	}

	// Verify rev-parse was called
	calls := mock.Calls()
	found := false
	for _, call := range calls {
		if len(call.Args) > 0 && call.Args[0] == "rev-parse" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected rev-parse to be called")
	}
}

func TestSmartMerge_NoConflicts(t *testing.T) {
	mock := git.NewMockExecutor()
	// git rev-parse succeeds (is a git repo)
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil // All Run calls succeed
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return []byte("/repo"), nil
		}
		return nil, nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil // Merge succeeds (no conflicts)
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}

	// Verify RunWithStdio was called (for the merge)
	calls := mock.Calls()
	foundMerge := false
	for _, call := range calls {
		if call.Method == "RunWithStdio" {
			foundMerge = true
			break
		}
	}
	if !foundMerge {
		t.Error("expected RunWithStdio to be called for merge")
	}
}

func TestSmartMerge_WithConflicts(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil // All Run calls succeed (git add, etc.)
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) == 0 {
			return nil, nil
		}
		switch args[0] {
		case "rev-parse":
			return []byte("/repo\n"), nil
		case "diff":
			return []byte("test.py"), nil
		case "show":
			return []byte("def foo():\n    pass"), nil
		}
		return nil, nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		// Merge returns exit code 1 (conflicts)
		return errWithExitCode(1)
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	// Should return success since the file content is identical in all branches
	// (mock returns same content for all stages - auto-merge case)
	if exitCode != exitcode.Success && exitCode != exitcode.ConflictsRemain {
		t.Errorf("expected exit code %d or %d, got %d", exitcode.Success, exitcode.ConflictsRemain, exitCode)
	}
}

func TestSmartMerge_DryRun(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil // All Run calls succeed (git merge --abort, etc.)
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) == 0 {
			return nil, nil
		}
		switch args[0] {
		case "rev-parse":
			return []byte("/repo\n"), nil
		case "diff":
			return []byte("test.py"), nil
		case "show":
			return []byte("def foo():\n    pass"), nil
		}
		return nil, nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		// Merge returns exit code 1 (conflicts)
		return errWithExitCode(1)
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "--dry-run", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}

	// Verify merge --abort was called
	calls := mock.Calls()
	foundAbort := false
	for _, call := range calls {
		if call.Method == "Run" && len(call.Args) >= 2 &&
			call.Args[0] == "merge" && call.Args[1] == "--abort" {
			foundAbort = true
			break
		}
	}
	if !foundAbort {
		t.Error("expected merge --abort to be called in dry-run mode")
	}
}

func TestSmartMerge_Timeout(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil // rev-parse succeeds
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return context.DeadlineExceeded // Merge times out
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	if exitCode != exitcode.TimeoutError {
		t.Errorf("expected exit code %d, got %d", exitcode.TimeoutError, exitCode)
	}
}

func TestSmartMerge_JSONOutput_NotGitRepo(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		if len(args) > 0 && args[0] == "rev-parse" {
			return git.ErrNotGitRepo
		}
		return nil
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "--json", "feature-branch"}, mock)

	if exitCode != exitcode.NotGitRepo {
		t.Errorf("expected exit code %d, got %d", exitcode.NotGitRepo, exitCode)
	}
}

func TestSmartMerge_JSONOutput_NoConflicts(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil // Merge succeeds
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "--json", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestMergeResultJSON(t *testing.T) {
	result := output.NewMergeResult()
	result.Success = true
	result.TotalConflicts = 3
	result.ResolvedCount = 2
	result.AddFileResult(output.FileResult{
		File:           "test.py",
		ConflictCount:  2,
		ResolvedCount:  1,
		AllAutoMerged:  false,
		HasMarkers:     true,
	})
	result.AddFileResult(output.FileResult{
		File:           "utils.py",
		ConflictCount:  1,
		ResolvedCount:  1,
		AllAutoMerged:  true,
		HasMarkers:     false,
	})

	var buf bytes.Buffer
	err := output.WriteJSON(&buf, result)
	if err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	// Verify JSON is valid
	var decoded output.MergeResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if decoded.Success != result.Success {
		t.Errorf("expected success=%v, got %v", result.Success, decoded.Success)
	}
	if len(decoded.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(decoded.Files))
	}
}

func TestPassthroughNotCalledForMerge(t *testing.T) {
	// This test verifies that merge commands don't trigger passthrough
	// We can't easily test passthrough itself since it uses syscall.Exec

	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil // Merge succeeds
	}

	// If this doesn't panic/exit, passthrough wasn't called
	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestExitCodeConstants(t *testing.T) {
	// Verify exit code values match documented conventions
	if exitcode.Success != 0 {
		t.Errorf("Success should be 0, got %d", exitcode.Success)
	}
	if exitcode.ConflictsRemain != 1 {
		t.Errorf("ConflictsRemain should be 1, got %d", exitcode.ConflictsRemain)
	}
	if exitcode.GitError != 2 {
		t.Errorf("GitError should be 2, got %d", exitcode.GitError)
	}
	if exitcode.TimeoutError != 3 {
		t.Errorf("TimeoutError should be 3, got %d", exitcode.TimeoutError)
	}
	if exitcode.NotGitRepo != 128 {
		t.Errorf("NotGitRepo should be 128, got %d", exitcode.NotGitRepo)
	}
}
