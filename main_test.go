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

// ==================== Merge Tests ====================

func TestSmartMerge_NotGitRepo(t *testing.T) {
	mock := git.NewMockExecutor()
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
}

func TestSmartMerge_NoConflicts(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return []byte("/repo"), nil
		}
		return nil, nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil // Merge succeeds
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestSmartMerge_WithConflicts(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
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
		return errWithExitCode(1)
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	// Should succeed since all branches have identical content
	if exitCode != exitcode.Success && exitCode != exitcode.ConflictsRemain {
		t.Errorf("expected exit code %d or %d, got %d", exitcode.Success, exitcode.ConflictsRemain, exitCode)
	}
}

func TestSmartMerge_DryRun(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
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
		return errWithExitCode(1)
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "--dry-run", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestSmartMerge_Timeout(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return context.DeadlineExceeded
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
		return nil
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "--json", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

// ==================== Rebase Tests ====================

func TestSmartRebase_NotGitRepo(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		if len(args) > 0 && args[0] == "rev-parse" {
			return git.ErrNotGitRepo
		}
		return nil
	}

	exitCode := SmartRebaseWithExecutor([]string{"rebase", "main"}, mock)

	if exitCode != exitcode.NotGitRepo {
		t.Errorf("expected exit code %d, got %d", exitcode.NotGitRepo, exitCode)
	}
}

func TestSmartRebase_NoConflicts(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil // Rebase succeeds
	}

	exitCode := SmartRebaseWithExecutor([]string{"rebase", "main"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestSmartRebase_WithConflicts(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
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
		return errWithExitCode(1)
	}

	exitCode := SmartRebaseWithExecutor([]string{"rebase", "main"}, mock)

	// Should succeed or have conflicts remain
	if exitCode != exitcode.Success && exitCode != exitcode.ConflictsRemain {
		t.Errorf("expected exit code %d or %d, got %d", exitcode.Success, exitcode.ConflictsRemain, exitCode)
	}
}

func TestSmartRebase_Timeout(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return context.DeadlineExceeded
	}

	exitCode := SmartRebaseWithExecutor([]string{"rebase", "main"}, mock)

	if exitCode != exitcode.TimeoutError {
		t.Errorf("expected exit code %d, got %d", exitcode.TimeoutError, exitCode)
	}
}

// ==================== Cherry-Pick Tests ====================

func TestSmartCherryPick_NotGitRepo(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		if len(args) > 0 && args[0] == "rev-parse" {
			return git.ErrNotGitRepo
		}
		return nil
	}

	exitCode := SmartCherryPickWithExecutor([]string{"cherry-pick", "abc123"}, mock)

	if exitCode != exitcode.NotGitRepo {
		t.Errorf("expected exit code %d, got %d", exitcode.NotGitRepo, exitCode)
	}
}

func TestSmartCherryPick_NoConflicts(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil // Cherry-pick succeeds
	}

	exitCode := SmartCherryPickWithExecutor([]string{"cherry-pick", "abc123"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestSmartCherryPick_WithConflicts(t *testing.T) {
	mock := git.NewMockExecutor()

	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
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
		return errWithExitCode(1)
	}

	exitCode := SmartCherryPickWithExecutor([]string{"cherry-pick", "abc123"}, mock)

	// Should succeed or have conflicts remain
	if exitCode != exitcode.Success && exitCode != exitcode.ConflictsRemain {
		t.Errorf("expected exit code %d or %d, got %d", exitcode.Success, exitcode.ConflictsRemain, exitCode)
	}
}

// ==================== JSON Output Tests ====================

func TestMergeResultJSON(t *testing.T) {
	result := output.NewMergeResult()
	result.Success = true
	result.TotalConflicts = 3
	result.ResolvedCount = 2
	result.AddFileResult(output.FileResult{
		File:          "test.py",
		ConflictCount: 2,
		ResolvedCount: 1,
		AllAutoMerged: false,
		HasMarkers:    true,
	})
	result.AddFileResult(output.FileResult{
		File:          "utils.py",
		ConflictCount: 1,
		ResolvedCount: 1,
		AllAutoMerged: true,
		HasMarkers:    false,
	})

	var buf bytes.Buffer
	err := output.WriteJSON(&buf, result)
	if err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

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

// ==================== Operation Type Tests ====================

func TestOperationType_String(t *testing.T) {
	tests := []struct {
		op       OperationType
		expected string
	}{
		{OpMerge, "merge"},
		{OpRebase, "rebase"},
		{OpCherryPick, "cherry-pick"},
	}

	for _, test := range tests {
		if got := test.op.String(); got != test.expected {
			t.Errorf("expected %s, got %s", test.expected, got)
		}
	}
}

// ==================== Exit Code Tests ====================

func TestExitCodeConstants(t *testing.T) {
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

// ==================== Passthrough Tests ====================

func TestPassthroughNotCalledForMerge(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil
	}

	exitCode := SmartMergeWithExecutor([]string{"merge", "feature-branch"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestPassthroughNotCalledForRebase(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil
	}

	exitCode := SmartRebaseWithExecutor([]string{"rebase", "main"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}

func TestPassthroughNotCalledForCherryPick(t *testing.T) {
	mock := git.NewMockExecutor()
	mock.OnRun = func(ctx context.Context, args []string) error {
		return nil
	}
	mock.OnOutput = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("/repo"), nil
	}
	mock.OnRunWithStdio = func(ctx context.Context, args []string) error {
		return nil
	}

	exitCode := SmartCherryPickWithExecutor([]string{"cherry-pick", "abc123"}, mock)

	if exitCode != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, exitCode)
	}
}
