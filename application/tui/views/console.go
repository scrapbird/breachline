package views

import (
	"fmt"
	"strings"
	"time"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogEntry represents a single console log line.
type LogEntry struct {
	Time    time.Time
	Level   string // "info", "warn", "error", "debug"
	Message string
}

// Console is a toggleable log panel at the bottom of the screen.
type Console struct {
	entries  []LogEntry
	visible  bool
	height   int // display height (rows)
	scroll   int // scroll offset from bottom
	maxLines int
	width    int
	theme    T.Theme
}

// NewConsole creates a console panel.
func NewConsole(theme T.Theme) Console {
	return Console{theme: theme, height: 6, maxLines: 500}
}

// Toggle shows/hides the console.
func (c *Console) Toggle() { c.visible = !c.visible }

// Visible returns visibility state.
func (c Console) Visible() bool { return c.visible }

// AddEntry appends a log entry.
func (c *Console) AddEntry(level, message string) {
	c.entries = append(c.entries, LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	})
	if len(c.entries) > c.maxLines {
		c.entries = c.entries[len(c.entries)-c.maxLines:]
	}
	c.scroll = 0 // auto-scroll to bottom
}

// Clear removes all entries.
func (c *Console) Clear() {
	c.entries = nil
	c.scroll = 0
}

// Init is a no-op.
func (c Console) Init() tea.Cmd { return nil }

// Update handles resize.
func (c Console) Update(msg tea.Msg) (Console, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		c.width = wsm.Width
	}
	return c, nil
}

// ViewHeight returns the rendered height (0 if hidden).
func (c Console) ViewHeight() int {
	if !c.visible {
		return 0
	}
	return c.height + 1 // entries + title bar
}

// View renders the console panel.
func (c Console) View() string {
	if !c.visible || c.width == 0 {
		return ""
	}

	var b strings.Builder

	// Title bar
	title := c.theme.StatusInfo.Width(c.width).Render(" Console ")
	b.WriteString(title)
	b.WriteString("\n")

	// Visible entries
	start := len(c.entries) - c.height - c.scroll
	if start < 0 {
		start = 0
	}
	end := start + c.height
	if end > len(c.entries) {
		end = len(c.entries)
	}

	for i := start; i < end; i++ {
		e := c.entries[i]
		ts := e.Time.Format("15:04:05")
		levelStyle := lipgloss.NewStyle()
		switch e.Level {
		case "error":
			levelStyle = levelStyle.Foreground(c.theme.Error)
		case "warn":
			levelStyle = levelStyle.Foreground(c.theme.Warning)
		case "debug":
			levelStyle = levelStyle.Foreground(c.theme.Muted)
		default:
			levelStyle = levelStyle.Foreground(c.theme.Fg)
		}
		line := fmt.Sprintf(" %s %s %s",
			lipgloss.NewStyle().Foreground(c.theme.FgDim).Render(ts),
			levelStyle.Render(fmt.Sprintf("%-5s", e.Level)),
			e.Message,
		)
		// Truncate to width
		if lipgloss.Width(line) > c.width {
			line = line[:c.width-1] + "…"
		}
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Pad remaining lines
	rendered := end - start
	for i := rendered; i < c.height; i++ {
		b.WriteString("\n" + c.theme.Console.Width(c.width).Render(""))
	}

	return b.String()
}
