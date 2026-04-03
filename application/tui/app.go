package tui

import (
	"fmt"
	"strings"

	"breachline/app/interfaces"
	"breachline/tui/adapters"
	T "breachline/tui/types"
	"breachline/tui/views"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CLIOptions holds command-line arguments parsed by the entry point.
type CLIOptions struct {
	Files       []string
	Query       string
	Timezone    string
	IngestTZ    string
	NoHeader    bool
	JPath       string
	Workspace   string
	LicensePath string
	NoSort      bool
	SortDesc    bool
}

// App is the root Bubble Tea model composing all TUI views.
type App struct {
	backend *adapters.Backend
	opts    CLIOptions
	theme   T.Theme
	keys    T.KeyMap

	mode       T.VimMode
	pendingKey *T.PendingKey
	ready      bool
	quitting   bool

	tabs      []tabState
	activeTab int

	grid      views.Grid
	statusBar views.StatusBar

	width  int
	height int
}

type tabState struct {
	id       string
	fileName string
	filePath string
	fileHash string
	headers  []string
	query    string
}

// NewApp creates the root TUI model.
func NewApp(backend *adapters.Backend, opts CLIOptions) App {
	theme := T.DefaultTheme()
	return App{
		backend:   backend,
		opts:      opts,
		theme:     theme,
		keys:      T.DefaultKeyMap(),
		mode:      T.ModeNormal,
		grid:      views.NewGrid(theme),
		statusBar: views.NewStatusBar(theme),
	}
}

// Init starts the TUI; opens files provided on the command line.
func (a App) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, path := range a.opts.Files {
		fileOpts := interfaces.FileOptions{
			JPath:                  a.opts.JPath,
			NoHeaderRow:            a.opts.NoHeader,
			IngestTimezoneOverride: a.opts.IngestTZ,
		}
		cmds = append(cmds, openFileCmd(a.backend, path, fileOpts))
	}
	return tea.Batch(cmds...)
}

// Update is the main message handler.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		gridMsg := tea.WindowSizeMsg{Width: msg.Width, Height: a.gridHeight()}
		a.grid, _ = a.grid.Update(gridMsg)
		a.statusBar, _ = a.statusBar.Update(msg)

	case T.FileOpenedMsg:
		a.tabs = append(a.tabs, tabState{
			id: msg.TabID, fileName: msg.FileName, filePath: msg.FilePath,
			fileHash: msg.FileHash, headers: msg.Headers,
		})
		a.activeTab = len(a.tabs) - 1
		a.statusBar, _ = a.statusBar.Update(msg)
		cmds = append(cmds, loadRowsCmd(a.backend, msg.TabID, a.opts.Query, 0, defaultPageSize))

	case T.FileErrorMsg:
		a.statusBar.SetMessage(fmt.Sprintf("Error: %v", msg.Err), "error")

	case T.RowsLoadedMsg:
		a.grid, _ = a.grid.Update(msg)
		a.statusBar, _ = a.statusBar.Update(msg)

	case T.RowsErrorMsg:
		a.statusBar.SetMessage(fmt.Sprintf("Query error: %v", msg.Err), "error")

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	return a, tea.Batch(cmds...)
}

const defaultPageSize = 100

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, a.keys.Escape) {
		a.mode = T.ModeNormal
		a.pendingKey = nil
		a.grid.ClearSelection()
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: T.ModeNormal})
		return a, nil
	}

	switch a.mode {
	case T.ModeNormal:
		return a.handleNormalMode(msg)
	case T.ModeVisual:
		return a.handleVisualMode(msg)
	case T.ModeCommand, T.ModeInsert:
		return a, nil
	}
	return a, nil
}

func (a App) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle multi-key sequences
	if a.pendingKey != nil && a.pendingKey.Key == "g" {
		switch msg.String() {
		case "g":
			a.grid.GotoTop()
			a.pendingKey = nil
			a.updateCursorInStatusBar()
			return a, nil
		case "t":
			a.pendingKey = nil
			return a.nextTab()
		case "T":
			a.pendingKey = nil
			return a.prevTab()
		default:
			a.pendingKey = nil
		}
	}

	switch {
	case key.Matches(msg, a.keys.Quit):
		if len(a.tabs) == 0 {
			a.quitting = true
			return a, tea.Quit
		}
		return a.closeCurrentTab()

	case key.Matches(msg, a.keys.Down):
		a.grid.MoveDown(1)
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.Up):
		a.grid.MoveUp(1)
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.Left):
		a.grid.MoveLeft(1)
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.Right):
		a.grid.MoveRight(1)
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.HalfPgDn):
		a.grid.HalfPageDown()
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.HalfPgUp):
		a.grid.HalfPageUp()
		a.updateCursorInStatusBar()

	case key.Matches(msg, a.keys.GotoTop):
		a.pendingKey = &T.PendingKey{Key: "g"}
		return a, nil
	case key.Matches(msg, a.keys.GotoEnd):
		a.grid.GotoBottom()
		a.updateCursorInStatusBar()

	case key.Matches(msg, a.keys.VisualMode):
		if a.grid.ToggleVisualMode() {
			a.mode = T.ModeVisual
		} else {
			a.mode = T.ModeNormal
		}
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: a.mode})

	case key.Matches(msg, a.keys.Search):
		a.mode = T.ModeInsert
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: a.mode})
	case key.Matches(msg, a.keys.Command):
		a.mode = T.ModeCommand
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: a.mode})

	case key.Matches(msg, a.keys.Help):
		a.statusBar.SetMessage("hjkl:move  /:search  ::command  v:select  q:quit  ?:help", "info")

	case key.Matches(msg, a.keys.NextTab):
		return a.nextTab()
	case key.Matches(msg, a.keys.PrevTab):
		return a.prevTab()
	case key.Matches(msg, a.keys.CloseTab):
		return a.closeCurrentTab()
	}

	return a, nil
}

func (a App) handleVisualMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Down):
		a.grid.MoveDown(1)
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.Up):
		a.grid.MoveUp(1)
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.HalfPgDn):
		a.grid.HalfPageDown()
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.HalfPgUp):
		a.grid.HalfPageUp()
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.GotoEnd):
		a.grid.GotoBottom()
		a.updateCursorInStatusBar()
	case key.Matches(msg, a.keys.VisualMode):
		a.grid.ClearSelection()
		a.mode = T.ModeNormal
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: a.mode})
	case key.Matches(msg, a.keys.Yank):
		start, end := a.grid.SelectedRange()
		count := end - start + 1
		a.grid.ClearSelection()
		a.mode = T.ModeNormal
		a.statusBar.SetMessage(fmt.Sprintf("%d rows yanked", count), "info")
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: a.mode})
	}
	return a, nil
}

func (a *App) updateCursorInStatusBar() {
	a.statusBar.SetCursor(a.grid.CursorRow(), a.grid.CursorCol())
}

func (a App) nextTab() (tea.Model, tea.Cmd) {
	if len(a.tabs) <= 1 {
		return a, nil
	}
	a.activeTab = (a.activeTab + 1) % len(a.tabs)
	return a, loadRowsCmd(a.backend, a.tabs[a.activeTab].id, a.tabs[a.activeTab].query, 0, defaultPageSize)
}

func (a App) prevTab() (tea.Model, tea.Cmd) {
	if len(a.tabs) <= 1 {
		return a, nil
	}
	a.activeTab--
	if a.activeTab < 0 {
		a.activeTab = len(a.tabs) - 1
	}
	return a, loadRowsCmd(a.backend, a.tabs[a.activeTab].id, a.tabs[a.activeTab].query, 0, defaultPageSize)
}

func (a App) closeCurrentTab() (tea.Model, tea.Cmd) {
	if len(a.tabs) == 0 {
		a.quitting = true
		return a, tea.Quit
	}
	tabID := a.tabs[a.activeTab].id
	_ = a.backend.CloseTab(tabID)
	a.tabs = append(a.tabs[:a.activeTab], a.tabs[a.activeTab+1:]...)
	if a.activeTab >= len(a.tabs) {
		a.activeTab = len(a.tabs) - 1
	}
	if len(a.tabs) == 0 {
		a.quitting = true
		return a, tea.Quit
	}
	return a, loadRowsCmd(a.backend, a.tabs[a.activeTab].id, a.tabs[a.activeTab].query, 0, defaultPageSize)
}

func (a App) gridHeight() int {
	h := a.height - 2
	if h < 1 {
		h = 1
	}
	return h
}

// View renders the full TUI.
func (a App) View() string {
	if a.quitting {
		return ""
	}
	if !a.ready {
		return "Initialising…"
	}

	var sections []string
	if len(a.tabs) > 1 {
		sections = append(sections, a.renderTabBar())
	}
	sections = append(sections, a.grid.View())
	sections = append(sections, a.statusBar.View())
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (a App) renderTabBar() string {
	var parts []string
	for i, tab := range a.tabs {
		label := tab.fileName
		if label == "" {
			label = fmt.Sprintf("Tab %d", i+1)
		}
		if i == a.activeTab {
			parts = append(parts, a.theme.TabActive.Render(label))
		} else {
			parts = append(parts, a.theme.TabInactive.Render(label))
		}
	}
	bar := strings.Join(parts, a.theme.GridBorder.Render("│"))
	w := lipgloss.Width(bar)
	if w < a.width {
		bar += a.theme.TabBar.Render(strings.Repeat(" ", a.width-w))
	}
	return bar
}
