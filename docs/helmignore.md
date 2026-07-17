# .helmignore

The **`.helmignore`** file tells Helm which files to exclude when packaging a chart. It lives in the chart's root directory alongside `Chart.yaml`.

## Syntax
- One pattern per line.
- Supports Unix shell globbing (e.g., `*.txt`).
- Relative paths are matched from the chart root.
- Negation using `!` is **not** supported (despite common `.gitignore` behavior).
- No `**` recursive wildcard.
- Trailing spaces are ignored.

## Common Patterns
```text
# Exclude the .helmignore itself
.helmignore

# Exclude VCS directories
.git
.gitignore

# Exclude generated files
*.bak
*.tmp

# Exclude documentation files
*.md

# Exclude local development directories
vendor/
node_modules/
```

## Example (full)
```text
# comment

.helmignore
.git
*.txt
mydir/
/*.txt
/foo.txt
a[b-d].txt
*/draft*
*/*/draft*
draft?
```

## Tips
- Keep the file at the chart root; Helm does **not** look elsewhere.
- Use comments (`#`) for clarity.
- Remember that patterns are evaluated with Go's `filepath.Match`, not the Unix `fnmatch` library.
- If you need more advanced exclusions, consider restructuring your chart to keep unwanted files out of the source tree.

For details on how Helm processes this file, see the official Helm documentation.
