# Gote Code Minimization Plan

**Goal:** Reduce codebase while maintaining all functionality
**Current:** ~6,000 lines across 35 Go files
**Target savings:** 400-600 lines (7-10%)

---

## High Priority (Low effort, High impact)

### 1. Extract Case-Insensitive Index Lookup (~50 lines saved)

**Problem:** Same 10-line pattern repeated 5 times in `core/notes.go`.

**Current (repeated 5x):**
```go
actualKey := title
meta, exists := index[title]
if !exists {
    for key := range index {
        if strings.EqualFold(key, title) {
            actualKey = key
            meta = index[key]
            exists = true
            break
        }
    }
}
```

**Solution:** Add to `data/index.go`:
```go
func LookupNote(index map[string]NoteMeta, name string) (actualName string, meta NoteMeta, found bool)
```

**Locations to update:**
- `core/notes.go`: CreateOrOpenNote, UpdateLastVisited, GetNoteInfo, RenameNote
- `core/pins.go`: ListPinnedNotes

---

### 2. Eliminate Pass-Through Functions (~40 lines saved)

**Problem:** Core layer functions that just forward to data layer with no logic.

**Delete these from `core/`:**
```go
// core/trash.go - delete entire file (36 lines)
func ListTrashedNotes() { return data.ListTrashedNotes() }
func EmptyTrash() { return data.EmptyTrash() }

// core/tags.go - delete ListTags (keep GetPopularTags)
func ListTags() { return data.LoadTags() }

// core/search.go - delete SearchTrash
func SearchTrash(query string) { return data.SearchTrash(query) }
```

**Update CLI to call `data.*` directly.**

---

### 3. Consolidate Tag Search Functions (~25 lines saved)

**Problem:** `SearchNotesByTags` (OR) and `FilterNotesByTags` (AND) are 80% identical.

**File:** `core/search.go`

**Merge into:**
```go
func SearchNotesByTags(tags []string, matchAll bool, limit int) ([]SearchResult, error)
```

---

### 4. Remove Not-Implemented Commands (~15 lines saved)

**Problem:** Dead code in `main.go`:
```go
case "popular", "pop": cli.NotImplementedCommand("popular")
case "today": cli.NotImplementedCommand("today")
case "journal", "j": cli.NotImplementedCommand("journal")
case "transfer": cli.NotImplementedCommand("transfer")
case "calendar", "cal": cli.NotImplementedCommand("calendar")
case "lint", "l": cli.NotImplementedCommand("lint")
```

**Action:** Delete until implemented.

---

## Medium Priority (Medium effort)

### 5. Simplify Main.go Command Dispatch (~40 lines saved)

**Problem:** 20 repetitive cases for mode variants:
```go
case "recent", "r": cli.RecentCommand(rest, false, false, false, false)
case "ro": cli.RecentCommand(rest, true, false, false, false)
case "rd": cli.RecentCommand(rest, false, true, false, false)
// ... repeats for search, tag, pinned
```

**Solution:** Parse suffix dynamically:
```go
func dispatchWithMode(cmd string, rest []string) bool {
    base, mode := parseMode(cmd) // "ro" -> "r", "open"
    switch base {
    case "r", "recent":
        cli.RecentCommand(rest, mode)
        return true
    // ...
    }
    return false
}
```

---

### 6. Externalize CSS Template (~140 lines moved)

**Problem:** 140 lines of CSS embedded in `view.go` as Go string.

**Options:**
1. Move to `~/.gote/view.css` (user customizable)
2. Use `//go:embed` directive
3. Keep but minify (remove whitespace/comments)

**Trade-off:** Adds complexity. Lower priority.

---

## Summary Table

| # | Change | Lines Saved | Effort | Risk |
|---|--------|-------------|--------|------|
| 1 | Case-insensitive lookup | 50 | Low | Low |
| 2 | Remove pass-throughs | 40 | Low | Low |
| 3 | Merge tag search | 25 | Medium | Low |
| 4 | Remove not-implemented | 15 | Low | None |
| 5 | Command dispatch | 40 | Medium | Medium |
| 6 | Externalize CSS | 140 | Medium | Medium |

**Conservative total:** ~170 lines
**With CSS extraction:** ~310 lines

---

## Not Recommended

- **Generic JSON helpers:** Type safety > brevity
- **Merge data/core layers:** Breaks separation of concerns
- **Reduce tests:** Coverage is valuable
- **Over-abstract CLI commands:** Would obscure flow

---

## Implementation Order

1. Start with #1 (case-insensitive lookup) - highest value, lowest risk
2. Do #2 (pass-throughs) and #4 (not-implemented) together - quick wins
3. #3 (tag search) if clean abstraction emerges
4. #5 (command dispatch) requires careful testing
5. #6 (CSS) only if maintenance becomes issue
