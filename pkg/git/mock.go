package git

import (
	"context"
	"errors"
	"sync"
)

// MockCall records a single call to the mock executor.
type MockCall struct {
	Method string
	Args   []string
}

// MockResponse configures the response for a mock call.
type MockResponse struct {
	Output []byte
	Error  error
}

// MockExecutor implements Executor for testing purposes.
// It records all calls and returns configurable responses.
type MockExecutor struct {
	mu        sync.Mutex
	calls     []MockCall
	responses map[string]MockResponse // keyed by first arg (e.g., "merge", "add")
	defaults  MockResponse            // default response if no match

	// Hooks for custom behavior
	OnRun         func(ctx context.Context, args []string) error
	OnOutput      func(ctx context.Context, args []string) ([]byte, error)
	OnRunWithStdio func(ctx context.Context, args []string) error
}

// NewMockExecutor creates a new MockExecutor with no default responses.
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		responses: make(map[string]MockResponse),
	}
}

// SetResponse configures the response for commands starting with the given arg.
func (m *MockExecutor) SetResponse(firstArg string, output []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[firstArg] = MockResponse{Output: output, Error: err}
}

// SetDefaultResponse sets the response when no specific match is found.
func (m *MockExecutor) SetDefaultResponse(output []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults = MockResponse{Output: output, Error: err}
}

// SetDefaultError sets a default error for all commands.
func (m *MockExecutor) SetDefaultError(err error) {
	m.SetDefaultResponse(nil, err)
}

// Calls returns a copy of all recorded calls.
func (m *MockExecutor) Calls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MockCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// Reset clears all recorded calls.
func (m *MockExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
}

// Run records the call and returns the configured response.
func (m *MockExecutor) Run(ctx context.Context, args ...string) error {
	m.recordCall("Run", args)

	if m.OnRun != nil {
		return m.OnRun(ctx, args)
	}

	resp := m.getResponse(args)
	return resp.Error
}

// Output records the call and returns the configured response.
func (m *MockExecutor) Output(ctx context.Context, args ...string) ([]byte, error) {
	m.recordCall("Output", args)

	if m.OnOutput != nil {
		return m.OnOutput(ctx, args)
	}

	resp := m.getResponse(args)
	return resp.Output, resp.Error
}

// RunWithStdio records the call and returns the configured response.
func (m *MockExecutor) RunWithStdio(ctx context.Context, args ...string) error {
	m.recordCall("RunWithStdio", args)

	if m.OnRunWithStdio != nil {
		return m.OnRunWithStdio(ctx, args)
	}

	resp := m.getResponse(args)
	return resp.Error
}

func (m *MockExecutor) recordCall(method string, args []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, MockCall{Method: method, Args: args})
}

func (m *MockExecutor) getResponse(args []string) MockResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(args) > 0 {
		if resp, ok := m.responses[args[0]]; ok {
			return resp
		}
	}
	return m.defaults
}

// Common error types for testing
var (
	ErrNotGitRepo  = errors.New("fatal: not a git repository")
	ErrConflict    = errors.New("CONFLICT (content): Merge conflict in file.go")
	ErrTimeout     = context.DeadlineExceeded
)
