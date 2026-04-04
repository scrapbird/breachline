package views

import (
	"fmt"
	"strings"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultPageSize = 100 // rows per page
	minColWidth     = 4
	maxColWidth     = 60
	colPadding      = 2 // spaces between columns
)

// Grid displays paginated tabular data with vim navigation.
type Grid struct {
	// Data
	header           []string
	rows             [][]string
	totalRows        int
	reachedEnd       bool
	annotations      []bool
	annotationColors []string

	// Viewport
	cursorRow   int
	cursorCol   int
	scrollRow   int
	scrollCol   int
	colWidths   []int
	visibleRows int
	visibleCols int

	// Selection (visual mode)
	selecting    bool
	selectAnchor int

	// Dimensions
	width  int
	height int

	// Styling
	theme T.Theme
}

// NewGrid creates a new data grid.
func NewGrid(theme T.Theme) Grid {
	return Grid{theme: theme}
}

// Init satisfies the Bubble Tea model interface.
func (g Grid) Init() tea.Cmd { return nil }

// SetData loads a page of data into the grid.
func (g *Grid) SetData(header []string, rows [][]string, totalRows int, reachedEnd bool, annotations []bool, annotationColors []string) {
	g.header = header
	g.rows = rows
	g.totalRows = totalRows
	g.reachedEnd = reachedEnd
	g.annotations = annotations
	g.annotationColors = annotationColors
	g.computeColumnWidths()
}

// CursorRow returns the current cursor row.
func (g Grid) CursorRow() int { return g.cursorRow }

// CursorCol returns the current cursor column.
func (g Grid) CursorCol() int { return g.cursorCol }

// computeColumnWidths samples loaded data to determine column widths.
func (g *Grid) computeColumnWidths() {
	if len(g.header) == 0 {
		g.colWidths = nil
		return
	}
	g.colWidths = make([]int, len(g.header))
	for i, h := range g.header {
		g.colWidths[i] = len(h)
	}
	sampleSize := 100
	if sampleSize > len(g.rows) {
		sampleSize = len(g.rows)
	}
	for _, row := range g.rows[:sampleSize] {
		for i, cell := range row {
			if i < len(g.colWidths) && len(cell) > g.colWidths[i] {
				g.colWidths[i] = len(cell)
			}
		}
	}
	for i := range g.colWidths {
		if g.colWidths[i] < minColWidth {
			g.colWidths[i] = minColWidth
		}
		if g.colWidths[i] > maxColWidth {
			g.colWidths[i] = maxColWidth
		}
	}
}

// calculateVisibleCols determines how many columns fit in the viewport.
func (g *Grid) calculateVisibleCols() {
	if len(g.colWidths) == 0 {
		g.visibleCols = 0
		return
	}
	used := 0
	count := 0
	for i := g.scrollCol; i < len(g.colWidths); i++ {
		needed := g.colWidths[i] + colPadding
		if used+needed > g.width {
			break
		}
		used += needed
		count++
	}
	if count == 0 && len(g.colWidths) > 0 {
		count = 1
	}
	g.visibleCols = count
}

// Update handles messages for the grid.
func (g Grid) Update(msg tea.Msg) (Grid, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.width = msg.Width
		g.height = msg.Height
		g.visibleRows = g.height - 1 // minus header row
		if g.visibleRows < 1 {
			g.visibleRows = 1
		}
		g.calculateVisibleCols()

	case T.RowsLoadedMsg:
		g.SetData(msg.Header, msg.Rows, msg.Total, msg.ReachedEnd, msg.Annotations, msg.AnnotationColors)
	}
	return g, nil
}

// MoveUp moves the cursor up by n rows.
func (g *Grid) MoveUp(n int) {
	g.cursorRow -= n
	if g.cursorRow < 0 {
		g.cursorRow = 0
	}
	g.ensureVisible()
}

// MoveDown moves the cursor down by n rows.
func (g *Grid) MoveDown(n int) {
	g.cursorRow += n
	maxRow := len(g.rows) - 1
	if maxRow < 0 {
		maxRow = 0
	}
	if g.cursorRow > maxRow {
		g.cursorRow = maxRow
	}
	g.ensureVisible()
}

// MoveLeft shifts column view left.
func (g *Grid) MoveLeft(n int) {
	g.cursorCol -= n
	if g.cursorCol < 0 {
		g.cursorCol = 0
	}
	if g.cursorCol < g.scrollCol {
		g.scrollCol = g.cursorCol
	}
	g.calculateVisibleCols()
}

// MoveRight shifts column view right.
func (g *Grid) MoveRight(n int) {
	g.cursorCol += n
	maxCol := len(g.header) - 1
	if maxCol < 0 {
		maxCol = 0
	}
	if g.cursorCol > maxCol {
		g.cursorCol = maxCol
	}
	if g.cursorCol >= g.scrollCol+g.visibleCols {
		g.scrollCol = g.cursorCol - g.visibleCols + 1
		if g.scrollCol < 0 {
			g.scrollCol = 0
		}
	}
	g.calculateVisibleCols()
}

// GotoTop moves cursor to the first row.
func (g *Grid) GotoTop() {
	g.cursorRow = 0
	g.ensureVisible()
}

// GotoBottom moves cursor to the last loaded row.
func (g *Grid) GotoBottom() {
	g.cursorRow = len(g.rows) - 1
	if g.cursorRow < 0 {
		g.cursorRow = 0
	}
	g.ensureVisible()
}

// HalfPageUp moves up by half the viewport height.
func (g *Grid) HalfPageUp() { g.MoveUp(g.visibleRows / 2) }

// HalfPageDown moves down by half the viewport height.
func (g *Grid) HalfPageDown() { g.MoveDown(g.visibleRows / 2) }

// ToggleVisualMode enters or exits visual (selection) mode.
func (g *Grid) ToggleVisualMode() bool {
	if g.selecting {
		g.selecting = false
		return false
	}
	g.selecting = true
	g.selectAnchor = g.cursorRow
	return true
}

// ClearSelection exits visual mode without action.
func (g *Grid) ClearSelection() { g.selecting = false }

// SelectedRange returns the inclusive row range currently selected.
func (g *Grid) SelectedRange() (start, end int) {
	if !g.selecting {
		return g.cursorRow, g.cursorRow
	}
	if g.selectAnchor <= g.cursorRow {
		return g.selectAnchor, g.cursorRow
	}
	return g.cursorRow, g.selectAnchor
}

// HasData returns true if data has been loaded.
func (g Grid) HasData() bool { return len(g.header) > 0 }

// CellAt returns the column name and cell value at the given row and column.
func (g Grid) CellAt(row, col int) (string, string) {
	colName := ""
	if col >= 0 && col < len(g.header) {
		colName = g.header[col]
	}
	cellVal := ""
	if row >= 0 && row < len(g.rows) && col >= 0 && col < len(g.rows[row]) {
		cellVal = g.rows[row][col]
	}
	return colName, cellVal
}

func (g *Grid) ensureVisible() {
	if g.cursorRow < g.scrollRow {
		g.scrollRow = g.cursorRow
	}
	if g.cursorRow >= g.scrollRow+g.visibleRows {
		g.scrollRow = g.cursorRow - g.visibleRows + 1
	}
	if g.scrollRow < 0 {
		g.scrollRow = 0
	}
}

// View renders the grid.
func (g Grid) View() string {
	if g.width == 0 || !g.HasData() {
		return ""
	}

	g.calculateVisibleCols()

	var b strings.Builder

	b.WriteString(g.renderRow(g.header, -1, true))
	b.WriteByte('\n')

	endRow := g.scrollRow + g.visibleRows
	if endRow > len(g.rows) {
		endRow = len(g.rows)
	}
	for i := g.scrollRow; i < endRow; i++ {
		b.WriteString(g.renderRow(g.rows[i], i, false))
		if i < endRow-1 {
			b.WriteByte('\n')
		}
	}

	rendered := endRow - g.scrollRow
	for i := rendered; i < g.visibleRows; i++ {
		b.WriteString(g.theme.GridRow.Render(strings.Repeat(" ", g.width)))
		if i < g.visibleRows-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func (g Grid) renderRow(row []string, rowIdx int, isHeader bool) string {
	var b strings.Builder

	endCol := g.scrollCol + g.visibleCols
	if endCol > len(g.colWidths) {
		endCol = len(g.colWidths)
	}

	for ci := g.scrollCol; ci < endCol; ci++ {
		w := g.colWidths[ci]
		cell := ""
		if ci < len(row) {
			cell = row[ci]
		}
		if len(cell) > w {
			cell = cell[:w-1] + "…"
		}
		cell = fmt.Sprintf("%-*s", w, cell)

		if isHeader {
			b.WriteString(g.theme.GridHeader.Render(cell))
		} else {
			style := g.cellStyle(rowIdx, ci)
			b.WriteString(style.Render(cell))
		}
		if ci < endCol-1 {
			b.WriteString(g.theme.GridBorder.Render("│"))
		}
	}

	used := lipgloss.Width(b.String())
	if used < g.width {
		padding := strings.Repeat(" ", g.width-used)
		if isHeader {
			b.WriteString(g.theme.GridHeader.Render(padding))
		} else {
			b.WriteString(g.theme.GridRow.Render(padding))
		}
	}

	return b.String()
}

func (g Grid) cellStyle(rowIdx, colIdx int) lipgloss.Style {
	// Active cursor cell gets a distinct highlight
	if rowIdx == g.cursorRow && colIdx == g.cursorCol && !g.selecting {
		return g.theme.GridCursor
	}

	if g.selecting {
		start, end := g.SelectedRange()
		if rowIdx >= start && rowIdx <= end {
			return g.theme.GridSelected
		}
	} else if rowIdx == g.cursorRow {
		return g.theme.GridSelected
	}

	if rowIdx < len(g.annotations) && g.annotations[rowIdx] {
		color := ""
		if rowIdx < len(g.annotationColors) {
			color = g.annotationColors[rowIdx]
		}
		annotColor := g.theme.AnnotationColor(color)
		return g.theme.GridRow.Foreground(annotColor)
	}

	if rowIdx%2 == 0 {
		return g.theme.GridRow
	}
	return g.theme.GridRowAlt
}
