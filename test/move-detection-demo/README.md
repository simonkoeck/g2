# Move Detection Demo

This demo creates a realistic merge conflict scenario that demonstrates g2's move detection feature.

## Scenario

The demo creates a git repository with:

1. **Initial state**: `utils.py` with 5 functions, `helpers.js` with 4 functions

2. **Feature branch** (`feature/reorganize`):
   - Moves `validate_email` and `parse_date` from `utils.py` → `validators.py`
   - Moves `deepClone` and `generateUUID` from `helpers.js` → `core.js`
   - Updates imports in original files

3. **Main branch** (diverged):
   - Improves `validate_email` with TLD validation
   - Improves `parse_date` with leap year support
   - Improves `deepClone` with circular reference handling
   - Improves `generateUUID` to use crypto API
   - Adds thousand separators to `format_currency`

When merging, git sees:
- Functions deleted on feature branch
- Functions modified on main branch
- New files with "new" functions on feature branch

**Without move detection**: These appear as delete/modify conflicts requiring manual resolution.

**With move detection**: g2 recognizes that the "deleted" functions match the "added" functions in new files, allowing automatic merge.

## Usage

```bash
# Run the setup script
./setup.sh

# Navigate to the demo repo
cd repo

# Run g2 merge to see move detection in action
g2 merge

# Or from the g2 project root:
go run . merge
```

## Expected Output

g2 should detect:
- `validate_email` moved from `utils.py` to `validators.py`
- `parse_date` moved from `utils.py` to `validators.py`
- `deepClone` moved from `helpers.js` to `core.js`
- `generateUUID` moved from `helpers.js` to `core.js`

And automatically merge the changes since the function bodies match (or nearly match with fuzzy detection).

## Cleanup

```bash
rm -rf repo
```
