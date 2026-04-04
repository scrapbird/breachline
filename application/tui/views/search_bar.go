package views

import (
	"fmt"
	"strings"

	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchBar provides query/search input with history.
type SearchBar struct {
	input   textinput.Model
	mode    SearchMode
	history []string
	histIdx int // -1 = current input, 0..len-1 = history
	current string
	errMsg  string
	width   int
	theme   T.Theme
	visible bool
}

// SearchMode distinguishes between / search and : command input.
type SearchMode int

const (
	SearchModeSearch  SearchMode = iota // / search
	SearchModeCommand                   // : command
)

// NewSearchBar creates a search bar.
func NewSearchBar(theme T.Theme) SearchBar {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Fg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(theme.Accent)
	ti.CharLimit = 512
	ti.Width = 80
	return SearchBar{
		input:   ti,
		histIdx: -1,
		theme:   theme,
	}
}

// Init is a no-op.
func (s SearchBar) Init() tea.Cmd { return nil }

// Show makes the search bar visible with the given mode and focuses it.
func (s *SearchBar) Show(mode SearchMode) tea.Cmd {
	s.visible = true
	s.mode = mode
	s.errMsg = ""
	s.histIdx = -1
	s.current = ""
	if mode == SearchModeCommand {
		s.input.Prompt = ":"
	} else {
		s.input.Prompt = "/"
	}
	s.input.PromptStyle = lipgloss.NewStyle().Foreground(s.theme.Primary).Bold(true)
	s.input.TextStyle = lipgloss.NewStyle().Foreground(s.theme.Fg)
	s.input.Cursor.Style = lipgloss.NewStyle().Foreground(s.theme.Accent)
	s.input.SetValue("")
	s.input.Focus()
	return textinput.Blink
}

// Hide makes the search bar invisible and unfocuses it.
func (s *SearchBar) Hide() {
	s.visible = false
	s.input.Blur()
}

// Visible returns whether the bar is visible.
func (s SearchBar) Visible() bool { return s.visible }

// Value returns the current input text.
func (s SearchBar) Value() string { return s.input.Value() }

// Mode returns the current search mode.
func (s SearchBar) Mode() SearchMode { return s.mode }

// SetError shows an error message on the search bar.
func (s *SearchBar) SetError(msg string) { s.errMsg = msg }

// AddHistory appends a query to the history.
func (s *SearchBar) AddHistory(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	// Deduplicate: remove if already in history
	for i, h := range s.history {
		if h == q {
			s.history = append(s.history[:i], s.history[i+1:]...)
			break
		}
	}
	s.history = append(s.history, q)
	if len(s.history) > 100 {
		s.history = s.history[len(s.history)-100:]
	}
}

// Update handles input events while the search bar is focused.
func (s SearchBar) Update(msg tea.Msg) (SearchBar, tea.Cmd) {
	if !s.visible {
		return s, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.input.Width = msg.Width - 4
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			s.navigateHistory(1)
			return s, nil
		case "down":
			s.navigateHistory(-1)
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

func (s *SearchBar) navigateHistory(delta int) {
	if len(s.history) == 0 {
		return
	}
	if s.histIdx == -1 {
		s.current = s.input.Value()
	}
	s.histIdx += delta
	if s.histIdx >= len(s.history) {
		s.histIdx = len(s.history) - 1
	}
	if s.histIdx < -1 {
		s.histIdx = -1
	}
	if s.histIdx == -1 {
		s.input.SetValue(s.current)
	} else {
		idx := len(s.history) - 1 - s.histIdx
		if idx >= 0 && idx < len(s.history) {
			s.input.SetValue(s.history[idx])
		}
	}
}

// View renders the search bar.
func (s SearchBar) View() string {
	if !s.visible {
		return ""
	}
	style := s.theme.SearchBar
	if s.errMsg != "" {
		style = style.BorderForeground(s.theme.Error)
	}
	bar := s.input.View()
	if s.errMsg != "" {
		bar += " " + lipgloss.NewStyle().Foreground(s.theme.Error).Render(s.errMsg)
	}
	return style.Width(s.width).Render(bar)
}

// ViewHeight returns the height of the search bar (0 if hidden).
func (s SearchBar) ViewHeight() int {
	if !s.visible {
		return 0
	}
	return 1
}

// FormatMatchCount returns a formatted match count string for the status bar.
func FormatMatchCount(current, total int) string {
	if total == 0 {
		return "No matches"
	}
	return fmt.Sprintf("Match %d/%d", current+1, total)
}
