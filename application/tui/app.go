package tui

import (
	"fmt"
	"strings"

	"breachline/app"
	"breachline/app/interfaces"
	"breachline/tui/adapters"
	"breachline/tui/dialogs"
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
	Version     string
}

const defaultPageSize = 100

// focusTarget identifies which panel currently receives key input.
type focusTarget int

const (
	focusGrid focusTarget = iota
	focusHistogram
	focusDashboard
)

// App is the root Bubble Tea model composing all TUI views.
type App struct {
	// Core
	backend *adapters.Backend
	opts    CLIOptions
	theme   T.Theme
	keys    T.KeyMap

	// State
	mode       T.VimMode
	pendingKey *T.PendingKey
	focus      focusTarget
	ready      bool
	quitting   bool

	// Tab management
	tabs      []tabState
	activeTab int

	// Views
	tabBar    views.TabBar
	searchBar views.SearchBar
	grid      views.Grid
	histogram views.Histogram
	console   views.Console
	statusBar views.StatusBar
	dashboard views.Dashboard

	// Dialogs
	annotationDlg  dialogs.Annotation
	fuzzyFinder    dialogs.FuzzyFinder
	cellViewer     dialogs.CellViewer
	confirmDlg     dialogs.Confirm
	shortcutsDlg   dialogs.Shortcuts
	syntaxDlg      dialogs.SyntaxHelp
	aboutDlg       dialogs.About
	settingsDlg    dialogs.Settings
	licenseDlg     dialogs.License
	fileOptionsDlg dialogs.FileOptions

	// Layout
	width  int
	height int
}

// tabState tracks per-tab metadata.
type tabState struct {
	id       string
	fileName string
	filePath string
	fileHash string
	fileType string
	headers  []string
	query    string
}

// NewApp creates the root TUI model.
func NewApp(backend *adapters.Backend, opts CLIOptions) App {
	theme := T.DefaultTheme()
	return App{
		backend:        backend,
		opts:           opts,
		theme:          theme,
		keys:           T.DefaultKeyMap(),
		mode:           T.ModeNormal,
		focus:          focusGrid,
		tabBar:         views.NewTabBar(theme),
		searchBar:      views.NewSearchBar(theme),
		grid:           views.NewGrid(theme),
		histogram:      views.NewHistogram(theme),
		console:        views.NewConsole(theme),
		statusBar:      views.NewStatusBar(theme),
		dashboard:      views.NewDashboard(theme),
		annotationDlg:  dialogs.NewAnnotation(theme),
		fuzzyFinder:    dialogs.NewFuzzyFinder(theme),
		cellViewer:     dialogs.NewCellViewer(theme),
		confirmDlg:     dialogs.NewConfirm(theme),
		shortcutsDlg:   dialogs.NewShortcuts(theme),
		syntaxDlg:      dialogs.NewSyntaxHelp(theme),
		aboutDlg:       dialogs.NewAbout(theme, opts.Version),
		settingsDlg:    dialogs.NewSettings(theme),
		licenseDlg:     dialogs.NewLicense(theme),
		fileOptionsDlg: dialogs.NewFileOptions(theme),
	}
}

// Init opens files provided on the command line.
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
	if a.opts.Workspace != "" {
		cmds = append(cmds, a.openWorkspaceCmd(a.opts.Workspace))
	}
	return tea.Batch(cmds...)
}

// Update is the main message handler.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// --- Dialog routing (dialogs consume events when visible) ---
	if cmd := a.routeToDialog(msg); cmd != nil {
		return a, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.propagateResize()

	case T.FileOpenedMsg:
		a.addTab(msg)
		a.statusBar, _ = a.statusBar.Update(msg)
		query := a.currentQuery()
		cmds = append(cmds, loadRowsCmd(a.backend, msg.TabID, query, 0, defaultPageSize))

	case T.FileErrorMsg:
		a.statusBar.SetMessage(fmt.Sprintf("Error: %v", msg.Err), "error")

	case T.RowsLoadedMsg:
		a.grid, _ = a.grid.Update(msg)
		a.statusBar, _ = a.statusBar.Update(msg)

	case T.RowsErrorMsg:
		a.statusBar.SetMessage(fmt.Sprintf("Query error: %v", msg.Err), "error")

	case searchResultMsg:
		if msg.err != nil {
			a.statusBar.SetMessage("Search error: "+msg.err.Error(), "error")
		} else {
			a.statusBar.SetMessage(views.FormatMatchCount(0, msg.totalCount), "info")
		}

	case annotationDoneMsg:
		a.statusBar.SetMessage(fmt.Sprintf("%d rows annotated", msg.count), "info")
		cmds = append(cmds, a.reloadCurrentTab())

	case copyDoneMsg:
		a.statusBar.SetMessage(fmt.Sprintf("%d rows copied", msg.count), "info")

	// Dialog results
	case dialogs.AnnotationSubmitMsg:
		cmds = append(cmds, a.handleAnnotationSubmit(msg))
	case dialogs.FuzzySelectMsg:
		cmds = append(cmds, openFileCmd(a.backend, msg.FilePath, interfaces.FileOptions{}))
	case dialogs.SettingsSaveMsg:
		a.handleSettingsSave(msg)
	case dialogs.FileOptionsSubmitMsg:
		cmds = append(cmds, openFileCmd(a.backend, msg.FilePath, interfaces.FileOptions{
			JPath:                  msg.JPath,
			NoHeaderRow:            msg.NoHeader,
			IngestTimezoneOverride: msg.IngestTZ,
		}))

	case adapters.BackendEvent:
		a.console.AddEntry("info", fmt.Sprintf("[event] %s", msg.Name))

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	return a, tea.Batch(cmds...)
}

// --- Key handling ---

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Escape always returns to normal mode
	if key.Matches(msg, a.keys.Escape) {
		if a.searchBar.Visible() {
			a.searchBar.Hide()
		}
		a.mode = T.ModeNormal
		a.pendingKey = nil
		a.focus = focusGrid
		a.grid.ClearSelection()
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: T.ModeNormal})
		return a, nil
	}

	// Route to search bar if active
	if a.searchBar.Visible() && (a.mode == T.ModeInsert || a.mode == T.ModeCommand) {
		return a.handleSearchBarKey(msg)
	}

	switch a.mode {
	case T.ModeNormal:
		if a.focus == focusHistogram {
			return a.handleHistogramKeys(msg)
		}
		if a.focus == focusDashboard {
			return a.handleDashboardKeys(msg)
		}
		return a.handleNormalMode(msg)
	case T.ModeVisual:
		return a.handleVisualMode(msg)
	}
	return a, nil
}

func (a App) handleSearchBarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := a.searchBar.Value()
		if a.searchBar.Mode() == views.SearchModeCommand {
			return a.executeCommand(val)
		}
		// Search mode: execute query
		a.searchBar.AddHistory(val)
		a.searchBar.Hide()
		a.mode = T.ModeNormal
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: T.ModeNormal})
		if len(a.tabs) > 0 {
			a.tabs[a.activeTab].query = val
			return a, loadRowsCmd(a.backend, a.tabs[a.activeTab].id, val, 0, defaultPageSize)
		}
		return a, nil
	}
	var cmd tea.Cmd
	a.searchBar, cmd = a.searchBar.Update(msg)
	return a, cmd
}

func (a App) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Multi-key sequences
	if a.pendingKey != nil && a.pendingKey.Key == "g" {
		switch msg.String() {
		case "g":
			a.grid.GotoTop()
			a.pendingKey = nil
			a.syncCursor()
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
	// Quit / close tab
	case key.Matches(msg, a.keys.Quit):
		if len(a.tabs) == 0 {
			a.quitting = true
			return a, tea.Quit
		}
		return a.closeCurrentTab()

	// Navigation
	case key.Matches(msg, a.keys.Down):
		a.grid.MoveDown(1)
		a.syncCursor()
	case key.Matches(msg, a.keys.Up):
		a.grid.MoveUp(1)
		a.syncCursor()
	case key.Matches(msg, a.keys.Left):
		a.grid.MoveLeft(1)
		a.syncCursor()
	case key.Matches(msg, a.keys.Right):
		a.grid.MoveRight(1)
		a.syncCursor()
	case key.Matches(msg, a.keys.HalfPgDn):
		a.grid.HalfPageDown()
		a.syncCursor()
	case key.Matches(msg, a.keys.HalfPgUp):
		a.grid.HalfPageUp()
		a.syncCursor()
	case key.Matches(msg, a.keys.GotoTop):
		a.pendingKey = &T.PendingKey{Key: "g"}
		return a, nil
	case key.Matches(msg, a.keys.GotoEnd):
		a.grid.GotoBottom()
		a.syncCursor()

	// Modes
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
		return a, a.searchBar.Show(views.SearchModeSearch)
	case key.Matches(msg, a.keys.Command):
		a.mode = T.ModeCommand
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: a.mode})
		return a, a.searchBar.Show(views.SearchModeCommand)

	// Cell viewer
	case key.Matches(msg, a.keys.Enter):
		if a.grid.HasData() {
			col, cell := a.grid.CellAt(a.grid.CursorRow(), a.grid.CursorCol())
			a.cellViewer.Show(col, cell, a.width, a.height)
		}

	// Toggles
	case key.Matches(msg, a.keys.ToggleHistogram):
		a.histogram.Toggle()
		if a.histogram.Visible() {
			a.focus = focusHistogram
		} else {
			a.focus = focusGrid
		}
	case key.Matches(msg, a.keys.ToggleConsole):
		a.console.Toggle()
	case key.Matches(msg, a.keys.FuzzyFinder):
		return a, a.showFuzzyFinder()

	// Tabs
	case key.Matches(msg, a.keys.NextTab):
		return a.nextTab()
	case key.Matches(msg, a.keys.PrevTab):
		return a.prevTab()
	case key.Matches(msg, a.keys.CloseTab):
		return a.closeCurrentTab()

	// Help
	case key.Matches(msg, a.keys.Help):
		a.shortcutsDlg.Show(a.width, a.height)

	// Annotate
	case key.Matches(msg, a.keys.Annotate):
		return a, a.annotationDlg.Show("", "grey")

	// Yank
	case key.Matches(msg, a.keys.Yank):
		return a, a.yankCurrentRow()
	}

	return a, nil
}

func (a App) handleVisualMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Down):
		a.grid.MoveDown(1)
		a.syncCursor()
	case key.Matches(msg, a.keys.Up):
		a.grid.MoveUp(1)
		a.syncCursor()
	case key.Matches(msg, a.keys.HalfPgDn):
		a.grid.HalfPageDown()
		a.syncCursor()
	case key.Matches(msg, a.keys.HalfPgUp):
		a.grid.HalfPageUp()
		a.syncCursor()
	case key.Matches(msg, a.keys.GotoEnd):
		a.grid.GotoBottom()
		a.syncCursor()
	case key.Matches(msg, a.keys.VisualMode):
		a.grid.ClearSelection()
		a.mode = T.ModeNormal
		a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: a.mode})
	case key.Matches(msg, a.keys.Yank):
		return a, a.yankSelection()
	case key.Matches(msg, a.keys.Annotate):
		return a, a.annotationDlg.Show("", "grey")
	}
	return a, nil
}

func (a App) handleHistogramKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Left):
		a.histogram.MoveLeft()
	case key.Matches(msg, a.keys.Right):
		a.histogram.MoveRight()
	case key.Matches(msg, a.keys.VisualMode):
		a.histogram.ToggleSelect()
	case key.Matches(msg, a.keys.Enter):
		// Apply histogram selection as time filter
		startMs, endMs := a.histogram.SelectedRange()
		if startMs > 0 && endMs > 0 && len(a.tabs) > 0 {
			// Build time filter query
			query := fmt.Sprintf("| after %d | before %d", startMs, endMs)
			a.tabs[a.activeTab].query = query
			a.focus = focusGrid
			return a, loadRowsCmd(a.backend, a.tabs[a.activeTab].id, query, 0, defaultPageSize)
		}
	case key.Matches(msg, a.keys.Quit), key.Matches(msg, a.keys.ToggleHistogram):
		a.focus = focusGrid
	}
	return a, nil
}

func (a App) handleDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Down):
		a.dashboard.MoveDown()
	case key.Matches(msg, a.keys.Up):
		a.dashboard.MoveUp()
	case key.Matches(msg, a.keys.Enter):
		path := a.dashboard.SelectedFile()
		if path != "" {
			return a, openFileCmd(a.backend, path, interfaces.FileOptions{})
		}
	case key.Matches(msg, a.keys.Quit):
		a.quitting = true
		return a, tea.Quit
	}
	return a, nil
}

// --- Command execution ---

func (a App) executeCommand(cmdStr string) (tea.Model, tea.Cmd) {
	a.searchBar.AddHistory(cmdStr)
	a.searchBar.Hide()
	a.mode = T.ModeNormal
	a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: T.ModeNormal})

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return a, nil
	}

	switch parts[0] {
	case "q", "quit":
		return a.closeCurrentTab()

	case "e", "open":
		if len(parts) < 2 {
			a.statusBar.SetMessage("Usage: :e <file>", "warn")
			return a, nil
		}
		return a, openFileCmd(a.backend, parts[1], interfaces.FileOptions{
			JPath:       a.opts.JPath,
			NoHeaderRow: a.opts.NoHeader,
		})

	case "tabnext":
		return a.nextTab()
	case "tabprev":
		return a.prevTab()
	case "tabclose":
		return a.closeCurrentTab()

	case "set", "settings":
		s := a.backend.GetSettings()
		a.settingsDlg.Show(s.DisplayTimezone, s.DefaultIngestTimezone, s.TimestampDisplayFormat,
			s.SortByTime, s.SortDescending, s.EnableQueryCache, s.EnablePlugins)
		return a, nil

	case "help":
		a.shortcutsDlg.Show(a.width, a.height)
		return a, nil

	case "syntax":
		a.syntaxDlg.Show(a.width, a.height)
		return a, nil

	case "about":
		licensed := a.backend.IsLicensed()
		info := ""
		if details, err := a.backend.GetLicenseDetails(); err == nil {
			info = details.Message
		}
		a.aboutDlg.Show(licensed, info)
		return a, nil

	case "histogram":
		a.histogram.Toggle()
		return a, nil

	case "console":
		if len(parts) > 1 && parts[1] == "clear" {
			a.console.Clear()
			return a, nil
		}
		a.console.Toggle()
		return a, nil

	case "dashboard":
		a.focus = focusDashboard
		return a, nil

	case "license":
		if len(parts) > 1 {
			switch parts[1] {
			case "import":
				if len(parts) < 3 {
					a.statusBar.SetMessage("Usage: :license import <path>", "warn")
					return a, nil
				}
				return a, a.licenseDlg.Show(parts[2])
			case "info":
				details, _ := a.backend.GetLicenseDetails()
				if details != nil && details.Licensed {
					a.licenseDlg.ShowMessage(details.Message, false)
				} else {
					a.licenseDlg.ShowMessage("No valid license", true)
				}
				return a, nil
			}
		}
		return a, a.licenseDlg.Show("")

	case "workspace":
		if len(parts) > 1 {
			switch parts[1] {
			case "open":
				if len(parts) < 3 {
					a.statusBar.SetMessage("Usage: :workspace open <path>", "warn")
					return a, nil
				}
				return a, a.openWorkspaceCmd(parts[2])
			case "close":
				if err := a.backend.CloseWorkspace(); err != nil {
					a.statusBar.SetMessage("Close workspace: "+err.Error(), "error")
				} else {
					a.statusBar.SetMessage("Workspace closed", "info")
				}
				return a, nil
			}
		}
		a.statusBar.SetMessage("Usage: :workspace open|close <path>", "warn")
		return a, nil

	default:
		// Treat as a query
		if len(a.tabs) > 0 {
			a.tabs[a.activeTab].query = cmdStr
			return a, loadRowsCmd(a.backend, a.tabs[a.activeTab].id, cmdStr, 0, defaultPageSize)
		}
		a.statusBar.SetMessage("Unknown command: "+parts[0], "warn")
		return a, nil
	}
}

// --- Helpers ---

func (a *App) syncCursor() {
	a.statusBar.SetCursor(a.grid.CursorRow(), a.grid.CursorCol())
}

func (a *App) addTab(msg T.FileOpenedMsg) {
	a.tabs = append(a.tabs, tabState{
		id: msg.TabID, fileName: msg.FileName, filePath: msg.FilePath,
		fileHash: msg.FileHash, headers: msg.Headers,
	})
	a.activeTab = len(a.tabs) - 1
	a.focus = focusGrid
	a.syncTabBar()
}

func (a *App) syncTabBar() {
	tabs := make([]views.TabInfo, len(a.tabs))
	for i, t := range a.tabs {
		tabs[i] = views.TabInfo{ID: t.id, FileName: t.fileName, FileType: t.fileType}
	}
	a.tabBar.SetTabs(tabs, a.activeTab)
}

func (a App) currentQuery() string {
	if a.opts.Query != "" {
		return a.opts.Query
	}
	if a.activeTab >= 0 && a.activeTab < len(a.tabs) {
		return a.tabs[a.activeTab].query
	}
	return ""
}

func (a App) nextTab() (tea.Model, tea.Cmd) {
	if len(a.tabs) <= 1 {
		return a, nil
	}
	a.activeTab = (a.activeTab + 1) % len(a.tabs)
	a.syncTabBar()
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
	a.syncTabBar()
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
	a.syncTabBar()
	if len(a.tabs) == 0 {
		a.quitting = true
		return a, tea.Quit
	}
	return a, loadRowsCmd(a.backend, a.tabs[a.activeTab].id, a.tabs[a.activeTab].query, 0, defaultPageSize)
}

func (a App) reloadCurrentTab() tea.Cmd {
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return nil
	}
	tab := a.tabs[a.activeTab]
	return loadRowsCmd(a.backend, tab.id, tab.query, 0, defaultPageSize)
}

func (a App) yankCurrentRow() tea.Cmd {
	if len(a.tabs) == 0 || !a.grid.HasData() {
		return nil
	}
	row := a.grid.CursorRow()
	tab := a.tabs[a.activeTab]
	return copySelectionCmd(a.backend, []app.RangeSpec{{Start: row, End: row}}, tab.headers, tab.query, "")
}

func (a App) yankSelection() tea.Cmd {
	if len(a.tabs) == 0 {
		return nil
	}
	start, end := a.grid.SelectedRange()
	a.grid.ClearSelection()
	a.mode = T.ModeNormal
	a.statusBar, _ = a.statusBar.Update(T.ModeChangedMsg{Mode: T.ModeNormal})
	tab := a.tabs[a.activeTab]
	return copySelectionCmd(a.backend, []app.RangeSpec{{Start: start, End: end}}, tab.headers, tab.query, "")
}

func (a App) handleAnnotationSubmit(msg dialogs.AnnotationSubmitMsg) tea.Cmd {
	if len(a.tabs) == 0 {
		return nil
	}
	start, end := a.grid.SelectedRange()
	a.grid.ClearSelection()
	a.mode = T.ModeNormal
	tab := a.tabs[a.activeTab]
	return annotateCmd(a.backend, msg.Note, msg.Color, tab.query, "", []app.RangeSpec{{Start: start, End: end}})
}

func (a *App) handleSettingsSave(msg dialogs.SettingsSaveMsg) {
	// TODO: apply settings via backend.SaveSettings
	a.statusBar.SetMessage("Settings saved", "info")
}

func (a App) showFuzzyFinder() tea.Cmd {
	var items []dialogs.FuzzyItem
	// Add workspace files if available
	if a.backend.IsWorkspaceOpen() {
		files, err := a.backend.GetWorkspaceFiles()
		if err == nil {
			for _, f := range files {
				badges := ""
				if f.Options.JPath != "" {
					badges += "JPath "
				}
				if f.Options.NoHeaderRow {
					badges += "NoHeader "
				}
				items = append(items, dialogs.FuzzyItem{
					Label:    f.RelativePath,
					FilePath: f.FilePath,
					Badges:   strings.TrimSpace(badges),
				})
			}
		}
	}
	// Add open tabs
	for _, t := range a.tabs {
		items = append(items, dialogs.FuzzyItem{
			Label:    t.fileName,
			FilePath: t.filePath,
		})
	}
	return a.fuzzyFinder.Show(items)
}

func (a App) openWorkspaceCmd(path string) tea.Cmd {
	return func() tea.Msg {
		err := a.backend.OpenWorkspace(path)
		if err != nil {
			return T.StatusMsg{Text: "Open workspace: " + err.Error(), Level: "error"}
		}
		return workspaceOpenedMsg{path: path}
	}
}

type workspaceOpenedMsg struct{ path string }

// --- Dialog routing ---

func (a *App) routeToDialog(msg tea.Msg) tea.Cmd {
	// Route events to the first visible dialog
	if a.annotationDlg.Visible() {
		var cmd tea.Cmd
		a.annotationDlg, cmd = a.annotationDlg.Update(msg)
		return cmd
	}
	if a.fuzzyFinder.Visible() {
		var cmd tea.Cmd
		a.fuzzyFinder, cmd = a.fuzzyFinder.Update(msg)
		return cmd
	}
	if a.cellViewer.Visible() {
		var cmd tea.Cmd
		a.cellViewer, cmd = a.cellViewer.Update(msg)
		return cmd
	}
	if a.confirmDlg.Visible() {
		var cmd tea.Cmd
		a.confirmDlg, cmd = a.confirmDlg.Update(msg)
		return cmd
	}
	if a.shortcutsDlg.Visible() {
		var cmd tea.Cmd
		a.shortcutsDlg, cmd = a.shortcutsDlg.Update(msg)
		return cmd
	}
	if a.syntaxDlg.Visible() {
		var cmd tea.Cmd
		a.syntaxDlg, cmd = a.syntaxDlg.Update(msg)
		return cmd
	}
	if a.aboutDlg.Visible() {
		var cmd tea.Cmd
		a.aboutDlg, cmd = a.aboutDlg.Update(msg)
		return cmd
	}
	if a.settingsDlg.Visible() {
		var cmd tea.Cmd
		a.settingsDlg, cmd = a.settingsDlg.Update(msg)
		return cmd
	}
	if a.licenseDlg.Visible() {
		var cmd tea.Cmd
		a.licenseDlg, cmd = a.licenseDlg.Update(msg)
		return cmd
	}
	if a.fileOptionsDlg.Visible() {
		var cmd tea.Cmd
		a.fileOptionsDlg, cmd = a.fileOptionsDlg.Update(msg)
		return cmd
	}
	return nil
}

// --- Layout ---

func (a *App) propagateResize() {
	gridH := a.gridHeight()
	gridMsg := tea.WindowSizeMsg{Width: a.width, Height: gridH}
	a.grid, _ = a.grid.Update(gridMsg)
	a.tabBar, _ = a.tabBar.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	a.searchBar, _ = a.searchBar.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	a.histogram, _ = a.histogram.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	a.console, _ = a.console.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	a.statusBar, _ = a.statusBar.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	a.dashboard, _ = a.dashboard.Update(tea.WindowSizeMsg{Width: a.width, Height: gridH})
}

func (a App) gridHeight() int {
	h := a.height
	h -= 1 // status bar
	h -= a.tabBar.ViewHeight()
	h -= a.searchBar.ViewHeight()
	h -= a.histogram.ViewHeight()
	h -= a.console.ViewHeight()
	if h < 1 {
		h = 1
	}
	return h
}

// --- View ---

func (a App) View() string {
	if a.quitting {
		return ""
	}
	if !a.ready {
		return "Initialising…"
	}

	// Check for dialog overlay
	if overlay := a.dialogOverlay(); overlay != "" {
		return a.renderWithOverlay(overlay)
	}

	var sections []string

	// Tab bar
	if tb := a.tabBar.View(); tb != "" {
		sections = append(sections, tb)
	}

	// Search bar
	if sb := a.searchBar.View(); sb != "" {
		sections = append(sections, sb)
	}

	// Main content
	if len(a.tabs) == 0 && a.focus == focusDashboard {
		sections = append(sections, a.dashboard.View())
	} else {
		sections = append(sections, a.grid.View())
	}

	// Histogram
	if hv := a.histogram.View(); hv != "" {
		sections = append(sections, hv)
	}

	// Console
	if cv := a.console.View(); cv != "" {
		sections = append(sections, cv)
	}

	// Status bar
	sections = append(sections, a.statusBar.View())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (a App) dialogOverlay() string {
	if a.annotationDlg.Visible() {
		return a.annotationDlg.View()
	}
	if a.fuzzyFinder.Visible() {
		return a.fuzzyFinder.View()
	}
	if a.cellViewer.Visible() {
		return a.cellViewer.View()
	}
	if a.confirmDlg.Visible() {
		return a.confirmDlg.View()
	}
	if a.shortcutsDlg.Visible() {
		return a.shortcutsDlg.View()
	}
	if a.syntaxDlg.Visible() {
		return a.syntaxDlg.View()
	}
	if a.aboutDlg.Visible() {
		return a.aboutDlg.View()
	}
	if a.settingsDlg.Visible() {
		return a.settingsDlg.View()
	}
	if a.licenseDlg.Visible() {
		return a.licenseDlg.View()
	}
	if a.fileOptionsDlg.Visible() {
		return a.fileOptionsDlg.View()
	}
	return ""
}

func (a App) renderWithOverlay(overlay string) string {
	// Center the dialog overlay on the screen
	dialogW := lipgloss.Width(overlay)
	dialogH := lipgloss.Height(overlay)

	padLeft := (a.width - dialogW) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	padTop := (a.height - dialogH) / 2
	if padTop < 0 {
		padTop = 0
	}

	var b strings.Builder
	for range padTop {
		b.WriteString(strings.Repeat(" ", a.width) + "\n")
	}
	for _, line := range strings.Split(overlay, "\n") {
		b.WriteString(strings.Repeat(" ", padLeft) + line + "\n")
	}
	return b.String()
}
