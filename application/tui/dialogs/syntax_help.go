package dialogs

import (
	"strings"

	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SyntaxHelpCloseMsg is sent when the syntax help dialog is closed.
type SyntaxHelpCloseMsg struct{}

// SyntaxHelp displays the query syntax reference.
type SyntaxHelp struct {
	viewport viewport.Model
	visible  bool
	theme    T.Theme
}

// NewSyntaxHelp creates a syntax help dialog.
func NewSyntaxHelp(theme T.Theme) SyntaxHelp {
	vp := viewport.New(60, 20)
	return SyntaxHelp{viewport: vp, theme: theme}
}

// Show opens the syntax help dialog.
func (s *SyntaxHelp) Show(width, height int) {
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
	s.viewport.SetContent(syntaxContent(s.theme))
}

// Hide closes the dialog.
func (s *SyntaxHelp) Hide() { s.visible = false }

// Visible returns visibility.
func (s SyntaxHelp) Visible() bool { return s.visible }

// Init is a no-op.
func (s SyntaxHelp) Init() tea.Cmd { return nil }

// Update handles scrolling and close.
func (s SyntaxHelp) Update(msg tea.Msg) (SyntaxHelp, tea.Cmd) {
	if !s.visible {
		return s, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "q":
			s.Hide()
			return s, func() tea.Msg { return SyntaxHelpCloseMsg{} }
		}
	}
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View renders the dialog.
func (s SyntaxHelp) View() string {
	if !s.visible {
		return ""
	}
	var b strings.Builder
	b.WriteString(s.theme.DialogTitle.Render("Query Syntax Reference"))
	b.WriteString("\n\n")
	b.WriteString(s.viewport.View())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(s.theme.FgDim).Render("j/k: scroll  q/Esc: close"))
	return s.theme.DialogBox.Render(b.String())
}

func syntaxContent(theme T.Theme) string {
	k := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	d := lipgloss.NewStyle().Foreground(theme.Fg)
	h := lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)

	sections := []string{
		h.Render("BASIC SEARCH"),
		k.Render("  term") + "           " + d.Render("Search all columns for 'term'"),
		k.Render("  field=value") + "    " + d.Render("Match exact value in column"),
		k.Render("  field!=value") + "   " + d.Render("Exclude rows where field equals value"),
		k.Render("  field>value") + "    " + d.Render("Numeric greater-than"),
		k.Render("  field<value") + "    " + d.Render("Numeric less-than"),
		"",
		h.Render("WILDCARDS & REGEX"),
		k.Render("  field=*pattern*") + d.Render("  Wildcard matching"),
		k.Render("  field=/regex/") + "  " + d.Render("Regular expression matching"),
		"",
		h.Render("BOOLEAN OPERATORS"),
		k.Render("  term1 term2") + "    " + d.Render("AND (implicit)"),
		k.Render("  term1 OR term2") + " " + d.Render("OR"),
		k.Render("  NOT term") + "       " + d.Render("Negation"),
		"",
		h.Render("PIPELINE STAGES"),
		k.Render("  | columns a,b,c") + "  " + d.Render("Show only specified columns"),
		k.Render("  | dedup field") + "     " + d.Render("Remove duplicate values"),
		k.Render("  | limit N") + "         " + d.Render("Show first N results"),
		k.Render("  | sort field") + "      " + d.Render("Sort by field"),
		"",
		h.Render("TIME FILTERS"),
		k.Render("  | after 1h") + "      " + d.Render("Events in the last hour"),
		k.Render("  | before 30m") + "    " + d.Render("Events before 30 minutes ago"),
		k.Render("  | after 2024-01-01") + d.Render("  Events after date"),
		"",
		h.Render("EXAMPLES"),
		d.Render("  status=500"),
		d.Render("  status=500 | after 1h"),
		d.Render("  src_ip=10.* | columns src_ip,dst_ip,port"),
		d.Render("  error OR warning | dedup message | limit 50"),
	}

	return strings.Join(sections, "\n")
}
