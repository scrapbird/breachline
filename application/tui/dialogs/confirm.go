package dialogs

import (
	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmYesMsg is sent when the user confirms.
type ConfirmYesMsg struct{ Tag string }

// ConfirmNoMsg is sent when the user denies.
type ConfirmNoMsg struct{ Tag string }

// Confirm is a generic yes/no confirmation dialog.
type Confirm struct {
	title   string
	message string
	tag     string // arbitrary tag for the caller to identify the dialog
	yes     bool   // cursor on yes
	visible bool
	theme   T.Theme
}

// NewConfirm creates a confirmation dialog.
func NewConfirm(theme T.Theme) Confirm {
	return Confirm{theme: theme, yes: true}
}

// Show opens the confirmation dialog.
func (c *Confirm) Show(title, message, tag string) {
	c.title = title
	c.message = message
	c.tag = tag
	c.yes = true
	c.visible = true
}

// Hide closes the dialog.
func (c *Confirm) Hide() { c.visible = false }

// Visible returns whether the dialog is shown.
func (c Confirm) Visible() bool { return c.visible }

// Init is a no-op.
func (c Confirm) Init() tea.Cmd { return nil }

// Update handles input.
func (c Confirm) Update(msg tea.Msg) (Confirm, tea.Cmd) {
	if !c.visible {
		return c, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			c.Hide()
			tag := c.tag
			return c, func() tea.Msg { return ConfirmNoMsg{Tag: tag} }
		case "enter":
			c.Hide()
			tag := c.tag
			if c.yes {
				return c, func() tea.Msg { return ConfirmYesMsg{Tag: tag} }
			}
			return c, func() tea.Msg { return ConfirmNoMsg{Tag: tag} }
		case "h", "left", "l", "right", "tab":
			c.yes = !c.yes
		case "y":
			c.Hide()
			tag := c.tag
			return c, func() tea.Msg { return ConfirmYesMsg{Tag: tag} }
		case "n":
			c.Hide()
			tag := c.tag
			return c, func() tea.Msg { return ConfirmNoMsg{Tag: tag} }
		}
	}
	return c, nil
}

// View renders the dialog.
func (c Confirm) View() string {
	if !c.visible {
		return ""
	}

	title := c.theme.DialogTitle.Render(c.title)
	msg := lipgloss.NewStyle().Foreground(c.theme.Fg).Render(c.message)

	yesStyle := lipgloss.NewStyle().Foreground(c.theme.Success).Padding(0, 2)
	noStyle := lipgloss.NewStyle().Foreground(c.theme.Error).Padding(0, 2)
	if c.yes {
		yesStyle = yesStyle.Bold(true).Underline(true)
	} else {
		noStyle = noStyle.Bold(true).Underline(true)
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, yesStyle.Render("[Y]es"), noStyle.Render("[N]o"))

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", msg, "", buttons)
	return c.theme.DialogBox.Render(content)
}
