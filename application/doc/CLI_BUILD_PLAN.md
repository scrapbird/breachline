# BreachLine CLI Build - Implementation Plan

## Overview

This document describes the plan for building a standalone CLI (TUI) version of BreachLine that is feature-complete with the existing Wails desktop application. The CLI build will be a separate binary that shares the same Go backend packages but replaces the Wails/React frontend with an interactive terminal UI.

## Technology Choice: Bubble Tea (charmbracelet)

**Framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) (charmbracelet/bubbletea)

**Supporting libraries:**
- **Bubbles** (`charmbracelet/bubbles`) - Reusable TUI components (tables, text inputs, lists, viewports, spinners, pagination)
- **Lipgloss** (`charmbracelet/lipgloss`) - Terminal styling and layout (borders, colours, padding, alignment)
- **Huh** (`charmbracelet/huh`) - Interactive forms and prompts (settings dialogs, file options)

**Why Bubble Tea:**
- Elm-inspired Model-View-Update architecture - clean, testable, composable
- Most actively maintained Go TUI framework (2025/2026)
- Rich component ecosystem via Bubbles (tables, lists, text inputs, viewports, fuzzy filtering)
- Fully customisable key bindings - vim bindings can be implemented cleanly via the message-driven architecture
- Professional styling via Lipgloss with no external dependencies
- Supports both full-screen and inline rendering modes
- Large community with proven complex applications (LazyDocker, Glow, Soft Serve)

## Vim Keybinding Philosophy

All views will favour vim-style navigation as the primary interaction model:

| Action | Binding |
|---|---|
| Navigation | `h` `j` `k` `l` for left/down/up/right |
| Page movement | `Ctrl+d` / `Ctrl+u` for half-page down/up |
| Jump to top/bottom | `g g` / `G` |
| Search | `/` to enter search, `n` / `N` for next/prev match |
| Command mode | `:` to enter command/query input |
| Quit | `q` or `:q` |
| Tab navigation | `g t` / `g T` for next/prev tab, `<n>gt` to go to tab n |
| Close tab | `:tabclose` or `:q` when in tab |
| Select | `v` for visual select, `V` for line select |
| Copy | `y` to yank selection |
| Annotate | `a` on selected rows |
| Open file | `:e <path>` or `:open <path>` |
| Fuzzy finder | `Ctrl+p` (file finder) |
| Toggle panels | `Ctrl+h` histogram, `` Ctrl+` `` console |
| Settings | `:set` or `:settings` |
| Help | `:help` or `?` |

## Architecture

### Directory Structure

```
application/
├── main.go                  # Existing Wails entry point (unchanged)
├── cmd/
│   └── breachline-cli/
│       └── main.go          # CLI entry point
├── app/                     # Shared backend (unchanged)
│   ├── app.go
│   ├── cache/
│   ├── fileloader/
│   ├── histogram/
│   ├── interfaces/
│   ├── plugin/
│   ├── query/
│   ├── settings/
│   ├── sync/
│   ├── timestamps/
│   └── workspace/
├── tui/                     # New: CLI frontend
│   ├── app.go               # Root Bubble Tea model
│   ├── keys.go              # Global vim keybinding definitions
│   ├── theme.go             # Lipgloss theme/style definitions
│   ├── commands.go          # Bubble Tea commands (async operations)
│   ├── messages.go          # Custom message types
│   ├── views/
│   │   ├── grid.go          # Data grid view (table + scrolling)
│   │   ├── dashboard.go     # Workspace file browser
│   │   ├── histogram.go     # ASCII histogram (sparkline/bar chart)
│   │   ├── search_bar.go    # Query input with history
│   │   ├── tab_bar.go       # Tab strip
│   │   ├── console.go       # Log panel
│   │   ├── results_panel.go # Annotations + search results
│   │   └── status_bar.go    # Bottom status line (vim-style)
│   ├── dialogs/
│   │   ├── settings.go      # Settings form
│   │   ├── annotation.go    # Annotation editor
│   │   ├── file_options.go  # File open options (jpath, no-header, tz)
│   │   ├── fuzzy_finder.go  # Fuzzy file finder
│   │   ├── cell_viewer.go   # Full cell content viewer
│   │   ├── confirm.go       # Generic yes/no confirmation
│   │   ├── about.go         # About / version info
│   │   ├── shortcuts.go     # Keyboard shortcuts reference
│   │   ├── syntax_help.go   # Query syntax reference
│   │   └── license.go       # License import dialog
│   └── adapters/
│       └── backend.go       # Adapter layer between TUI and app package
└── frontend/                # Existing React frontend (unchanged)
```

### Build Configuration

Two separate build targets:

```makefile
# Desktop (Wails) build - existing
build-desktop:
	cd application && wails build -webview2 browser -tags webkit2_42

# CLI build - new
build-cli:
	cd application && go build -o breachline-cli ./cmd/breachline-cli/
```

The CLI binary will have no dependency on Wails, WebKit, GTK, or any GUI libraries. It will be a pure terminal application that can run on headless servers and over SSH.

### Backend Adapter Layer

The existing `app` package is tightly coupled to Wails via `wails/v2/pkg/runtime` for:
- File dialogs (`runtime.OpenFileDialog`, `runtime.SaveFileDialog`)
- Window title updates (`runtime.WindowSetTitle`)
- Event emission (`runtime.EventsEmit`)
- Context injection (`context.Context` from Wails)

The adapter layer (`tui/adapters/backend.go`) will:

1. **Create a CLI-compatible App instance** that initialises the same services (`App`, `SettingsService`, `LicenseService`, `WorkspaceManager`) without Wails context
2. **Replace Wails runtime calls** with CLI equivalents:
   - File dialogs → fuzzy finder / path input with tab completion
   - Window title → terminal title escape sequence or status bar
   - Events → Bubble Tea messages via `Program.Send()`
3. **Expose the same operations** that the React frontend calls, translating between Bubble Tea messages and backend method calls

A key design decision: the adapter will call backend methods directly (they are Go functions in the same process) rather than going through any IPC layer. This is simpler and faster than the Wails binding mechanism.

## Implementation Phases

### Phase 1: Core Framework & File Viewing

**Goal:** Open a single file and view its data in the terminal with vim navigation.

**Tasks:**

1. **CLI entry point** (`cmd/breachline-cli/main.go`)
   - Parse CLI arguments: file path(s) to open, optional flags (`--query`, `--timezone`, `--no-header`)
   - Initialise backend services (App, Settings, License)
   - Create and start Bubble Tea program
   - Handle clean shutdown (save settings, close files)

2. **Root model** (`tui/app.go`)
   - Top-level Bubble Tea model composing all child views
   - Vim mode state machine: `Normal` → `Insert` (search bar) → `Visual` (selection) → `Command` (`:` commands)
   - Layout manager: divide terminal into regions (tab bar, search bar, grid, histogram, console, status bar)
   - Focus management: track which view receives key events
   - Dialog overlay system: modal dialogs render on top of main layout

3. **Theme & styling** (`tui/theme.go`)
   - Dark theme matching the desktop app aesthetic
   - Lipgloss style definitions for all components
   - Consistent colour palette: borders, highlights, selections, annotation colours
   - Responsive layout calculations based on terminal dimensions

4. **Vim keybindings** (`tui/keys.go`)
   - Key binding registry with context-aware dispatch
   - Multi-key sequence support (e.g., `gg`, `gt`, `gT`)
   - Mode-specific bindings (Normal vs Insert vs Visual vs Command)
   - Configurable key timeout for multi-key sequences

5. **Data grid** (`tui/views/grid.go`)
   - Paginated table using `bubbles/table` as base, extended with:
     - Vim navigation (`hjkl`, `gg`/`G`, `Ctrl+d`/`Ctrl+u`)
     - Column scrolling for wide datasets (horizontal panning)
     - Visual mode row selection (`v` to start, movement to extend)
     - Annotation colour highlighting (row background tinting)
     - Cell truncation with width-aware ellipsis
     - Column width auto-sizing based on content sampling
   - Infinite scroll: fetch pages from backend on demand (same `GetCSVRowsFiltered` pagination as desktop)
   - Row count indicator in status bar

6. **Status bar** (`tui/views/status_bar.go`)
   - Vim-style bottom bar showing: mode indicator, file info, row count, cursor position
   - Command input when in `:` mode
   - Message display area for feedback (e.g., "3 rows copied")

7. **Backend adapter - file operations** (`tui/adapters/backend.go`)
   - `OpenFile(path string, opts FileOptions) → tab ID, header, error`
   - `GetRows(tabID, query, startRow, count) → RowsPage`
   - `GetRowCount(tabID) → int`
   - File type auto-detection (CSV, XLSX, JSON)
   - JSON file handling with JSONPath expression

### Phase 2: Search, Filtering & Query

**Goal:** Full query language support with search highlighting.

**Tasks:**

1. **Search bar** (`tui/views/search_bar.go`)
   - Text input activated by `/` (search) or `:` (command)
   - Query history with up/down arrow navigation
   - Fuzzy history filtering as you type
   - Enter to execute, Escape to cancel
   - Error state display (red border on invalid query)
   - SPL syntax support (same query language as desktop)

2. **Search result highlighting**
   - Highlight matching cells in the grid when search is active
   - `n` / `N` to jump between matches
   - Match count display in status bar

3. **Find in grid** (`/` search)
   - Regex and plain text modes (toggle with `Ctrl+r`)
   - Real-time search as you type
   - Results panel showing matches with context

4. **Time filter integration**
   - `after` / `before` query syntax working with histogram brush
   - Relative time expressions (`5m ago`, `2h`, `now`)

5. **Column projection**
   - `| columns` pipeline stage rendering only selected columns
   - Column visibility toggling via command (`:columns hide colA`)

6. **Dedup & limit stages**
   - Full pipeline support matching desktop query engine

### Phase 3: Multi-Tab & Workspace

**Goal:** Multiple files open simultaneously with workspace persistence.

**Tasks:**

1. **Tab bar** (`tui/views/tab_bar.go`)
   - Horizontal tab strip at top of screen
   - Active tab highlighting
   - File type indicators
   - Tab switching: `gt`/`gT`, `<n>gt`, or `:tabnext`/`:tabprev`
   - Close tab: `:tabclose` or `:q`
   - New tab: `:tabnew` or `:e <file>`

2. **Dashboard view** (`tui/views/dashboard.go`)
   - Shown when no tabs are open or via `:dashboard`
   - List workspace files with annotations count
   - File descriptions
   - Enter to open file in new tab
   - Vim list navigation (`j`/`k`)

3. **Workspace operations**
   - `:workspace open` - file picker for .breachline files
   - `:workspace close` - close active workspace
   - `:workspace add` - add current file to workspace
   - `:workspace export` - export merged timeline
   - All workspace features gated on license (same as desktop)

4. **Annotation system**
   - `a` key on selected rows opens annotation dialog
   - Annotation dialog: note text input + colour picker (numbered 1-6)
   - Annotation panel: toggled view showing all annotations
   - Colour-coded row highlighting in grid
   - Annotation persistence to workspace file

5. **Fuzzy file finder** (`tui/dialogs/fuzzy_finder.go`)
   - `Ctrl+p` to open
   - Fuzzy matching against workspace files and recent files
   - File status badges (JPath, NoHeader, Timezone)
   - vim navigation in results list

### Phase 4: Histogram & Visualisation

**Goal:** Time-series visualisation in the terminal.

**Tasks:**

1. **ASCII histogram** (`tui/views/histogram.go`)
   - Horizontal bar chart using Unicode block characters (▁▂▃▄▅▆▇█)
   - Time bucket labels on X axis (timezone-aware)
   - Count labels on Y axis
   - Responsive width based on terminal columns
   - Toggled with `Ctrl+h` or `:histogram`

2. **Histogram interaction**
   - Navigate buckets with `h`/`l` when histogram is focused
   - Select time range: `v` to start visual select, movement to extend, Enter to apply as filter
   - Selected range creates `after`/`before` query filter
   - Bucket tooltip showing exact time range and count

3. **Sparkline mode**
   - Compact single-line histogram above the grid
   - Uses sparkline characters (▁▂▃▄▅▆▇) for minimal space usage
   - Toggle between full and sparkline mode

### Phase 5: Settings, License & Polish

**Goal:** Complete feature parity and production readiness.

**Tasks:**

1. **Settings dialog** (`tui/dialogs/settings.go`)
   - Form-based settings using `charmbracelet/huh`
   - Tabs: General, Cache, Plugins
   - Sort settings, timezone selection, cache toggle
   - Plugin enable/disable
   - Save/Cancel with Escape/Enter

2. **License management**
   - `:license import <path>` to import license file
   - `:license info` to show license details
   - License validation at startup
   - Expiration warnings in status bar

3. **Plugin support**
   - Plugin loading and file type registration
   - Plugin selection dialog when multiple plugins match
   - Plugin configuration in settings

4. **Clipboard operations**
   - `y` to yank (copy) selected rows to clipboard
   - TSV format for spreadsheet compatibility
   - Selection count feedback in status bar
   - Fallback: write to file if clipboard unavailable (headless/SSH)

5. **Cell viewer** (`tui/dialogs/cell_viewer.go`)
   - Enter on a cell opens full content viewer
   - JSON pretty-printing with syntax highlighting
   - Scrollable viewport for large content
   - Copy cell content to clipboard

6. **Console panel** (`tui/views/console.go`)
   - Toggled with `` Ctrl+` `` or `:console`
   - Resizable panel at bottom
   - Level-based colouring (info=white, warn=yellow, error=red)
   - Auto-scroll to latest entry
   - Clear with `:console clear`

7. **Help system**
   - `?` or `:help` shows keyboard shortcuts
   - `:syntax` shows query syntax reference
   - `:about` shows version and license info

8. **Sync service** (remote workspaces)
   - `:sync login` / `:sync logout`
   - `:sync list` - list remote workspaces
   - `:sync open <name>` - open remote workspace
   - PIN entry dialog for authentication

### Phase 6: Polish & Distribution

**Goal:** Production-quality CLI ready for release.

**Tasks:**

1. **Error handling**
   - Graceful error display in status bar
   - Error dialog for critical failures
   - Panic recovery with state preservation

2. **Terminal compatibility**
   - Test across: xterm-256color, tmux, screen, Windows Terminal, iTerm2, Alacritty
   - Fallback rendering for limited colour support
   - Mouse support (optional): click to select cells, scroll wheel

3. **Performance**
   - Profile and optimise rendering for large grids
   - Lazy column width calculation
   - Efficient viewport updates (only re-render changed cells)
   - Same query caching as desktop (backend handles this)

4. **Build & distribution**
   - Cross-compilation: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
   - Single static binary with no external dependencies
   - Version embedding via `-ldflags`
   - Add to existing release pipeline alongside desktop builds

5. **Documentation**
   - CLI-specific README with installation and usage
   - Vim keybinding cheat sheet
   - Man page generation

## Shared Code Strategy

The key principle is **maximum code reuse** with the existing backend:

| Layer | Shared? | Notes |
|---|---|---|
| `app/` (core logic) | Yes | All business logic, query engine, caching |
| `app/fileloader/` | Yes | CSV, XLSX, JSON file reading |
| `app/query/` | Yes | Query parsing and execution |
| `app/cache/` | Yes | Query result caching |
| `app/histogram/` | Yes | Histogram bucket calculation |
| `app/timestamps/` | Yes | Timestamp parsing and formatting |
| `app/settings/` | Yes | Settings persistence (same YAML file) |
| `app/workspace/` | Yes | Workspace file management |
| `app/plugin/` | Yes | Plugin loading and execution |
| `app/sync/` | Yes | Remote workspace sync |
| `app/interfaces/` | Yes | Shared type definitions |
| `main.go` | No | Wails-specific entry point |
| `frontend/` | No | React UI (desktop only) |
| `tui/` | No | New CLI-only TUI code |
| `cmd/breachline-cli/` | No | CLI entry point |

### Decoupling Wails Dependencies

Some backend code currently imports `wails/v2/pkg/runtime` for:
- `runtime.EventsEmit()` - used for histogram async events, workspace updates
- `runtime.OpenFileDialog()` / `runtime.SaveFileDialog()` - file pickers
- `runtime.WindowSetTitle()` - window title updates
- `runtime.LogInfo()` etc. - logging

**Strategy:** Introduce a thin `RuntimeAdapter` interface in `app/interfaces/`:

```go
type RuntimeAdapter interface {
    EmitEvent(name string, data ...interface{})
    Log(level string, message string)
    ShowOpenFileDialog(opts OpenDialogOptions) (string, error)
    ShowSaveFileDialog(opts SaveDialogOptions) (string, error)
    SetTitle(title string)
}
```

- The desktop build provides a Wails implementation
- The CLI build provides a Bubble Tea implementation that converts events to `tea.Msg` values
- Backend code calls methods on this interface instead of `wails/v2/pkg/runtime` directly

This decoupling is the most critical refactoring step and should be done first, as it unlocks all subsequent work.

## Command-Line Interface

```
breachline-cli [flags] [file...]

Flags:
  -q, --query <query>         Apply initial query
  -t, --timezone <tz>         Set display timezone
      --ingest-tz <tz>        Set ingest timezone
      --no-header             Treat first row as data (CSV)
  -j, --jpath <expr>          JSONPath expression (JSON files)
  -w, --workspace <path>      Open workspace file
  -l, --license <path>        Import license file
      --no-sort               Disable time-based sorting
      --sort-desc             Sort descending (default: ascending)
      --version               Show version
  -h, --help                  Show help

Examples:
  breachline-cli access.log
  breachline-cli -q "status=500 | after 1h" nginx.csv
  breachline-cli -j '$.events[*]' data.json
  breachline-cli -w investigation.breachline
  breachline-cli *.csv
```

## Testing Strategy

1. **Unit tests** for each TUI component (Bubble Tea models are pure functions - easy to test)
2. **Integration tests** using `teatest` package for end-to-end TUI interaction testing
3. **Backend tests** remain unchanged (shared code)
4. **Manual testing matrix:** Linux (xterm, tmux, Alacritty), macOS (Terminal.app, iTerm2), Windows (Windows Terminal)

## Risk Assessment

| Risk | Impact | Mitigation |
|---|---|---|
| Wails coupling in backend | High | Introduce RuntimeAdapter interface early (Phase 1 prerequisite) |
| Terminal rendering performance with large grids | Medium | Paginated virtual scrolling, same as desktop infinite scroll model |
| Clipboard on headless/SSH | Low | Fallback to file output, OSC 52 escape sequence for tmux |
| Column width calculation for wide datasets | Low | Sample first N rows, allow manual resize via keybinding |
| Bubble Tea component complexity | Medium | Leverage existing Bubbles components, avoid custom rendering where possible |

## Estimated Scope

- **Phase 1:** Core framework + file viewing - largest phase, establishes architecture
- **Phase 2:** Search & query - leverages existing backend query engine
- **Phase 3:** Multi-tab & workspace - composition of Phase 1 components
- **Phase 4:** Histogram - novel TUI rendering, ASCII chart design
- **Phase 5:** Settings & polish - forms, dialogs, edge cases
- **Phase 6:** Distribution - build pipeline, cross-compilation, docs
