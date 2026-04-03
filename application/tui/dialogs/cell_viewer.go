package dialogs

import (
	"strings"

	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CellViewerCloseMsg is sent when the cell viewer is closed.
type CellViewerCloseMsg struct{}

// CellViewer displays the full content of a grid cell in a scrollable viewport.
type CellViewer struct {
	viewport viewport.Model
	content  string
	column   string
	visible  bool
	width    int
	height   int
	theme    T.Theme
}

// NewCellViewer creates a cell viewer dialog.
func NewCellViewer(theme T.Theme) CellViewer {
	vp := viewport.New(60, 20)
	return CellViewer{viewport: vp, theme: theme}
}

// Show opens the cell viewer with the given content.
func (c *CellViewer) Show(column, content string, width, height int) {
	c.visible = true
	c.column = column
	c.content = content
	c.width = width
	c.height = height

	viewW := width * 3 / 4
	viewH := height * 3 / 4
	if viewW < 40 {
		viewW = 40
	}
	if viewH < 10 {
		viewH = 10
	}
	c.viewport.Width = viewW - 4 // border padding
	c.viewport.Height = viewH - 4

	// Try JSON pretty-print
	formatted := tryPrettyJSON(content)
	c.viewport.SetContent(formatted)
}

// Hide closes the dialog.
func (c *CellViewer) Hide() { c.visible = false }

// Visible returns whether the dialog is shown.
func (c CellViewer) Visible() bool { return c.visible }

// Init is a no-op.
func (c CellViewer) Init() tea.Cmd { return nil }

// Update handles scrolling and close.
func (c CellViewer) Update(msg tea.Msg) (CellViewer, tea.Cmd) {
	if !c.visible {
		return c, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "enter":
			c.Hide()
			return c, func() tea.Msg { return CellViewerCloseMsg{} }
		}
	}
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	return c, cmd
}

// View renders the cell viewer.
func (c CellViewer) View() string {
	if !c.visible {
		return ""
	}

	var b strings.Builder
	title := c.theme.DialogTitle.Render("Cell: " + c.column)
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(c.viewport.View())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(c.theme.FgDim).Render("j/k: scroll  q/Esc: close"))

	return c.theme.DialogBox.Render(b.String())
}

// tryPrettyJSON attempts to format JSON; returns original if not valid JSON.
func tryPrettyJSON(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}
	// Simple check: if it starts with { or [, try indenting
	if s[0] == '{' || s[0] == '[' {
		return indentJSON(s)
	}
	return s
}

// indentJSON does a simple manual JSON indentation.
func indentJSON(s string) string {
	var b strings.Builder
	indent := 0
	inString := false
	escape := false

	for _, ch := range s {
		if escape {
			b.WriteRune(ch)
			escape = false
			continue
		}
		if ch == '\\' && inString {
			b.WriteRune(ch)
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			b.WriteRune(ch)
			continue
		}
		if inString {
			b.WriteRune(ch)
			continue
		}
		switch ch {
		case '{', '[':
			b.WriteRune(ch)
			indent++
			b.WriteString("\n" + strings.Repeat("  ", indent))
		case '}', ']':
			indent--
			b.WriteString("\n" + strings.Repeat("  ", indent))
			b.WriteRune(ch)
		case ',':
			b.WriteRune(ch)
			b.WriteString("\n" + strings.Repeat("  ", indent))
		case ':':
			b.WriteString(": ")
		case ' ', '\t', '\n', '\r':
			// skip whitespace outside strings
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}
