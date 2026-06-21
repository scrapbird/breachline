package dialogs

import (
	"strings"

	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ShortcutsCloseMsg is sent when the shortcuts dialog is closed.
type ShortcutsCloseMsg struct{}

// Shortcuts displays a keyboard shortcuts reference.
type Shortcuts struct {
	viewport viewport.Model
	visible  bool
	theme    T.Theme
}

// NewShortcuts creates a shortcuts dialog.
func NewShortcuts(theme T.Theme) Shortcuts {
	vp := viewport.New(60, 20)
	return Shortcuts{viewport: vp, theme: theme}
}

// Show opens the shortcuts dialog.
func (s *Shortcuts) Show(width, height int) {
	s.visible = true
	viewW := width * 3 / 4
	viewH := height * 3 / 4
	if viewW < 50 {
		viewW = 50
	}
	if viewH < 15 {
		viewH = 15
	}
	s.viewport.Width = viewW - 4
	s.viewport.Height = viewH - 6
	s.viewport.SetContent(shortcutsContent(s.theme))
}

// Hide closes the dialog.
func (s *Shortcuts) Hide() { s.visible = false }

// Visible returns visibility.
func (s Shortcuts) Visible() bool { return s.visible }

// Init is a no-op.
func (s Shortcuts) Init() tea.Cmd { return nil }

// Update handles scrolling and close.
func (s Shortcuts) Update(msg tea.Msg) (Shortcuts, tea.Cmd) {
	if !s.visible {
		return s, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "q", "?":
			s.Hide()
			return s, func() tea.Msg { return ShortcutsCloseMsg{} }
		}
	}
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View renders the dialog.
func (s Shortcuts) View() string {
	if !s.visible {
		return ""
	}
	var b strings.Builder
	b.WriteString(s.theme.DialogTitle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")
	b.WriteString(s.viewport.View())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(s.theme.FgDim).Render("j/k: scroll  q/Esc: close"))
	return s.theme.DialogBox.Render(b.String())
}

func shortcutsContent(theme T.Theme) string {
	k := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	d := lipgloss.NewStyle().Foreground(theme.Fg)

	entries := []struct{ key, desc string }{
		{"h j k l", "Navigate left/down/up/right"},
		{"Ctrl+d / Ctrl+u", "Half page down/up"},
		{"g g", "Go to top"},
		{"G", "Go to bottom"},
		{"", ""},
		{"/ ", "Search / filter query"},
		{": ", "Command mode"},
		{"n / N", "Next / previous match"},
		{"Esc", "Return to normal mode"},
		{"", ""},
		{"v", "Visual select mode"},
		{"y", "Yank (copy) selected rows"},
		{"a", "Annotate selected rows"},
		{"Enter", "View cell content"},
		{"", ""},
		{"gt / gT", "Next / previous tab"},
		{"Tab / Shift+Tab", "Next / previous tab"},
		{"Ctrl+w", "Close current tab"},
		{"", ""},
		{"Ctrl+h", "Toggle histogram"},
		{"Ctrl+`", "Toggle console"},
		{"Ctrl+p", "Fuzzy file finder"},
		{"", ""},
		{":e <file>", "Open file"},
		{":q", "Close tab / quit"},
		{":set", "Open settings"},
		{":help", "Show this help"},
		{":syntax", "Query syntax reference"},
		{":about", "About BreachLine"},
		{"", ""},
		{"q", "Close tab (or quit if last tab)"},
		{"?", "Show this help"},
	}

	var lines []string
	for _, e := range entries {
		if e.key == "" {
			lines = append(lines, "")
			continue
		}
		line := k.Render(padRight(e.key, 20)) + "  " + d.Render(e.desc)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
