package dialogs

import (
	"strings"

	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FuzzyItem represents a selectable item in the fuzzy finder.
type FuzzyItem struct {
	Label    string
	FilePath string
	Badges   string // e.g. "JPath NoHeader"
}

// FuzzySelectMsg is sent when the user selects an item.
type FuzzySelectMsg struct {
	FilePath string
}

// FuzzyCancelMsg is sent when the user cancels.
type FuzzyCancelMsg struct{}

// FuzzyFinder is a fuzzy-search file picker dialog.
type FuzzyFinder struct {
	input    textinput.Model
	items    []FuzzyItem
	filtered []int // indices into items
	cursor   int
	visible  bool
	width    int
	height   int
	theme    T.Theme
}

// NewFuzzyFinder creates a fuzzy finder dialog.
func NewFuzzyFinder(theme T.Theme) FuzzyFinder {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.CharLimit = 256
	ti.Width = 60
	return FuzzyFinder{input: ti, theme: theme}
}

// Show opens the fuzzy finder with the given items.
func (f *FuzzyFinder) Show(items []FuzzyItem) tea.Cmd {
	f.visible = true
	f.items = items
	f.input.SetValue("")
	f.cursor = 0
	f.refilter()
	f.input.Focus()
	return textinput.Blink
}

// Hide closes the fuzzy finder.
func (f *FuzzyFinder) Hide() {
	f.visible = false
	f.input.Blur()
}

// Visible returns whether the dialog is shown.
func (f FuzzyFinder) Visible() bool { return f.visible }

// Init is a no-op.
func (f FuzzyFinder) Init() tea.Cmd { return nil }

// Update handles input.
func (f FuzzyFinder) Update(msg tea.Msg) (FuzzyFinder, tea.Cmd) {
	if !f.visible {
		return f, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			f.Hide()
			return f, func() tea.Msg { return FuzzyCancelMsg{} }
		case "enter":
			if f.cursor >= 0 && f.cursor < len(f.filtered) {
				item := f.items[f.filtered[f.cursor]]
				f.Hide()
				return f, func() tea.Msg { return FuzzySelectMsg{FilePath: item.FilePath} }
			}
			return f, nil
		case "up", "ctrl+k":
			f.cursor--
			if f.cursor < 0 {
				f.cursor = 0
			}
			return f, nil
		case "down", "ctrl+j":
			f.cursor++
			if f.cursor >= len(f.filtered) {
				f.cursor = len(f.filtered) - 1
			}
			if f.cursor < 0 {
				f.cursor = 0
			}
			return f, nil
		}
	}

	prevVal := f.input.Value()
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	if f.input.Value() != prevVal {
		f.refilter()
		f.cursor = 0
	}
	return f, cmd
}

func (f *FuzzyFinder) refilter() {
	query := strings.ToLower(f.input.Value())
	f.filtered = nil
	for i, item := range f.items {
		if query == "" || fuzzyMatch(strings.ToLower(item.Label), query) {
			f.filtered = append(f.filtered, i)
		}
	}
}

func fuzzyMatch(s, pattern string) bool {
	pi := 0
	for i := 0; i < len(s) && pi < len(pattern); i++ {
		if s[i] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}

// View renders the fuzzy finder.
func (f FuzzyFinder) View() string {
	if !f.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(f.theme.DialogTitle.Render("Open File"))
	b.WriteString("\n\n")
	b.WriteString(f.input.View())
	b.WriteString("\n\n")

	maxVisible := 10
	if f.height > 0 {
		maxVisible = f.height/2 - 4
		if maxVisible < 3 {
			maxVisible = 3
		}
	}

	start := 0
	if f.cursor >= maxVisible {
		start = f.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(f.filtered) {
		end = len(f.filtered)
	}

	for i := start; i < end; i++ {
		item := f.items[f.filtered[i]]
		style := lipgloss.NewStyle().Foreground(f.theme.Fg)
		if i == f.cursor {
			style = style.Background(lipgloss.Color("#2D3A50")).Bold(true)
		}
		line := item.Label
		if item.Badges != "" {
			line += "  " + lipgloss.NewStyle().Foreground(f.theme.FgDim).Render(item.Badges)
		}
		b.WriteString(style.Render("  "+line) + "\n")
	}

	if len(f.filtered) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(f.theme.FgDim).Render("  No matches"))
	}

	return f.theme.DialogBox.Render(b.String())
}
