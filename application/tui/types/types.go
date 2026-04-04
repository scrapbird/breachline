// Package types provides shared type definitions for the TUI layer.
// This exists to break import cycles between tui, tui/views, and tui/adapters.
package types

import (
	"breachline/app/interfaces"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Vim Modes
// ---------------------------------------------------------------------------

// VimMode represents the current editing mode.
type VimMode int

const (
	ModeNormal  VimMode = iota
	ModeInsert          // search bar / text input focused
	ModeVisual          // row selection
	ModeCommand         // : command input
)

// String returns a display label for the mode.
func (m VimMode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	case ModeCommand:
		return "COMMAND"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Key Bindings
// ---------------------------------------------------------------------------

// KeyMap defines global vim-style key bindings.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	HalfPgUp  key.Binding
	HalfPgDn  key.Binding
	GotoTop   key.Binding
	GotoEnd   key.Binding
	Search    key.Binding
	Command   key.Binding
	Escape    key.Binding
	Enter     key.Binding
	NextMatch key.Binding
	PrevMatch key.Binding
	NextTab   key.Binding
	PrevTab   key.Binding
	CloseTab  key.Binding
	VisualMode key.Binding
	Yank       key.Binding
	Annotate   key.Binding
	ToggleHistogram key.Binding
	ToggleConsole   key.Binding
	FuzzyFinder     key.Binding
	Quit key.Binding
	Help key.Binding
}

// DefaultKeyMap returns the standard vim-style key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "right"),
		),
		HalfPgUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "half page up"),
		),
		HalfPgDn: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "half page down"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "go to top"),
		),
		GotoEnd: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "command"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		NextMatch: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		PrevMatch: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("gt/tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("gT/shift+tab", "prev tab"),
		),
		CloseTab: key.NewBinding(
			key.WithKeys("ctrl+w"),
			key.WithHelp("ctrl+w", "close tab"),
		),
		VisualMode: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "visual select"),
		),
		Yank: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yank/copy"),
		),
		Annotate: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "annotate"),
		),
		ToggleHistogram: key.NewBinding(
			key.WithKeys("ctrl+h"),
			key.WithHelp("ctrl+h", "histogram"),
		),
		ToggleConsole: key.NewBinding(
			key.WithKeys("ctrl+`"),
			key.WithHelp("ctrl+`", "console"),
		),
		FuzzyFinder: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "fuzzy finder"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

// PendingKey tracks multi-key sequences (e.g., gg, gt, gT).
type PendingKey struct {
	Key   string
	Count int
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// FileOpenedMsg is sent when a file has been successfully opened.
type FileOpenedMsg struct {
	TabID    string
	FilePath string
	FileName string
	FileHash string
	Headers  []string
}

// FileErrorMsg is sent when a file operation fails.
type FileErrorMsg struct {
	FilePath string
	Err      error
}

// RowsLoadedMsg carries a page of rows from the backend.
type RowsLoadedMsg struct {
	TabID            string
	Header           []string
	Rows             [][]string
	StartRow         int
	Total            int
	ReachedEnd       bool
	Annotations      []bool
	AnnotationColors []string
}

// RowsErrorMsg is sent when a data fetch fails.
type RowsErrorMsg struct {
	TabID string
	Err   error
}

// TabSwitchedMsg notifies the TUI that a tab switch has occurred.
type TabSwitchedMsg struct {
	TabID string
}

// TabClosedMsg notifies that a tab was closed.
type TabClosedMsg struct {
	TabID string
}

// QueryExecutedMsg signals that a query completed.
type QueryExecutedMsg struct {
	TabID  string
	Query  string
	Header []string
	Rows   [][]string
	Total  int
}

// QueryErrorMsg signals a failed query.
type QueryErrorMsg struct {
	TabID string
	Query string
	Err   error
}

// BackendEventMsg wraps an event emitted by the backend via RuntimeAdapter.
type BackendEventMsg struct {
	Name string
	Data []interface{}
}

// StatusMsg displays a temporary message in the status bar.
type StatusMsg struct {
	Text  string
	Level string // "info", "warn", "error"
}

// SettingsChangedMsg notifies that settings have been updated.
type SettingsChangedMsg struct {
	Settings *interfaces.Settings
}

// ModeChangedMsg notifies that the vim mode changed.
type ModeChangedMsg struct {
	Mode VimMode
}

// ---------------------------------------------------------------------------
// Theme
// ---------------------------------------------------------------------------

// Theme holds all the Lipgloss styles used across the TUI.
type Theme struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Success   lipgloss.Color
	Muted     lipgloss.Color
	BgDark    lipgloss.Color
	BgMedium  lipgloss.Color
	BgLight   lipgloss.Color
	Fg        lipgloss.Color
	FgDim     lipgloss.Color

	TabBar       lipgloss.Style
	TabActive    lipgloss.Style
	TabInactive  lipgloss.Style
	StatusBar    lipgloss.Style
	StatusMode   lipgloss.Style
	StatusInfo   lipgloss.Style
	SearchBar    lipgloss.Style
	SearchInput  lipgloss.Style
	GridHeader   lipgloss.Style
	GridRow      lipgloss.Style
	GridRowAlt   lipgloss.Style
	GridSelected lipgloss.Style
	GridCursor   lipgloss.Style
	GridCell     lipgloss.Style
	GridBorder   lipgloss.Style
	Console      lipgloss.Style
	DialogBox    lipgloss.Style
	DialogTitle  lipgloss.Style
	HelpKey      lipgloss.Style
	HelpDesc     lipgloss.Style

	AnnotationGrey   lipgloss.Color
	AnnotationBlue   lipgloss.Color
	AnnotationYellow lipgloss.Color
	AnnotationGreen  lipgloss.Color
	AnnotationOrange lipgloss.Color
	AnnotationRed    lipgloss.Color
}

// AnnotationColor returns the lipgloss.Color for a named annotation colour.
func (t Theme) AnnotationColor(name string) lipgloss.Color {
	switch name {
	case "blue":
		return t.AnnotationBlue
	case "yellow":
		return t.AnnotationYellow
	case "green":
		return t.AnnotationGreen
	case "orange":
		return t.AnnotationOrange
	case "red":
		return t.AnnotationRed
	default:
		return t.AnnotationGrey
	}
}

// DefaultTheme returns the dark theme matching the desktop app aesthetic.
func DefaultTheme() Theme {
	primary := lipgloss.Color("#7C9FE5")
	secondary := lipgloss.Color("#5EEAD4")
	accent := lipgloss.Color("#F59E0B")
	errorC := lipgloss.Color("#EF4444")
	warning := lipgloss.Color("#F59E0B")
	success := lipgloss.Color("#22C55E")
	muted := lipgloss.Color("#6B7280")
	bgDark := lipgloss.Color("#111827")
	bgMedium := lipgloss.Color("#1F2937")
	bgLight := lipgloss.Color("#374151")
	fg := lipgloss.Color("#F9FAFB")
	fgDim := lipgloss.Color("#9CA3AF")

	return Theme{
		Primary: primary, Secondary: secondary, Accent: accent,
		Error: errorC, Warning: warning, Success: success,
		Muted: muted, BgDark: bgDark, BgMedium: bgMedium, BgLight: bgLight,
		Fg: fg, FgDim: fgDim,

		TabBar:      lipgloss.NewStyle().Background(bgDark).Foreground(fg).Padding(0, 1),
		TabActive:   lipgloss.NewStyle().Background(bgMedium).Foreground(primary).Bold(true).Padding(0, 2),
		TabInactive: lipgloss.NewStyle().Background(bgDark).Foreground(fgDim).Padding(0, 2),
		StatusBar:   lipgloss.NewStyle().Background(bgDark).Foreground(fg),
		StatusMode:  lipgloss.NewStyle().Background(primary).Foreground(bgDark).Bold(true).Padding(0, 1),
		StatusInfo:  lipgloss.NewStyle().Background(bgMedium).Foreground(fgDim).Padding(0, 1),
		SearchBar:   lipgloss.NewStyle().Background(bgMedium).Foreground(fg).Padding(0, 1),
		SearchInput: lipgloss.NewStyle().Background(bgMedium).Foreground(fg),
		GridHeader:  lipgloss.NewStyle().Background(bgMedium).Foreground(primary).Bold(true).Padding(0, 1),
		GridRow:     lipgloss.NewStyle().Foreground(fg).Padding(0, 1),
		GridRowAlt:  lipgloss.NewStyle().Background(bgDark).Foreground(fg).Padding(0, 1),
		GridSelected: lipgloss.NewStyle().Background(lipgloss.Color("#2D3A50")).Foreground(fg).Padding(0, 1),
		GridCursor:   lipgloss.NewStyle().Background(primary).Foreground(bgDark).Bold(true).Padding(0, 1),
		GridCell:    lipgloss.NewStyle().Foreground(fg).Padding(0, 1),
		GridBorder:  lipgloss.NewStyle().Foreground(muted),
		Console:     lipgloss.NewStyle().Background(bgDark).Foreground(fgDim).Padding(0, 1),
		DialogBox:   lipgloss.NewStyle().Background(bgMedium).Foreground(fg).Border(lipgloss.RoundedBorder()).BorderForeground(primary).Padding(1, 2),
		DialogTitle: lipgloss.NewStyle().Foreground(primary).Bold(true),
		HelpKey:     lipgloss.NewStyle().Foreground(primary).Bold(true),
		HelpDesc:    lipgloss.NewStyle().Foreground(fgDim),

		AnnotationGrey: lipgloss.Color("#6B7280"), AnnotationBlue: lipgloss.Color("#3B82F6"),
		AnnotationYellow: lipgloss.Color("#F59E0B"), AnnotationGreen: lipgloss.Color("#22C55E"),
		AnnotationOrange: lipgloss.Color("#F97316"), AnnotationRed: lipgloss.Color("#EF4444"),
	}
}
