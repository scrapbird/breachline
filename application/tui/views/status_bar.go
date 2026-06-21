package views

import (
	"fmt"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatusBar displays the vim-style status line at the bottom of the screen.
type StatusBar struct {
	mode      T.VimMode
	fileName  string
	filePath  string
	cursorRow int
	cursorCol int
	totalRows int
	message   string
	msgLevel  string
	width     int
	theme     T.Theme
}

// NewStatusBar creates a new status bar.
func NewStatusBar(theme T.Theme) StatusBar {
	return StatusBar{mode: T.ModeNormal, theme: theme}
}

// Init satisfies the Bubble Tea model interface (no-op).
func (s StatusBar) Init() tea.Cmd { return nil }

// Update handles messages for the status bar.
func (s StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
	case T.ModeChangedMsg:
		s.mode = msg.Mode
	case T.StatusMsg:
		s.message = msg.Text
		s.msgLevel = msg.Level
	case T.FileOpenedMsg:
		s.fileName = msg.FileName
		s.filePath = msg.FilePath
		s.message = ""
	case T.RowsLoadedMsg:
		s.totalRows = msg.Total
		s.message = ""
	}
	return s, nil
}

// SetCursor updates the cursor position display.
func (s *StatusBar) SetCursor(row, col int) {
	s.cursorRow = row
	s.cursorCol = col
}

// SetMessage sets a temporary status message.
func (s *StatusBar) SetMessage(text, level string) {
	s.message = text
	s.msgLevel = level
}

// ClearMessage clears the status message.
func (s *StatusBar) ClearMessage() {
	s.message = ""
	s.msgLevel = ""
}

// View renders the status bar.
func (s StatusBar) View() string {
	if s.width == 0 {
		return ""
	}

	modeStyle := s.theme.StatusMode
	switch s.mode {
	case T.ModeInsert:
		modeStyle = modeStyle.Background(s.theme.Success)
	case T.ModeVisual:
		modeStyle = modeStyle.Background(s.theme.Warning)
	case T.ModeCommand:
		modeStyle = modeStyle.Background(s.theme.Accent)
	}
	modeStr := modeStyle.Render(fmt.Sprintf(" %s ", s.mode))

	fileInfo := ""
	if s.fileName != "" {
		fileInfo = s.theme.StatusInfo.Render(fmt.Sprintf(" %s ", s.fileName))
	}

	posInfo := ""
	if s.totalRows > 0 {
		posInfo = s.theme.StatusInfo.Render(fmt.Sprintf(" %d/%d ", s.cursorRow+1, s.totalRows))
	}

	msgStr := ""
	if s.message != "" {
		msgStyle := s.theme.StatusBar
		switch s.msgLevel {
		case "error":
			msgStyle = msgStyle.Foreground(s.theme.Error)
		case "warn":
			msgStyle = msgStyle.Foreground(s.theme.Warning)
		}
		msgStr = msgStyle.Render(" " + s.message + " ")
	}

	leftWidth := lipgloss.Width(modeStr) + lipgloss.Width(fileInfo) + lipgloss.Width(msgStr)
	rightWidth := lipgloss.Width(posInfo)
	gapWidth := s.width - leftWidth - rightWidth
	if gapWidth < 0 {
		gapWidth = 0
	}
	gap := s.theme.StatusBar.Render(fmt.Sprintf("%*s", gapWidth, ""))

	return lipgloss.JoinHorizontal(lipgloss.Top, modeStr, fileInfo, msgStr, gap, posInfo)
}
