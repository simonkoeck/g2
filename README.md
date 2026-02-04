# G2 - Smart Git Merge with Semantic Conflict Analysis

A drop-in Git replacement that intercepts `merge` commands to provide intelligent, semantic-level conflict analysis. Instead of just telling you "there's a conflict", G2 tells you *what* conflicted and *why*.

## Features

-  **Drop-in Git replacement** - Use `g2` exactly like `git`, all commands pass through seamlessly
-  **Semantic conflict detection** - Identifies conflicts at the function, class, interface, and key level
-  **Multi-language support** - Python, JavaScript, TypeScript, JSON, and YAML
-  **Smart whitespace handling** - Detects when changes are semantically identical but differ only in formatting
-  **Beautiful terminal UI** - Styled output with Nerd Font icons and color-coded status

## Installation

```bash
# Build from source
git clone https://github.com/simonkoeck/g2.git
cd g2
go build -o g2 .

# Install system-wide
sudo cp g2 /usr/local/bin/

# Or add an alias to your shell config
echo 'alias g2="/path/to/g2"' >> ~/.bashrc
```

### Requirements

- Go 1.21+
- Git
- A terminal with Nerd Font support (optional, for icons)

## Usage

Use `g2` exactly like `git`:

```bash
g2 status
g2 log --oneline -5
g2 commit -m "message"
g2 push origin main
```

The magic happens when you merge:

```bash
g2 merge feature-branch
```

## Example Output

When conflicts occur, G2 provides semantic analysis:

```
╭─────────────────╮
│  G2 Smart Merge │
╰─────────────────╯

 Running git merge...
 Merge conflicts detected!
 Analyzing conflicts...

╭──────────────┬─────────────────────────────┬──────────────────╮
│  FILE        │ CONFLICT TYPE               │ STATUS           │
├──────────────┼─────────────────────────────┼──────────────────┤
│ app.py       │ Function 'process' Modified │ Needs Resolution │
│ utils.ts     │ Interface 'User' Modified   │ Needs Resolution │
│ config.json  │ Key 'version' Modified      │ Needs Resolution │
│ helpers.js   │ Function 'format' Modified (same) │ Can Auto-merge │
╰──────────────┴─────────────────────────────┴──────────────────╯

 3 of 4 conflicts need manual resolution
```

## Supported Languages

| Language   | Extensions                     | Detected Elements                              |
|------------|--------------------------------|------------------------------------------------|
| Python     | `.py`                          | Functions, Classes                             |
| JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx`  | Functions, Classes, Arrow functions, Variables |
| TypeScript | `.ts`, `.mts`, `.cts`, `.tsx`  | Functions, Classes, Interfaces, Type aliases   |
| JSON       | `.json`                        | Top-level keys                                 |
| YAML       | `.yaml`, `.yml`                | Top-level keys                                 |

Other file types fall back to standard "Text Conflict" detection.

## Conflict Types

G2 detects several types of semantic conflicts:

| Conflict Type | Description |
|---------------|-------------|
| `Modified` | Both branches changed the same definition differently |
| `Modified (same)` | Both branches made identical changes (can auto-merge) |
| `Added (differs)` | Both branches added the same definition with different implementations |
| `Added (identical)` | Both branches added identical definitions (can auto-merge) |
| `Delete/Modify` | One branch deleted, the other modified |
| `Modify/Delete` | One branch modified, the other deleted |
| `Text Conflict` | Non-semantic conflict in unsupported file types |
| `Binary Conflict` | Conflict in binary files |

## How It Works

1. **Passthrough Mode**: For all non-merge commands, G2 uses `syscall.Exec` to replace itself with Git entirely, preserving colors, interactivity, and stdin/stdout.

2. **Smart Merge Mode**: When you run `g2 merge`:
   - Executes `git merge` and captures the result
   - If conflicts occur, retrieves the base, local, and remote versions of each file
   - Parses files using [Tree-sitter](https://tree-sitter.github.io/) grammars
   - Extracts definitions (functions, classes, interfaces, keys)
   - Compares definitions across versions to identify semantic conflicts
   - Displays results in a beautiful table

3. **Whitespace Normalization**: When comparing changes, G2 normalizes whitespace so that formatting-only differences (tabs vs spaces, extra newlines) are detected as "Can Auto-merge" rather than conflicts.

## Project Structure

```
g2/
├── main.go                    # Entry point, Git wrapper logic
├── pkg/
│   ├── ui/
│   │   └── ui.go              # Terminal UI components (lipgloss)
│   └── semantic/
│       └── analyzer.go        # Tree-sitter parsing & conflict analysis
├── go.mod
└── README.md
```

## Dependencies

- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [go-tree-sitter](https://github.com/smacker/go-tree-sitter) - Tree-sitter bindings for Go

## License

MIT

## Contributing

Contributions are welcome! Some ideas for improvements:

- Add support for more languages (Go, Rust, Java, etc.)
- Implement actual auto-merge for "Can Auto-merge" conflicts
- Add `--json` output flag for CI integration
- Syntax highlighting in conflict details
