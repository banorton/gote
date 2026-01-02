# Gote Refactoring Plan

Multi-phase implementation plan for code quality improvements identified in CODE_ANALYSIS.md.

---

## Phase 1: Quick Win (15 min)

### 1.1 Fix LoadIndex() Inside Loop

**File:** `src/cli/interactive.go:206-215`

**Problem:** `LoadIndex()` is called inside the pagination loop, causing repeated disk I/O on every selection.

**Steps:**
1. Find the loop in `interactive.go` where `LoadIndex()` is called
2. Move the `LoadIndex()` call outside/before the loop
3. Pass the index as a parameter or use the pre-loaded value inside the loop
4. Run tests to verify behavior unchanged

**Verification:** `go test ./...`

---

## Phase 2: Helper Functions (2-4 hours)

### 2.1 Create `LoadConfigAndUI()` Helper

**New File:** `src/cli/util.go` (or add to existing utils)

**Signature:**
```go
func LoadConfigAndUI() (data.Config, *UI, error)
```

**Steps:**
1. Create the helper function that:
   - Calls `data.LoadConfig()`
   - Creates `NewUI(cfg.FancyUI)`
   - Returns both plus any error
2. Update all 21+ occurrences across these files:
   - `src/cli/commands.go` (5 occurrences)
   - `src/cli/pins.go` (4 occurrences)
   - `src/cli/interactive.go` (6 occurrences)
   - `src/cli/trash.go` (3 occurrences)
   - `src/cli/info.go` (2 occurrences)
   - `src/cli/templates.go` (1 occurrence)
3. Run tests after each file modification

**Pattern to replace:**
```go
// Before (5 lines)
cfg, err := data.LoadConfig()
if err != nil {
    fmt.Println("Error loading config:", err)
    return
}
ui := NewUI(cfg.FancyUI)

// After (4 lines)
cfg, ui, err := LoadConfigAndUI()
if err != nil {
    return
}
```

---

### 2.2 Create `SaveIndexWithTags()` Atomic Function

**File:** `src/data/index.go`

**Signature:**
```go
func SaveIndexWithTags(index Index) error
```

**Steps:**
1. Add new function to `data/index.go`:
   ```go
   func SaveIndexWithTags(index Index) error {
       if err := SaveIndex(index); err != nil {
           return err
       }
       return UpdateTagsIndex(index)
   }
   ```
2. Find all occurrences of the dual-call pattern (8+ locations):
   - `src/core/notes.go`
   - `src/data/trash.go`
   - `src/cli/commands.go`
   - Others via grep
3. Replace each occurrence with single call
4. Run tests after each file

**Pattern to replace:**
```go
// Before
if err := data.SaveIndex(index); err != nil { return err }
return data.UpdateTagsIndex(index)

// After
return data.SaveIndexWithTags(index)
```

---

### 2.3 Create Generic `FormatJSONFile()` Function

**File:** `src/data/format.go` (new file)

**Signature:**
```go
func FormatJSONFile(path string) error
```

**Steps:**
1. Create `src/data/format.go` with generic implementation:
   ```go
   func FormatJSONFile(path string) error {
       content, err := os.ReadFile(path)
       if err != nil {
           return err
       }
       var buf bytes.Buffer
       if err := json.Indent(&buf, content, "", "  "); err != nil {
           return err
       }
       return os.WriteFile(path, buf.Bytes(), 0644)
   }
   ```
2. Replace these 4 identical functions:
   - `FormatIndexFile()` in `data/index.go:181-198`
   - `FormatTagsFile()` in `data/tags.go:52-69`
   - `FormatPinsFile()` in `data/pins.go:41-58`
   - `FormatConfigFile()` in `data/config.go:102-119`
3. Update callers to use new generic function
4. Delete old functions
5. Run tests

---

## Phase 3: Consolidation (4-6 hours)

### 3.1 Merge Tag Parsers

**Problem:** Two different tag parsing implementations exist.

**Files:**
- `src/data/index.go` - Tag extraction from note content
- `src/cli/args.go` - Tag parsing from CLI arguments

**Steps:**
1. Analyze both implementations to understand differences
2. Create unified `ParseTags(input string) []string` in `src/data/tags.go`
3. Handle both use cases (content extraction vs CLI args)
4. Update callers in both files
5. Remove duplicate implementations
6. Add unit tests for edge cases

---

### 3.2 Mode Handler Abstraction

**Problem:** Open/delete/pin/view mode handling is repeated 5x with slight variations.

**Steps:**
1. Identify the common pattern across:
   - `SearchCommand` view mode
   - `RecentCommand` view mode
   - `TagCommand` view mode
   - `SelectCommand` modes
   - Any other interactive commands
2. Design a `ModeHandler` interface or callback pattern:
   ```go
   type NoteAction func(noteName string, meta data.NoteMeta) error

   func HandleNoteSelection(notes []data.NoteMeta, action NoteAction) error
   ```
3. Implement handlers for each mode: open, delete, pin, view
4. Refactor commands to use the abstraction
5. Run tests after each command refactor

---

## Phase 4: Large Refactors (20+ hours)

### 4.1 Break Up Monster Functions

#### NoteCommand (84 lines, 6 responsibilities)
**File:** `src/cli/commands.go`

Split into:
- `parseNoteArgs()` - Argument parsing
- `handleQuickNote()` - Quick note logic
- `handleTemplateNote()` - Template creation
- `handleExistingNote()` - Open existing
- `handleNewNote()` - Create new
- `NoteCommand()` - Dispatcher only

#### SearchCommand (130 lines, 8 responsibilities)
**File:** `src/cli/interactive.go`

Split into:
- `parseSearchArgs()` - Argument parsing
- `buildSearchQuery()` - Query construction
- `executeSearch()` - Run search
- `displaySearchResults()` - Output formatting
- `handleSearchSelection()` - Interactive mode
- `SearchCommand()` - Dispatcher only

#### SelectCommand (240 lines, 5 responsibilities)
**File:** `src/cli/interactive.go`

Split into:
- `buildNoteList()` - Get notes to display
- `displayNoteList()` - Pagination/display
- `handleSelection()` - User input handling
- `executeAction()` - Perform selected action
- `SelectCommand()` - Dispatcher only

---

### 4.2 Add Custom Error Types

**New File:** `src/data/errors.go`

```go
package data

import "errors"

var (
    ErrNoteNotFound     = errors.New("note not found")
    ErrNoteExists       = errors.New("note already exists")
    ErrTemplateNotFound = errors.New("template not found")
    ErrInvalidNoteName  = errors.New("invalid note name")
    ErrConfigNotFound   = errors.New("config not found")
)
```

**Steps:**
1. Create error types file
2. Update `core/` functions to return typed errors
3. Update `cli/` to check error types for user-friendly messages
4. Update tests to check for specific error types

---

### 4.3 Fix Layer Violations

#### Move Date Parsing to CLI Layer
- Date parsing for `-d`, `-dt` flags belongs in `cli/`, not `core/`
- `core/` should receive parsed `time.Time` values

#### Move BuildNoteMeta to Core Layer
- `data.BuildNoteMeta()` contains business logic (tag extraction, preview generation)
- Should be `core.BuildNoteMeta()` calling `data/` for file I/O only

**Steps:**
1. Create `src/core/metadata.go`
2. Move `BuildNoteMeta` logic there
3. Keep only file reading in `data/`
4. Update all callers
5. Run tests

---

## Verification Checklist

After each phase:
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `./scripts/build.sh` completes
- [ ] Manual smoke test of affected commands
- [ ] Commit with descriptive message

---

## Progress Tracking

| Phase | Item | Status |
|-------|------|--------|
| 1 | Fix LoadIndex loop | ✅ Done |
| 2.1 | LoadConfigAndUI helper | ✅ Done |
| 2.2 | SaveIndexWithTags atomic | ✅ Done |
| 2.3 | FormatJSONFile generic | ✅ Done |
| 3.1 | Merge tag parsers | ⬜ Pending |
| 3.2 | Mode handler abstraction | ⬜ Pending |
| 4.1 | Break up NoteCommand | ⬜ Pending |
| 4.1 | Break up SearchCommand | ⬜ Pending |
| 4.1 | Break up SelectCommand | ⬜ Pending |
| 4.2 | Custom error types | ⬜ Pending |
| 4.3 | Fix layer violations | ⬜ Pending |
