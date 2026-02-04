// Package git provides an abstraction layer for executing git commands
// with support for timeouts, context cancellation, and testing.
package git

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// DefaultTimeout is the default timeout for git operations.
const DefaultTimeout = 30 * time.Second

// Executor defines the interface for running git commands.
// This abstraction allows for testing and timeout support.
type Executor interface {
	// Run executes a git command and discards stdout/stderr.
	// Returns error if the command fails.
	Run(ctx context.Context, args ...string) error

	// Output executes a git command and returns stdout.
	// Returns the output and any error.
	Output(ctx context.Context, args ...string) ([]byte, error)

	// RunWithStdio executes a git command with stdin/stdout/stderr
	// connected to the current process (for interactive commands).
	RunWithStdio(ctx context.Context, args ...string) error
}

// DefaultExecutor implements Executor using exec.CommandContext.
type DefaultExecutor struct {
	Timeout time.Duration
}

// NewDefaultExecutor creates a new DefaultExecutor with the default timeout.
func NewDefaultExecutor() *DefaultExecutor {
	return &DefaultExecutor{Timeout: DefaultTimeout}
}

// NewExecutorWithTimeout creates a new DefaultExecutor with a custom timeout.
func NewExecutorWithTimeout(timeout time.Duration) *DefaultExecutor {
	return &DefaultExecutor{Timeout: timeout}
}

// Run executes a git command and discards stdout/stderr.
func (e *DefaultExecutor) Run(ctx context.Context, args ...string) error {
	ctx, cancel := e.contextWithTimeout(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// Output executes a git command and returns stdout.
func (e *DefaultExecutor) Output(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := e.contextWithTimeout(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	return cmd.Output()
}

// RunWithStdio executes a git command with stdin/stdout/stderr connected.
func (e *DefaultExecutor) RunWithStdio(ctx context.Context, args ...string) error {
	ctx, cancel := e.contextWithTimeout(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// contextWithTimeout returns a context with the executor's timeout applied.
// If the provided context already has a deadline, it is used if shorter.
func (e *DefaultExecutor) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if e.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, e.Timeout)
}

// IsTimeoutError checks if an error is due to context deadline exceeded.
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if err == context.DeadlineExceeded {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ProcessState != nil && exitErr.ProcessState.ExitCode() == -1
	}
	return false
}
