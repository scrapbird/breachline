package views

import (
	"fmt"
	"strings"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardFile represents a file entry on the dashboard.
type DashboardFile struct {
	FilePath        string
	FileName        string
	Description     string
	AnnotationCount int
}

// Dashboard shows workspace files when no tabs are open.
type Dashboard struct {
	files     []DashboardFile
	cursor    int
	workspace string
	width     int
	height    int
	theme     T.Theme
}

// NewDashboard creates a dashboard view.
func NewDashboard(theme T.Theme) Dashboard {
	return Dashboard{theme: theme}
}

// SetFiles updates the dashboard file list.
func (d *Dashboard) SetFiles(files []DashboardFile, workspace string) {
	d.files = files
	d.workspace = workspace
	if d.cursor >= len(d.files) {
		d.cursor = len(d.files) - 1
	}
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// SelectedFile returns the currently highlighted file path, or empty.
func (d Dashboard) SelectedFile() string {
	if d.cursor >= 0 && d.cursor < len(d.files) {
		return d.files[d.cursor].FilePath
	}
	return ""
}

// Init is a no-op.
func (d Dashboard) Init() tea.Cmd { return nil }

// Update handles input and resize.
func (d Dashboard) Update(msg tea.Msg) (Dashboard, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
	}
	return d, nil
}

// MoveUp moves cursor up.
func (d *Dashboard) MoveUp() {
	d.cursor--
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// MoveDown moves cursor down.
func (d *Dashboard) MoveDown() {
	d.cursor++
	if d.cursor >= len(d.files) {
		d.cursor = len(d.files) - 1
	}
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// View renders the dashboard.
func (d Dashboard) View() string {
	if d.width == 0 {
		return ""
	}

	var b strings.Builder

	// Title
	title := "BreachLine"
	if d.workspace != "" {
		title += " — " + d.workspace
	}
	titleStyle := d.theme.DialogTitle.Width(d.width).Align(lipgloss.Center)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if len(d.files) == 0 {
		empty := lipgloss.NewStyle().Foreground(d.theme.FgDim).Width(d.width).Align(lipgloss.Center)
		b.WriteString(empty.Render("No files open. Use :e <file> or provide files as arguments."))
		b.WriteString("\n")
		b.WriteString(empty.Render("Press ? for help."))
	} else {
		for i, f := range d.files {
			line := fmt.Sprintf("  %s", f.FileName)
			if f.AnnotationCount > 0 {
				line += fmt.Sprintf("  [%d annotations]", f.AnnotationCount)
			}
			if f.Description != "" {
				line += fmt.Sprintf("  — %s", f.Description)
			}

			style := d.theme.GridRow
			if i == d.cursor {
				style = d.theme.GridSelected
			}
			b.WriteString(style.Width(d.width).Render(line))
			b.WriteString("\n")
		}
	}

	// Pad remaining height
	rendered := strings.Count(b.String(), "\n")
	for i := rendered; i < d.height-1; i++ {
		b.WriteString("\n")
	}

	return b.String()
}
