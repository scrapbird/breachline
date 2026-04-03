package dialogs

import (
	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LicenseImportMsg is sent when the user submits a license path.
type LicenseImportMsg struct {
	Path string
}

// LicenseCancelMsg is sent when the user cancels.
type LicenseCancelMsg struct{}

// License is a dialog for importing license files.
type License struct {
	input   textinput.Model
	message string
	isError bool
	visible bool
	theme   T.Theme
}

// NewLicense creates a license import dialog.
func NewLicense(theme T.Theme) License {
	ti := textinput.New()
	ti.Prompt = "License file: "
	ti.CharLimit = 512
	ti.Width = 50
	return License{input: ti, theme: theme}
}

// Show opens the license dialog.
func (l *License) Show(initialPath string) tea.Cmd {
	l.visible = true
	l.message = ""
	l.isError = false
	l.input.SetValue(initialPath)
	l.input.Focus()
	return textinput.Blink
}

// ShowMessage shows a message (e.g. license info or import result).
func (l *License) ShowMessage(msg string, isError bool) {
	l.visible = true
	l.message = msg
	l.isError = isError
}

// Hide closes the dialog.
func (l *License) Hide() {
	l.visible = false
	l.input.Blur()
}

// Visible returns visibility.
func (l License) Visible() bool { return l.visible }

// Init is a no-op.
func (l License) Init() tea.Cmd { return nil }

// Update handles input.
func (l License) Update(msg tea.Msg) (License, tea.Cmd) {
	if !l.visible {
		return l, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			l.Hide()
			return l, func() tea.Msg { return LicenseCancelMsg{} }
		case "enter":
			path := l.input.Value()
			if path != "" {
				l.Hide()
				return l, func() tea.Msg { return LicenseImportMsg{Path: path} }
			}
			return l, nil
		}
	}
	var cmd tea.Cmd
	l.input, cmd = l.input.Update(msg)
	return l, cmd
}

// View renders the dialog.
func (l License) View() string {
	if !l.visible {
		return ""
	}

	title := l.theme.DialogTitle.Render("License")
	content := title + "\n\n"

	if l.message != "" {
		style := lipgloss.NewStyle().Foreground(l.theme.Success)
		if l.isError {
			style = style.Foreground(l.theme.Error)
		}
		content += style.Render(l.message) + "\n\n"
	}

	content += l.input.View() + "\n\n"
	content += lipgloss.NewStyle().Foreground(l.theme.FgDim).Render("Enter: import  Esc: cancel")

	return l.theme.DialogBox.Render(content)
}
