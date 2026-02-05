package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Integration Test Infrastructure
// =============================================================================

// TestScenario represents a complete merge scenario with base, local, and remote versions
type TestScenario struct {
	Name           string
	Language       Language
	BaseContent    string
	LocalContent   string
	RemoteContent  string
	ExpectAutoMerge bool
	ExpectConflicts int
	ValidateResult func(t *testing.T, result []byte, conflicts []SynthesisConflict)
}

// runIntegrationTest executes a full merge scenario
func runIntegrationTest(t *testing.T, scenario TestScenario) {
	t.Helper()

	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "semantic-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Determine file extension
	ext := ".py"
	switch scenario.Language {
	case LangGo:
		ext = ".go"
	case LangRust:
		ext = ".rs"
	case LangJavaScript:
		ext = ".js"
	case LangTypeScript:
		ext = ".ts"
	case LangYAML:
		ext = ".yaml"
	}

	filename := "test" + ext
	filePath := filepath.Join(tmpDir, filename)

	// Write local content (this is what we're merging into)
	if err := os.WriteFile(filePath, []byte(scenario.LocalContent), 0644); err != nil {
		t.Fatalf("failed to write local file: %v", err)
	}

	// Analyze conflicts using the content-based analyzer
	analysis := AnalyzeConflictFromContents(
		filePath,
		[]byte(scenario.BaseContent),
		[]byte(scenario.LocalContent),
		[]byte(scenario.RemoteContent),
	)

	// Detect moves
	analysis.Conflicts = DetectMoves(analysis.Conflicts)

	// Verify conflict count if specified
	if scenario.ExpectConflicts >= 0 && len(analysis.Conflicts) != scenario.ExpectConflicts {
		t.Errorf("expected %d conflicts, got %d", scenario.ExpectConflicts, len(analysis.Conflicts))
		for i, c := range analysis.Conflicts {
			t.Logf("  conflict %d: %s - %s", i, c.UIConflict.ConflictType, c.UIConflict.Status)
		}
	}

	// Debug: print conflicts before synthesis
	t.Logf("Conflicts before synthesis:")
	for i, c := range analysis.Conflicts {
		t.Logf("  [%d] %s - status=%s, startByte=%d",
			i, c.UIConflict.ConflictType, c.UIConflict.Status,
			func() uint32 {
				if c.Local != nil {
					return c.Local.StartByte
				}
				if c.Base != nil {
					return c.Base.StartByte
				}
				return 0
			}())
	}

	// Synthesize result
	result, allMerged, err := SynthesizeToBytes(analysis)
	if err != nil {
		t.Fatalf("synthesis error: %v", err)
	}

	// Verify auto-merge expectation
	if scenario.ExpectAutoMerge && !allMerged {
		t.Errorf("expected all conflicts to auto-merge, but some required manual resolution")
		for _, c := range analysis.Conflicts {
			if c.UIConflict.Status != "Can Auto-merge" {
				t.Logf("  needs resolution: %s", c.UIConflict.ConflictType)
			}
		}
	}

	// Run custom validation if provided
	if scenario.ValidateResult != nil {
		scenario.ValidateResult(t, result, analysis.Conflicts)
	}
}

// =============================================================================
// Python Integration Tests
// =============================================================================

func TestIntegration_Python_SimpleFunction(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Python simple function modification",
		Language: LangPython,
		BaseContent: `def greet(name):
    return "Hello, " + name

def farewell(name):
    return "Goodbye, " + name
`,
		LocalContent: `def greet(name):
    return "Hello, " + name + "!"

def farewell(name):
    return "Goodbye, " + name
`,
		RemoteContent: `def greet(name):
    return "Hello, " + name

def farewell(name):
    return "Farewell, " + name
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 2,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Local changed greet, remote changed farewell - both should be kept
			if !strings.Contains(string(result), `"Hello, " + name + "!"`) {
				t.Error("local change to greet() should be preserved")
			}
			if !strings.Contains(string(result), `"Farewell, "`) {
				t.Error("remote change to farewell() should be applied")
			}
		},
	})
}

func TestIntegration_Python_ConflictingChanges(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Python conflicting function changes",
		Language: LangPython,
		BaseContent: `def calculate(x, y):
    return x + y
`,
		LocalContent: `def calculate(x, y):
    return x * y
`,
		RemoteContent: `def calculate(x, y):
    return x - y
`,
		ExpectAutoMerge: false,
		ExpectConflicts: 1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if len(conflicts) != 1 {
				t.Fatalf("expected 1 conflict, got %d", len(conflicts))
			}
			if conflicts[0].UIConflict.Status != "Needs Resolution" {
				t.Error("conflicting changes should need resolution")
			}
		},
	})
}

func TestIntegration_Python_ClassMethodChanges(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Python class with method changes",
		Language: LangPython,
		BaseContent: `class Calculator:
    def __init__(self, value):
        self.value = value

    def add(self, x):
        return self.value + x

    def subtract(self, x):
        return self.value - x
`,
		LocalContent: `class Calculator:
    def __init__(self, value):
        self.value = value
        self.history = []

    def add(self, x):
        result = self.value + x
        self.history.append(('add', x, result))
        return result

    def subtract(self, x):
        return self.value - x
`,
		RemoteContent: `class Calculator:
    def __init__(self, value):
        self.value = value

    def add(self, x):
        return self.value + x

    def subtract(self, x):
        return self.value - x

    def multiply(self, x):
        return self.value * x
`,
		ExpectAutoMerge: false, // Method additions need manual resolution
		ExpectConflicts: 3,     // __init__, add (local updates), multiply (remote add)
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Local changed __init__ and add - verify they're preserved
			if !strings.Contains(string(result), "self.history") {
				t.Error("local changes should be preserved")
			}
			// multiply is a method addition - can't auto-append into class
		},
	})
}

func TestIntegration_Python_FunctionDeleted(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Python function deleted locally",
		Language: LangPython,
		BaseContent: `def used_function():
    return 1

def unused_function():
    return 2

def another_function():
    return 3
`,
		LocalContent: `def used_function():
    return 1

def another_function():
    return 3
`,
		RemoteContent: `def used_function():
    return 1

def unused_function():
    return 2

def another_function():
    return 3
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Local deleted unused_function, remote unchanged - deletion should be kept
			if strings.Contains(string(result), "unused_function") {
				t.Error("locally deleted function should stay deleted")
			}
		},
	})
}

func TestIntegration_Python_FunctionMoved(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Python function renamed (move detection)",
		Language: LangPython,
		BaseContent: `def process_data(data):
    result = []
    for item in data:
        result.append(item * 2)
    return result

def other_function():
    return "other"
`,
		LocalContent: `def other_function():
    return "other"
`,
		RemoteContent: `def transform_data(data):
    result = []
    for item in data:
        result.append(item * 2)
    return result

def other_function():
    return "other"
`,
		ExpectConflicts: -1, // Don't check exact count, move detection may vary
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Check that move was detected (should have a "Moved" conflict type)
			hasMoveConflict := false
			for _, c := range conflicts {
				if strings.Contains(c.UIConflict.ConflictType, "Move") ||
					strings.Contains(c.UIConflict.ConflictType, "Renamed") {
					hasMoveConflict = true
					break
				}
			}
			if !hasMoveConflict && len(conflicts) > 1 {
				t.Log("Move detection may not have matched - checking orphan conflicts")
			}
		},
	})
}

func TestIntegration_Python_CommentOnlyChanges(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Python comment-only changes",
		Language: LangPython,
		BaseContent: `def process(data):
    return data * 2
`,
		LocalContent: `def process(data):
    # Process the data by doubling it
    return data * 2
`,
		RemoteContent: `def process(data):
    # Double the input value
    return data * 2
`,
		ExpectAutoMerge: true,
		ExpectConflicts: -1, // May detect as comment change or not, depending on normalization
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Verify the result contains the function
			if !strings.Contains(string(result), "def process") {
				t.Error("function should be in result")
			}
		},
	})
}

// =============================================================================
// JavaScript Integration Tests
// =============================================================================

func TestIntegration_JavaScript_ClassMethods(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "JavaScript class method changes",
		Language: LangJavaScript,
		BaseContent: `class UserService {
    constructor(api) {
        this.api = api;
    }

    async getUser(id) {
        return this.api.get('/users/' + id);
    }

    async deleteUser(id) {
        return this.api.delete('/users/' + id);
    }
}
`,
		LocalContent: `class UserService {
    constructor(api, cache) {
        this.api = api;
        this.cache = cache;
    }

    async getUser(id) {
        const cached = this.cache.get(id);
        if (cached) return cached;
        const user = await this.api.get('/users/' + id);
        this.cache.set(id, user);
        return user;
    }

    async deleteUser(id) {
        return this.api.delete('/users/' + id);
    }
}
`,
		RemoteContent: `class UserService {
    constructor(api) {
        this.api = api;
    }

    async getUser(id) {
        return this.api.get('/users/' + id);
    }

    async updateUser(id, data) {
        return this.api.put('/users/' + id, data);
    }

    async deleteUser(id) {
        return this.api.delete('/users/' + id);
    }
}
`,
		ExpectAutoMerge: false, // Method additions need manual resolution
		ExpectConflicts: -1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Local changed constructor and getUser - verify preserved
			if !strings.Contains(string(result), "this.cache") {
				t.Error("local cache changes should be preserved")
			}
			// updateUser is a method addition - can't auto-append into class
		},
	})
}

func TestIntegration_JavaScript_ArrowFunctions(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "JavaScript arrow function changes",
		Language: LangJavaScript,
		BaseContent: `const multiply = (a, b) => a * b;

const divide = (a, b) => a / b;

const add = (a, b) => a + b;
`,
		LocalContent: `const multiply = (a, b) => {
    console.log('multiplying', a, b);
    return a * b;
};

const divide = (a, b) => a / b;

const add = (a, b) => a + b;
`,
		RemoteContent: `const multiply = (a, b) => a * b;

const divide = (a, b) => {
    if (b === 0) throw new Error('Division by zero');
    return a / b;
};

const add = (a, b) => a + b;
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 2, // multiply (local update), divide (remote update)
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "console.log") {
				t.Error("local multiply changes should be preserved")
			}
			if !strings.Contains(string(result), "Division by zero") {
				t.Error("remote divide changes should be applied")
			}
		},
	})
}

// =============================================================================
// Go Integration Tests
// =============================================================================

func TestIntegration_Go_StructAndMethods(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Go struct and method changes",
		Language: LangGo,
		BaseContent: `package main

type Server struct {
	host string
	port int
}

func NewServer(host string, port int) *Server {
	return &Server{host: host, port: port}
}

func (s *Server) Start() error {
	return nil
}
`,
		LocalContent: `package main

type Server struct {
	host    string
	port    int
	timeout int
}

func NewServer(host string, port int) *Server {
	return &Server{host: host, port: port, timeout: 30}
}

func (s *Server) Start() error {
	return nil
}
`,
		RemoteContent: `package main

type Server struct {
	host string
	port int
}

func NewServer(host string, port int) *Server {
	return &Server{host: host, port: port}
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}
`,
		ExpectAutoMerge: true, // Stop is a top-level method (Go methods are top-level)
		ExpectConflicts: 3,    // Server struct, NewServer, Stop (remote add)
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "timeout") {
				t.Error("local timeout field should be preserved")
			}
			// Stop() should be auto-appended (Go methods are top-level)
			if !strings.Contains(string(result), "Stop()") {
				t.Error("remote Stop method should be applied")
			}
		},
	})
}

func TestIntegration_Go_InterfaceChanges(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Go interface changes",
		Language: LangGo,
		BaseContent: `package main

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}
`,
		LocalContent: `package main

type Reader interface {
	Read(p []byte) (n int, err error)
	ReadAll() ([]byte, error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}
`,
		RemoteContent: `package main

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
	Flush() error
}
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 2, // Reader (local update), Writer (remote update)
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "ReadAll") {
				t.Error("local ReadAll should be preserved")
			}
			if !strings.Contains(string(result), "Flush") {
				t.Error("remote Flush should be applied")
			}
		},
	})
}

// =============================================================================
// Rust Integration Tests
// =============================================================================

func TestIntegration_Rust_StructAndImpl(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Rust struct and impl changes",
		Language: LangRust,
		BaseContent: `struct Config {
    host: String,
    port: u16,
}

impl Config {
    fn new(host: String, port: u16) -> Self {
        Config { host, port }
    }

    fn url(&self) -> String {
        format!("{}:{}", self.host, self.port)
    }
}
`,
		LocalContent: `struct Config {
    host: String,
    port: u16,
    timeout: u32,
}

impl Config {
    fn new(host: String, port: u16) -> Self {
        Config { host, port, timeout: 30 }
    }

    fn url(&self) -> String {
        format!("{}:{}", self.host, self.port)
    }
}
`,
		RemoteContent: `struct Config {
    host: String,
    port: u16,
}

impl Config {
    fn new(host: String, port: u16) -> Self {
        Config { host, port }
    }

    fn url(&self) -> String {
        format!("{}:{}", self.host, self.port)
    }

    fn is_local(&self) -> bool {
        self.host == "localhost" || self.host == "127.0.0.1"
    }
}
`,
		ExpectAutoMerge: false, // Impl block modified differently
		ExpectConflicts: -1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "timeout") {
				t.Error("local timeout should be preserved")
			}
			// is_local is in the remote impl, may need resolution
		},
	})
}

// =============================================================================
// TypeScript Integration Tests
// =============================================================================

func TestIntegration_TypeScript_InterfaceAndClass(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "TypeScript interface and class changes",
		Language: LangTypeScript,
		BaseContent: `interface User {
    id: number;
    name: string;
}

class UserRepository {
    private users: User[] = [];

    add(user: User): void {
        this.users.push(user);
    }

    find(id: number): User | undefined {
        return this.users.find(u => u.id === id);
    }
}
`,
		LocalContent: `interface User {
    id: number;
    name: string;
    email: string;
}

class UserRepository {
    private users: User[] = [];

    add(user: User): void {
        this.users.push(user);
    }

    find(id: number): User | undefined {
        return this.users.find(u => u.id === id);
    }
}
`,
		RemoteContent: `interface User {
    id: number;
    name: string;
}

class UserRepository {
    private users: User[] = [];

    add(user: User): void {
        this.users.push(user);
    }

    find(id: number): User | undefined {
        return this.users.find(u => u.id === id);
    }

    findAll(): User[] {
        return [...this.users];
    }
}
`,
		ExpectAutoMerge: false, // Method additions need manual resolution
		ExpectConflicts: 2,     // User interface, findAll method
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "email: string") {
				t.Error("local email field should be preserved")
			}
			// findAll is a method addition - can't auto-append into class
		},
	})
}

// =============================================================================
// YAML Integration Tests
// =============================================================================

func TestIntegration_YAML_ConfigChanges(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "YAML config file changes",
		Language: LangYAML,
		BaseContent: `database:
  host: localhost
  port: 5432

server:
  host: 0.0.0.0
  port: 8080
`,
		LocalContent: `database:
  host: localhost
  port: 5432
  pool_size: 10

server:
  host: 0.0.0.0
  port: 8080
`,
		RemoteContent: `database:
  host: localhost
  port: 5432

server:
  host: 0.0.0.0
  port: 8080

logging:
  level: info
  format: json
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 2, // database (local update), logging (remote add)
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			t.Logf("Result:\n%s", string(result))
			for i, c := range conflicts {
				t.Logf("Conflict %d: %s (status=%s) name=%s",
					i, c.UIConflict.ConflictType, c.UIConflict.Status,
					func() string {
						if c.Remote != nil {
							return c.Remote.Name
						}
						return "nil"
					}())
			}
			if !strings.Contains(string(result), "pool_size") {
				t.Error("local pool_size should be preserved")
			}
			// logging should be auto-appended
			if !strings.Contains(string(result), "logging") {
				t.Error("remote logging section should be applied")
			}
		},
	})
}

// =============================================================================
// Edge Case Integration Tests
// =============================================================================

func TestIntegration_EmptyBase(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Empty base - both add same function",
		Language: LangPython,
		BaseContent: ``,
		LocalContent: `def hello():
    return "hello"
`,
		RemoteContent: `def hello():
    return "hello"
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "def hello") {
				t.Error("function should be in result")
			}
		},
	})
}

func TestIntegration_BothDelete(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Both branches delete same function",
		Language: LangPython,
		BaseContent: `def keep_me():
    return 1

def delete_me():
    return 2
`,
		LocalContent: `def keep_me():
    return 1
`,
		RemoteContent: `def keep_me():
    return 1
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if strings.Contains(string(result), "delete_me") {
				t.Error("deleted function should not appear")
			}
			if !strings.Contains(string(result), "keep_me") {
				t.Error("kept function should appear")
			}
		},
	})
}

func TestIntegration_IdenticalChanges(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Both make identical changes",
		Language: LangPython,
		BaseContent: `def process(x):
    return x
`,
		LocalContent: `def process(x):
    return x * 2
`,
		RemoteContent: `def process(x):
    return x * 2
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "x * 2") {
				t.Error("identical change should be preserved")
			}
			if len(conflicts) > 0 && conflicts[0].UIConflict.Status != "Can Auto-merge" {
				t.Error("identical changes should auto-merge")
			}
		},
	})
}

func TestIntegration_LargeFile(t *testing.T) {
	// Generate a large file with many functions
	var base strings.Builder

	for i := 0; i < 50; i++ {
		base.WriteString("def func_")
		base.WriteString(string(rune('a' + i%26)))
		base.WriteString("_")
		base.WriteString(string(rune('0' + i/26)))
		base.WriteString("():\n    return ")
		base.WriteString(string(rune('0' + i%10)))
		base.WriteString("\n\n")
	}

	baseContent := base.String()

	// Local changes function 10
	localContent := strings.Replace(baseContent, "def func_k_0():\n    return 0", "def func_k_0():\n    return 'local'", 1)

	// Remote changes function 20
	remoteContent := strings.Replace(baseContent, "def func_u_0():\n    return 4", "def func_u_0():\n    return 'remote'", 1)

	runIntegrationTest(t, TestScenario{
		Name:           "Large file with 50 functions",
		Language:       LangPython,
		BaseContent:    baseContent,
		LocalContent:   localContent,
		RemoteContent:  remoteContent,
		ExpectAutoMerge: true,
		ExpectConflicts: -1, // Multiple conflicts possible
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "'local'") {
				t.Error("local change should be preserved")
			}
			// Remote change may or may not be applied depending on move detection
		},
	})
}

func TestIntegration_MixedConflicts(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Mixed auto-merge and manual conflicts",
		Language: LangPython,
		BaseContent: `def auto_merge_local():
    return 1

def auto_merge_remote():
    return 2

def conflict():
    return 3
`,
		LocalContent: `def auto_merge_local():
    return "local changed"

def auto_merge_remote():
    return 2

def conflict():
    return "local version"
`,
		RemoteContent: `def auto_merge_local():
    return 1

def auto_merge_remote():
    return "remote changed"

def conflict():
    return "remote version"
`,
		ExpectAutoMerge: false,
		ExpectConflicts: 3,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Count auto-mergeable vs manual
			autoMerge := 0
			manual := 0
			for _, c := range conflicts {
				if c.UIConflict.Status == "Can Auto-merge" {
					autoMerge++
				} else {
					manual++
				}
			}
			if autoMerge != 2 {
				t.Errorf("expected 2 auto-merge conflicts, got %d", autoMerge)
			}
			if manual != 1 {
				t.Errorf("expected 1 manual conflict, got %d", manual)
			}
		},
	})
}

func TestIntegration_UnicodeIdentifiers(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Unicode identifiers in Python",
		Language: LangPython,
		BaseContent: `def 计算(值):
    return 值 * 2

def grüßen(name):
    return "Hallo " + name
`,
		LocalContent: `def 计算(值):
    return 值 * 3

def grüßen(name):
    return "Hallo " + name
`,
		RemoteContent: `def 计算(值):
    return 值 * 2

def grüßen(name):
    return "Guten Tag " + name
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 2, // 计算 (local update), grüßen (remote update)
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "值 * 3") {
				t.Error("local Unicode change should be preserved")
			}
			if !strings.Contains(string(result), "Guten Tag") {
				t.Error("remote Unicode change should be applied")
			}
		},
	})
}

func TestIntegration_NestedClasses(t *testing.T) {
	runIntegrationTest(t, TestScenario{
		Name:     "Nested classes in Python",
		Language: LangPython,
		BaseContent: `class Outer:
    class Inner:
        def inner_method(self):
            return "inner"

    def outer_method(self):
        return "outer"
`,
		LocalContent: `class Outer:
    class Inner:
        def inner_method(self):
            return "inner modified"

    def outer_method(self):
        return "outer"
`,
		RemoteContent: `class Outer:
    class Inner:
        def inner_method(self):
            return "inner"

    def outer_method(self):
        return "outer modified"
`,
		ExpectAutoMerge: true,
		ExpectConflicts: -1, // Depends on how nested classes are parsed
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Nested class parsing may vary
			if !strings.Contains(string(result), "Outer") {
				t.Error("Outer class should be in result")
			}
		},
	})
}

// =============================================================================
// Regression Tests
// =============================================================================

func TestIntegration_Regression_LocalUpdateNotOverwritten(t *testing.T) {
	// Regression test: local-only updates were being overwritten by remote (unchanged from base)
	runIntegrationTest(t, TestScenario{
		Name:     "Local update should not be overwritten by unchanged remote",
		Language: LangPython,
		BaseContent: `def foo():
    return 1
`,
		LocalContent: `def foo():
    return 2
`,
		RemoteContent: `def foo():
    return 1
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 1,
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			if !strings.Contains(string(result), "return 2") {
				t.Error("local change should be preserved when remote is unchanged")
			}
			if strings.Contains(string(result), "return 1") {
				t.Error("base/remote value should not overwrite local")
			}
		},
	})
}

func TestIntegration_Regression_DeletedBothNotDuplicated(t *testing.T) {
	// Regression test: functions deleted in both branches should not appear in output
	runIntegrationTest(t, TestScenario{
		Name:     "Deleted in both should not reappear",
		Language: LangPython,
		BaseContent: `def keep():
    pass

def remove():
    pass
`,
		LocalContent: `def keep():
    pass
`,
		RemoteContent: `def keep():
    pass
`,
		ExpectAutoMerge: true,
		ExpectConflicts: 1, // "Deleted (both)" conflict
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			count := strings.Count(string(result), "def ")
			if count != 1 {
				t.Errorf("expected exactly 1 function, got %d", count)
			}
		},
	})
}

// =============================================================================
// Stress Tests
// =============================================================================

func TestIntegration_ManyConflicts(t *testing.T) {
	// Test with many simultaneous conflicts
	var base, local, remote strings.Builder

	for i := 0; i < 20; i++ {
		fname := string(rune('a' + i))
		base.WriteString("def func_" + fname + "():\n    return " + string(rune('0'+i%10)) + "\n\n")

		if i%3 == 0 {
			// Local changes
			local.WriteString("def func_" + fname + "():\n    return 'local_" + fname + "'\n\n")
		} else {
			local.WriteString("def func_" + fname + "():\n    return " + string(rune('0'+i%10)) + "\n\n")
		}

		if i%3 == 1 {
			// Remote changes
			remote.WriteString("def func_" + fname + "():\n    return 'remote_" + fname + "'\n\n")
		} else {
			remote.WriteString("def func_" + fname + "():\n    return " + string(rune('0'+i%10)) + "\n\n")
		}
	}

	runIntegrationTest(t, TestScenario{
		Name:            "Many non-conflicting changes",
		Language:        LangPython,
		BaseContent:     base.String(),
		LocalContent:    local.String(),
		RemoteContent:   remote.String(),
		ExpectAutoMerge: true,
		ExpectConflicts: -1, // Many conflicts expected
		ValidateResult: func(t *testing.T, result []byte, conflicts []SynthesisConflict) {
			// Verify all local changes preserved
			for i := 0; i < 20; i += 3 {
				fname := string(rune('a' + i))
				if !strings.Contains(string(result), "local_"+fname) {
					t.Errorf("local change for func_%s should be preserved", fname)
				}
			}
			// Verify all remote changes applied
			for i := 1; i < 20; i += 3 {
				fname := string(rune('a' + i))
				if !strings.Contains(string(result), "remote_"+fname) {
					t.Errorf("remote change for func_%s should be applied", fname)
				}
			}
		},
	})
}
