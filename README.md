# G2 - Semantic Git Merge

A drop-in Git replacement that understands your code. G2 intercepts merge commands and provides intelligent, semantic-level conflict analysis with automatic resolution for common scenarios.

## Features

- **Drop-in Git replacement** - Use `g2` exactly like `git`, all commands pass through seamlessly
- **Move/rename detection** - Automatically detects when functions are renamed or moved between files
- **Semantic conflict analysis** - Identifies conflicts at the function, class, interface, and key level
- **Interactive TUI** - Resolve conflicts visually with a terminal UI
- **Smart auto-merge** - Automatically resolves identical changes, formatting differences, and delete-vs-rename conflicts
- **Multi-language support** - Python, JavaScript, TypeScript, JSON, and YAML

## Installation

### Building from source

```bash
git clone https://github.com/simonkoeck/g2.git
cd g2
go build -o g2 .

# Install system-wide
sudo cp g2 /usr/local/bin/
```

### Using Nix

```bash
nix build
# Or enter a dev shell
nix develop
```

### Requirements

- Go 1.21+
- Git
- GCC (for tree-sitter CGO bindings)

## Usage

Use `g2` exactly like `git`:

```bash
g2 status
g2 log --oneline -5
g2 commit -m "feat: add feature"
g2 push origin main
```

The magic happens on merge:

```bash
g2 merge feature-branch
```

### Command-line options

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview changes without writing |
| `--verbose` / `-v` | Show detailed analysis progress |
| `--no-backup` | Skip creating `.orig` backup files |

### Git Merge Driver (Automatic Integration)

Instead of using `g2 merge`, you can configure Git to automatically use g2 for specific file types. This way, regular `git merge` commands will use g2's semantic merging.

**Step 1: Configure the merge driver** (add to `~/.gitconfig`):

```ini
[merge "g2"]
    name = G2 semantic merge driver
    driver = g2 merge-driver %O %A %B %L %P
```

**Step 2: Enable for file types** (add to `.gitattributes` in your repo):

```
*.py merge=g2
*.js merge=g2
*.ts merge=g2
*.tsx merge=g2
*.jsx merge=g2
```

Now when you run `git merge feature-branch`, Git will automatically invoke g2 for Python/JS/TS files, giving you semantic conflict resolution without changing your workflow.

## Move Detection

G2's standout feature is detecting when code is renamed or moved. When one branch deletes a function and another branch renames it, G2 recognizes they're the same code and auto-merges:

**Scenario:**
- Base: `def calc_total(items): ...`
- Main branch: deletes `calc_total`
- Feature branch: renames to `calculate_order_total(items): ...`

**Result:** G2 detects this as "Delete vs Rename" and keeps the renamed version automatically.

### How it works

1. **Parse** - Tree-sitter extracts function/class definitions from base, local, and remote
2. **Match** - Compares function bodies using Jaccard similarity (ignoring names)
3. **Synthesize** - Generates merged output, auto-resolving where safe

Match types:
- **Exact Match** (100% body similarity) - Auto-merge
- **Fuzzy Match** (>75% similarity) - Auto-merge with high confidence
- **No Match** - Requires manual resolution

## Interactive TUI

When conflicts require manual resolution, G2 launches an interactive terminal UI:

```
╭─────────────────────────────────────────────────────────────╮
│                    Conflict Resolution                       │
╰─────────────────────────────────────────────────────────────╯

  Conflicts (3)

  > utils.py: validate_email (Modified)        [Needs Resolution]
    utils.py: process_order (Modified)         [Needs Resolution]
    utils.py: calculate_shipping (Modified)    [Needs Resolution]

  ↑↓ navigate • Enter view • 1-5 resolve • a apply • q quit
```

### TUI Controls

| Key | Action |
|-----|--------|
| `↑`/`k` | Move up |
| `↓`/`j` | Move down |
| `Enter` | View conflict details (3-panel diff) |
| `1` | Keep Local version |
| `2` | Keep Remote version |
| `3` | Keep Both versions |
| `4` | Keep Base version |
| `5`/`s` | Skip (edit manually later) |
| `a` | Apply all resolutions |
| `q` | Quit/abort |

### Detail View

Pressing Enter shows a 3-panel view:

```
╭─ BASE ──────────────────────────────────────────────────────╮
│ def validate_email(email):                                   │
│     return "@" in email and "." in email                     │
╰─────────────────────────────────────────────────────────────╯
╭─ LOCAL ─────────────────────────────────────────────────────╮
│ def validate_email(email):                                   │
│     import re                                                │
│     pattern = r'^[a-zA-Z0-9._%+-]+@...'                      │
│     return bool(re.match(pattern, email or ''))              │
╰─────────────────────────────────────────────────────────────╯
╭─ REMOTE ────────────────────────────────────────────────────╮
│ def validate_email(email):                                   │
│     if not email or not isinstance(email, str):              │
│         return False                                         │
│     parts = email.split('@')                                 │
│     ...                                                      │
╰─────────────────────────────────────────────────────────────╯
```

## Conflict Types

| Type | Description | Auto-merge? |
|------|-------------|-------------|
| `Modified` | Both branches changed differently | No |
| `Modified (same)` | Both made identical changes | Yes |
| `Formatted Change` | Same changes, different whitespace | Yes |
| `Added (identical)` | Both added the same code | Yes |
| `Added (differs)` | Both added different code with same name | No |
| `Delete/Rename` | One deleted, other renamed | Yes |
| `Delete/Modify` | One deleted, other modified | No |

## Supported Languages

| Language | Extensions | Extracted Definitions |
|----------|------------|----------------------|
| Python | `.py` | Functions, Classes |
| JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx` | Functions, Classes, Arrow functions |
| TypeScript | `.ts`, `.mts`, `.cts`, `.tsx` | Functions, Classes, Interfaces, Types |
| JSON | `.json` | Top-level keys |
| YAML | `.yaml`, `.yml` | Top-level keys |

Unsupported files fall back to standard text conflict detection.

## Example Session

```bash
$ cd my-project
$ g2 merge feature/refactor

Running git merge...
Merge conflicts detected!
Analyzing 1 conflicted file...

╭──────────┬─────────────────────────────────────┬────────────────╮
│ FILE     │ CONFLICT                            │ STATUS         │
├──────────┼─────────────────────────────────────┼────────────────┤
│ utils.py │ calc_total → calculate_order_total  │ Auto: Rename   │
│ utils.py │ fmt_price → format_currency         │ Auto: Rename   │
│ utils.py │ validate_email (Modified)           │ Manual         │
│ utils.py │ process_order (Modified)            │ Manual         │
╰──────────┴─────────────────────────────────────┴────────────────╯

2 conflicts auto-merged, 2 need resolution

[TUI launches for manual conflicts...]
```

## Project Structure

```
g2/
├── main.go                     # Entry point & Git wrapper
├── pkg/
│   ├── semantic/
│   │   ├── analyzer.go         # Tree-sitter parsing
│   │   ├── moves.go            # Move/rename detection
│   │   ├── synthesize.go       # File synthesis & auto-merge
│   │   └── *_test.go           # Test suites
│   ├── tui/
│   │   ├── model.go            # Bubbletea model
│   │   ├── views.go            # UI rendering
│   │   ├── resolver.go         # Conflict resolution logic
│   │   └── styles.go           # Lipgloss styles
│   └── ui/
│       └── ui.go               # Non-interactive output
├── test/
│   └── */setup.sh              # Test scenario generators
├── go.mod
└── flake.nix
```

## Running Tests

```bash
go test ./pkg/semantic -v
go test ./pkg/tui -v
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [go-tree-sitter](https://github.com/smacker/go-tree-sitter) - AST parsing

## Contributing

Ideas for contribution:
- Add more languages (Go, Rust, Java, C++)
- Improve fuzzy matching heuristics
- Add `--json` output for CI integration
- Syntax highlighting in conflict views

## License

MIT
