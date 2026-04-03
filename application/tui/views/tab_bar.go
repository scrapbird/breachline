package views

import (
	"fmt"
	"strings"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TabInfo holds display data for a single tab.
type TabInfo struct {
	ID       string
	FileName string
	FileType string // "csv", "json", "xlsx"
}

// TabBar renders a horizontal tab strip.
type TabBar struct {
	tabs      []TabInfo
	activeIdx int
	width     int
	theme     T.Theme
}

// NewTabBar creates a tab bar.
func NewTabBar(theme T.Theme) TabBar {
	return TabBar{theme: theme}
}

// SetTabs replaces the tab list.
func (t *TabBar) SetTabs(tabs []TabInfo, activeIdx int) {
	t.tabs = tabs
	t.activeIdx = activeIdx
}

// Init is a no-op.
func (t TabBar) Init() tea.Cmd { return nil }

// Update handles window resize.
func (t TabBar) Update(msg tea.Msg) (TabBar, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		t.width = wsm.Width
	}
	return t, nil
}

// View renders the tab bar.
func (t TabBar) View() string {
	if len(t.tabs) <= 1 {
		return ""
	}

	var parts []string
	for i, tab := range t.tabs {
		label := tab.FileName
		if label == "" {
			label = fmt.Sprintf("Tab %d", i+1)
		}
		// Add file type indicator
		if tab.FileType != "" {
			label = fmt.Sprintf("[%s] %s", tab.FileType, label)
		}
		if i == t.activeIdx {
			parts = append(parts, t.theme.TabActive.Render(label))
		} else {
			parts = append(parts, t.theme.TabInactive.Render(label))
		}
	}

	bar := strings.Join(parts, t.theme.GridBorder.Render("│"))
	w := lipgloss.Width(bar)
	if w < t.width {
		bar += t.theme.TabBar.Render(strings.Repeat(" ", t.width-w))
	}
	return bar
}

// ViewHeight returns 0 if <=1 tab, 1 otherwise.
func (t TabBar) ViewHeight() int {
	if len(t.tabs) <= 1 {
		return 0
	}
	return 1
}
