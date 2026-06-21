package dialogs

import (
	"fmt"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AboutCloseMsg is sent when the about dialog is closed.
type AboutCloseMsg struct{}

// About shows version and license info.
type About struct {
	version     string
	licensed    bool
	licenseInfo string
	visible     bool
	theme       T.Theme
}

// NewAbout creates an about dialog.
func NewAbout(theme T.Theme, version string) About {
	return About{theme: theme, version: version}
}

// Show opens the about dialog.
func (a *About) Show(licensed bool, licenseInfo string) {
	a.visible = true
	a.licensed = licensed
	a.licenseInfo = licenseInfo
}

// Hide closes the dialog.
func (a *About) Hide() { a.visible = false }

// Visible returns visibility.
func (a About) Visible() bool { return a.visible }

// Init is a no-op.
func (a About) Init() tea.Cmd { return nil }

// Update handles close.
func (a About) Update(msg tea.Msg) (About, tea.Cmd) {
	if !a.visible {
		return a, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "q", "enter":
			a.Hide()
			return a, func() tea.Msg { return AboutCloseMsg{} }
		}
	}
	return a, nil
}

// View renders the dialog.
func (a About) View() string {
	if !a.visible {
		return ""
	}

	title := a.theme.DialogTitle.Render("BreachLine CLI")
	ver := lipgloss.NewStyle().Foreground(a.theme.Fg).Render(fmt.Sprintf("Version: %s", a.version))

	licStatus := lipgloss.NewStyle().Foreground(a.theme.Success).Render("Licensed")
	if !a.licensed {
		licStatus = lipgloss.NewStyle().Foreground(a.theme.Warning).Render("Unlicensed (community edition)")
	}

	info := ""
	if a.licenseInfo != "" {
		info = lipgloss.NewStyle().Foreground(a.theme.FgDim).Render(a.licenseInfo)
	}

	footer := lipgloss.NewStyle().Foreground(a.theme.FgDim).Render("Press Esc to close")

	content := title + "\n\n" + ver + "\n" + licStatus
	if info != "" {
		content += "\n" + info
	}
	content += "\n\n" + footer

	return a.theme.DialogBox.Render(content)
}
