package dialogs

import (
	"fmt"
	"strings"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SettingsSaveMsg is sent when settings are saved.
type SettingsSaveMsg struct {
	DisplayTimezone        string
	DefaultIngestTimezone  string
	TimestampDisplayFormat string
	SortByTime             bool
	SortDescending         bool
	EnableQueryCache       bool
	EnablePlugins          bool
}

// SettingsCancelMsg is sent when settings are cancelled.
type SettingsCancelMsg struct{}

// settingsField identifies a form field.
type settingsField int

const (
	fieldDisplayTZ settingsField = iota
	fieldIngestTZ
	fieldTimestampFmt
	fieldSortByTime
	fieldSortDesc
	fieldQueryCache
	fieldPlugins
	fieldCount
)

// Settings is a form-based settings dialog.
type Settings struct {
	// Field values
	displayTZ   string
	ingestTZ    string
	tsFmt       string
	sortByTime  bool
	sortDesc    bool
	queryCache  bool
	plugins     bool

	cursor  settingsField
	visible bool
	width   int
	theme   T.Theme
}

// NewSettings creates a settings dialog.
func NewSettings(theme T.Theme) Settings {
	return Settings{theme: theme}
}

// Show opens the settings dialog with current values.
func (s *Settings) Show(displayTZ, ingestTZ, tsFmt string, sortByTime, sortDesc, cache, plugins bool) {
	s.visible = true
	s.displayTZ = displayTZ
	s.ingestTZ = ingestTZ
	s.tsFmt = tsFmt
	s.sortByTime = sortByTime
	s.sortDesc = sortDesc
	s.queryCache = cache
	s.plugins = plugins
	s.cursor = fieldDisplayTZ
}

// Hide closes the dialog.
func (s *Settings) Hide() { s.visible = false }

// Visible returns visibility.
func (s Settings) Visible() bool { return s.visible }

// Init is a no-op.
func (s Settings) Init() tea.Cmd { return nil }

// Update handles navigation and editing.
func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	if !s.visible {
		return s, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			s.Hide()
			return s, func() tea.Msg { return SettingsCancelMsg{} }
		case "enter":
			s.Hide()
			return s, func() tea.Msg {
				return SettingsSaveMsg{
					DisplayTimezone:        s.displayTZ,
					DefaultIngestTimezone:  s.ingestTZ,
					TimestampDisplayFormat: s.tsFmt,
					SortByTime:             s.sortByTime,
					SortDescending:         s.sortDesc,
					EnableQueryCache:       s.queryCache,
					EnablePlugins:          s.plugins,
				}
			}
		case "j", "down":
			s.cursor++
			if s.cursor >= fieldCount {
				s.cursor = fieldCount - 1
			}
		case "k", "up":
			s.cursor--
			if s.cursor < 0 {
				s.cursor = 0
			}
		case " ", "x":
			s.toggleCurrent()
		}
	}
	return s, nil
}

func (s *Settings) toggleCurrent() {
	switch s.cursor {
	case fieldSortByTime:
		s.sortByTime = !s.sortByTime
	case fieldSortDesc:
		s.sortDesc = !s.sortDesc
	case fieldQueryCache:
		s.queryCache = !s.queryCache
	case fieldPlugins:
		s.plugins = !s.plugins
	}
}

// View renders the settings dialog.
func (s Settings) View() string {
	if !s.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(s.theme.DialogTitle.Render("Settings"))
	b.WriteString("\n\n")

	fields := []struct {
		label string
		value string
		toggle bool
		on     bool
	}{
		{"Display Timezone", s.displayTZ, false, false},
		{"Ingest Timezone", s.ingestTZ, false, false},
		{"Timestamp Format", s.tsFmt, false, false},
		{"Sort by Time", "", true, s.sortByTime},
		{"Sort Descending", "", true, s.sortDesc},
		{"Query Cache", "", true, s.queryCache},
		{"Enable Plugins", "", true, s.plugins},
	}

	for i, f := range fields {
		cursor := "  "
		if settingsField(i) == s.cursor {
			cursor = "▸ "
		}

		style := lipgloss.NewStyle().Foreground(s.theme.Fg)
		if settingsField(i) == s.cursor {
			style = style.Bold(true)
		}

		line := cursor + padRight(f.label, 22)
		if f.toggle {
			if f.on {
				line += lipgloss.NewStyle().Foreground(s.theme.Success).Render("[x]")
			} else {
				line += lipgloss.NewStyle().Foreground(s.theme.FgDim).Render("[ ]")
			}
		} else {
			val := f.value
			if val == "" {
				val = "(default)"
			}
			line += lipgloss.NewStyle().Foreground(s.theme.FgDim).Render(val)
		}

		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(s.theme.FgDim).Render(
		fmt.Sprintf("j/k: navigate  Space: toggle  Enter: save  Esc: cancel")))

	return s.theme.DialogBox.Render(b.String())
}
