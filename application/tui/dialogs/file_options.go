package dialogs

import (
	"fmt"

	T "breachline/tui/types"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileOptionsSubmitMsg is sent when the user confirms file open options.
type FileOptionsSubmitMsg struct {
	FilePath  string
	JPath     string
	NoHeader  bool
	IngestTZ  string
}

// FileOptionsCancelMsg is sent when cancelled.
type FileOptionsCancelMsg struct{}

type fileOptionsField int

const (
	foFieldPath fileOptionsField = iota
	foFieldJPath
	foFieldNoHeader
	foFieldIngestTZ
	foFieldCount
)

// FileOptions is a dialog for specifying file open options.
type FileOptions struct {
	pathInput    textinput.Model
	jpathInput   textinput.Model
	ingestInput  textinput.Model
	noHeader     bool
	cursor       fileOptionsField
	visible      bool
	theme        T.Theme
}

// NewFileOptions creates a file options dialog.
func NewFileOptions(theme T.Theme) FileOptions {
	pi := textinput.New()
	pi.Prompt = ""
	pi.CharLimit = 512
	pi.Width = 50

	ji := textinput.New()
	ji.Prompt = ""
	ji.CharLimit = 256
	ji.Width = 50

	ii := textinput.New()
	ii.Prompt = ""
	ii.CharLimit = 64
	ii.Width = 30

	return FileOptions{
		pathInput:   pi,
		jpathInput:  ji,
		ingestInput: ii,
		theme:       theme,
	}
}

// Show opens the file options dialog.
func (f *FileOptions) Show(filePath string) tea.Cmd {
	f.visible = true
	f.pathInput.SetValue(filePath)
	f.jpathInput.SetValue("")
	f.ingestInput.SetValue("")
	f.noHeader = false
	f.cursor = foFieldPath
	f.pathInput.Focus()
	return textinput.Blink
}

// Hide closes the dialog.
func (f *FileOptions) Hide() {
	f.visible = false
	f.pathInput.Blur()
	f.jpathInput.Blur()
	f.ingestInput.Blur()
}

// Visible returns visibility.
func (f FileOptions) Visible() bool { return f.visible }

// Init is a no-op.
func (f FileOptions) Init() tea.Cmd { return nil }

// Update handles input.
func (f FileOptions) Update(msg tea.Msg) (FileOptions, tea.Cmd) {
	if !f.visible {
		return f, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			f.Hide()
			return f, func() tea.Msg { return FileOptionsCancelMsg{} }
		case "enter":
			f.Hide()
			return f, func() tea.Msg {
				return FileOptionsSubmitMsg{
					FilePath: f.pathInput.Value(),
					JPath:    f.jpathInput.Value(),
					NoHeader: f.noHeader,
					IngestTZ: f.ingestInput.Value(),
				}
			}
		case "tab":
			f.cursor++
			if f.cursor >= foFieldCount {
				f.cursor = 0
			}
			f.focusCurrent()
			return f, nil
		case "shift+tab":
			f.cursor--
			if f.cursor < 0 {
				f.cursor = foFieldCount - 1
			}
			f.focusCurrent()
			return f, nil
		case " ":
			if f.cursor == foFieldNoHeader {
				f.noHeader = !f.noHeader
				return f, nil
			}
		}
	}

	var cmd tea.Cmd
	switch f.cursor {
	case foFieldPath:
		f.pathInput, cmd = f.pathInput.Update(msg)
	case foFieldJPath:
		f.jpathInput, cmd = f.jpathInput.Update(msg)
	case foFieldIngestTZ:
		f.ingestInput, cmd = f.ingestInput.Update(msg)
	}
	return f, cmd
}

func (f *FileOptions) focusCurrent() {
	f.pathInput.Blur()
	f.jpathInput.Blur()
	f.ingestInput.Blur()
	switch f.cursor {
	case foFieldPath:
		f.pathInput.Focus()
	case foFieldJPath:
		f.jpathInput.Focus()
	case foFieldIngestTZ:
		f.ingestInput.Focus()
	}
}

// View renders the dialog.
func (f FileOptions) View() string {
	if !f.visible {
		return ""
	}

	title := f.theme.DialogTitle.Render("Open File with Options")

	fields := []struct {
		label  string
		input  string
		toggle bool
		on     bool
	}{
		{"File Path", f.pathInput.View(), false, false},
		{"JSONPath", f.jpathInput.View(), false, false},
		{"No Header Row", "", true, f.noHeader},
		{"Ingest Timezone", f.ingestInput.View(), false, false},
	}

	content := title + "\n\n"
	for i, fld := range fields {
		cursor := "  "
		if fileOptionsField(i) == f.cursor {
			cursor = "▸ "
		}
		label := lipgloss.NewStyle().Foreground(f.theme.Fg).Render(fmt.Sprintf("%s%-18s", cursor, fld.label))
		if fld.toggle {
			check := "[ ]"
			if fld.on {
				check = "[x]"
			}
			content += label + " " + check + "\n"
		} else {
			content += label + " " + fld.input + "\n"
		}
	}
	content += "\n" + lipgloss.NewStyle().Foreground(f.theme.FgDim).Render("Tab: next field  Space: toggle  Enter: open  Esc: cancel")

	return f.theme.DialogBox.Render(content)
}
