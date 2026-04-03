package dialogs

import (
	"fmt"
	"strings"

	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AnnotationColors defines the available annotation colour names.
var AnnotationColors = []string{"grey", "blue", "yellow", "green", "orange", "red"}

// AnnotationResult holds the result when the annotation dialog is confirmed.
type AnnotationResult struct {
	Note  string
	Color string
}

// AnnotationSubmitMsg is sent when the user confirms the annotation.
type AnnotationSubmitMsg struct {
	Note  string
	Color string
}

// AnnotationCancelMsg is sent when the user cancels.
type AnnotationCancelMsg struct{}

// Annotation is a dialog for creating/editing row annotations.
type Annotation struct {
	input      textinput.Model
	colorIdx   int
	visible    bool
	width      int
	theme      T.Theme
}

// NewAnnotation creates an annotation dialog.
func NewAnnotation(theme T.Theme) Annotation {
	ti := textinput.New()
	ti.Prompt = "Note: "
	ti.CharLimit = 256
	ti.Width = 40
	return Annotation{input: ti, theme: theme}
}

// Show makes the dialog visible and focuses the input.
func (a *Annotation) Show(existingNote, existingColor string) tea.Cmd {
	a.visible = true
	a.input.SetValue(existingNote)
	a.colorIdx = 0
	for i, c := range AnnotationColors {
		if c == existingColor {
			a.colorIdx = i
			break
		}
	}
	a.input.Focus()
	return textinput.Blink
}

// Hide closes the dialog.
func (a *Annotation) Hide() {
	a.visible = false
	a.input.Blur()
}

// Visible returns whether the dialog is shown.
func (a Annotation) Visible() bool { return a.visible }

// Init is a no-op.
func (a Annotation) Init() tea.Cmd { return nil }

// Update handles input.
func (a Annotation) Update(msg tea.Msg) (Annotation, tea.Cmd) {
	if !a.visible {
		return a, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.Hide()
			return a, func() tea.Msg { return AnnotationCancelMsg{} }
		case "enter":
			result := AnnotationSubmitMsg{
				Note:  a.input.Value(),
				Color: AnnotationColors[a.colorIdx],
			}
			a.Hide()
			return a, func() tea.Msg { return result }
		case "tab":
			a.colorIdx = (a.colorIdx + 1) % len(AnnotationColors)
			return a, nil
		case "shift+tab":
			a.colorIdx--
			if a.colorIdx < 0 {
				a.colorIdx = len(AnnotationColors) - 1
			}
			return a, nil
		}
	}
	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

// View renders the annotation dialog.
func (a Annotation) View() string {
	if !a.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(a.theme.DialogTitle.Render("Annotate Rows"))
	b.WriteString("\n\n")
	b.WriteString(a.input.View())
	b.WriteString("\n\n")

	// Colour picker
	b.WriteString("Color: ")
	for i, c := range AnnotationColors {
		style := lipgloss.NewStyle().Foreground(a.theme.AnnotationColor(c))
		label := fmt.Sprintf(" %s ", c)
		if i == a.colorIdx {
			style = style.Bold(true).Underline(true)
		}
		b.WriteString(style.Render(label))
	}
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(a.theme.FgDim).Render("Tab: cycle colour  Enter: save  Esc: cancel"))

	return a.theme.DialogBox.Render(b.String())
}
