# 🚀 G2 - Smart Git Merge with Semantic Conflict Analysis

Ever stared at a Git merge conflict and thought "okay but *what actually broke?*" — G2 is here to help! It's a drop-in Git replacement that intercepts merge commands and gives you intelligent, semantic-level conflict analysis. Instead of cryptic conflict markers, G2 tells you exactly **what** conflicted and **why**.

## ✨ Features

- 🔄 **Drop-in Git replacement** — Use `g2` exactly like `git`, all commands pass through seamlessly
- 🧠 **Semantic conflict detection** — Identifies conflicts at the function, class, interface, and key level
- 🌍 **Multi-language support** — Works with Python, JavaScript, TypeScript, JSON, and YAML
- 🎨 **Smart whitespace handling** — Detects when changes are semantically identical but differ only in formatting
- 💅 **Beautiful terminal UI** — Styled output with Nerd Font icons and color-coded status
- ⚡ **Auto-merge where possible** — Automatically resolves identical additions and formatting-only changes
- 🛡️ **Safe by default** — Creates backup files and uses atomic writes to protect your code

## 📦 Installation

### Building from source

```bash
git clone https://github.com/simonkoeck/g2.git
cd g2
go build -o g2 .

# Install system-wide
sudo cp g2 /usr/local/bin/

# Or just add an alias to your shell config
echo 'alias g2="/path/to/g2"' >> ~/.bashrc
```

### Using Nix 🐧

If you're a Nix user, there's a flake ready for you:

```bash
# Build it
nix build

# Or jump into a dev shell
nix develop
```

### Requirements

- Go 1.21 or newer
- Git (obviously!)
- A terminal with Nerd Font support (optional, but makes things prettier)

## 🎮 Usage

Just use `g2` like you'd use `git` — it works the same way:

```bash
g2 status
g2 log --oneline -5
g2 commit -m "feat: add cool feature"
g2 push origin main
```

The magic happens when you merge:

```bash
g2 merge feature-branch
```

### Command-line options 🛠️

G2 adds a few handy flags on top of the standard git merge options:

| Flag | What it does |
|------|--------------|
| `--dry-run` | Preview changes without actually writing anything |
| `--verbose` / `-v` | Show detailed progress as G2 analyzes your conflicts |
| `--no-backup` | Skip creating `.orig` backup files (useful for CI) |

All the regular `git merge` flags work too — they just get passed through.

## 📺 Example Output

When conflicts pop up, G2 gives you a nice semantic breakdown:

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

No more digging through files trying to figure out what went wrong!

## 🗣️ Supported Languages

| Language | Extensions | What G2 understands |
|----------|------------|---------------------|
| 🐍 Python | `.py` | Functions, Classes |
| 🟨 JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx` | Functions, Classes, Arrow functions, Variables |
| 🔷 TypeScript | `.ts`, `.mts`, `.cts`, `.tsx` | Functions, Classes, Interfaces, Type aliases |
| 📋 JSON | `.json` | Top-level keys |
| 📄 YAML | `.yaml`, `.yml` | Top-level keys |

For everything else, G2 falls back to standard "Text Conflict" detection — you're never left hanging.

## 🔍 Conflict Types

G2 categorizes conflicts so you know exactly what you're dealing with:

| Type | What happened | Can auto-merge? |
|------|---------------|-----------------|
| `Modified` | Both branches changed the same thing differently | ❌ |
| `Modified (same)` | Both branches made identical changes | ✅ |
| `Formatted Change` | Same changes, just different whitespace | ✅ |
| `Added (identical)` | Both branches added the exact same code | ✅ |
| `Added (differs)` | Both branches added something with the same name, but different | ❌ |
| `Delete/Modify` | One branch deleted it, the other modified it | ❌ |
| `Modify/Delete` | One branch modified it, the other deleted it | ❌ |
| `Text Conflict` | Standard conflict in unsupported file types | ❌ |
| `Binary Conflict` | Binary file conflicts | ❌ |

## ⚙️ How It Works

### 1. Passthrough Mode 🚦

For any command that's not a merge, G2 simply hands off to Git using `syscall.Exec`. This means colors, interactivity, stdin/stdout — everything works exactly as you'd expect. G2 gets out of the way.

### 2. Smart Merge Mode 🧠

When you run `g2 merge`, here's what happens behind the scenes:

1. G2 runs `git merge` and captures the result
2. If conflicts occur, it grabs the base, local, and remote versions of each conflicting file
3. Files are parsed using [Tree-sitter](https://tree-sitter.github.io/) grammars — real AST parsing, not regex hacks
4. G2 extracts all the definitions (functions, classes, interfaces, keys, etc.)
5. It compares definitions across all three versions to figure out the semantic conflicts
6. Results are displayed in a beautiful, easy-to-read table
7. Where possible, conflicts are auto-merged and staged for you

### 3. Whitespace Normalization 🧹

G2 normalizes whitespace when comparing changes. So if you and your teammate both made the same fix but with different formatting (tabs vs spaces, trailing newlines, etc.), G2 recognizes they're semantically identical and auto-merges them. One less thing to argue about!

## 📁 Project Structure

```
g2/
├── main.go                        # Entry point & Git wrapper logic
├── pkg/
│   ├── semantic/
│   │   ├── analyzer.go            # Tree-sitter parsing & conflict analysis
│   │   ├── synthesize.go          # File synthesis & auto-merge logic
│   │   └── synthesize_test.go     # Comprehensive test suite
│   └── ui/
│       └── ui.go                  # Terminal UI components (lipgloss)
├── go.mod
├── go.sum
├── flake.nix                      # Nix flake for development
├── shell.nix                      # Nix shell config
└── README.md
```

## 🧪 Running Tests

```bash
go test ./pkg/semantic -v
go test ./pkg/ui -v
```

The test suite covers collision detection, atomic file writes, conflict marker insertion, byte replacement logic, and more.

## 📚 Dependencies

- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Beautiful terminal styling
- [go-tree-sitter](https://github.com/smacker/go-tree-sitter) — Tree-sitter bindings for Go

## 🤝 Contributing

Contributions are super welcome! Here are some ideas if you're looking for ways to help:

- 🦀 Add support for more languages (Go, Rust, Java, C++, etc.)
- 📊 Add `--json` output flag for CI/CD integration
- 🎨 Syntax highlighting in conflict details
- 🔧 Improve collision detection for deeply nested code structures
- 📖 More documentation and examples

## 📄 License

MIT — do whatever you want with it!

---

Made with ☕ and frustration from too many confusing merge conflicts.
