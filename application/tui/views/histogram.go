package views

import (
	"fmt"
	"math"
	"strings"
	"time"

	T "breachline/tui/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Unicode block characters for histogram bars.
var barChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// HistogramBucket holds data for one time bucket.
type HistogramBucket struct {
	StartTime int64 // milliseconds
	EndTime   int64 // milliseconds
	Count     int
}

// Histogram renders an ASCII time-series histogram.
type Histogram struct {
	buckets    []HistogramBucket
	cursor     int
	selectMode bool
	selectFrom int
	sparkline  bool // compact single-line mode
	visible    bool
	width      int
	height     int // full histogram height (default 8)
	theme      T.Theme
}

// NewHistogram creates a histogram view.
func NewHistogram(theme T.Theme) Histogram {
	return Histogram{theme: theme, height: 8}
}

// SetBuckets updates the histogram data.
func (h *Histogram) SetBuckets(buckets []HistogramBucket) {
	h.buckets = buckets
	if h.cursor >= len(h.buckets) {
		h.cursor = len(h.buckets) - 1
	}
	if h.cursor < 0 {
		h.cursor = 0
	}
}

// Toggle shows/hides the histogram.
func (h *Histogram) Toggle() { h.visible = !h.visible }

// Visible returns visibility state.
func (h Histogram) Visible() bool { return h.visible }

// ToggleSparkline switches between full and compact mode.
func (h *Histogram) ToggleSparkline() { h.sparkline = !h.sparkline }

// MoveLeft moves cursor left.
func (h *Histogram) MoveLeft() {
	h.cursor--
	if h.cursor < 0 {
		h.cursor = 0
	}
}

// MoveRight moves cursor right.
func (h *Histogram) MoveRight() {
	h.cursor++
	if h.cursor >= len(h.buckets) {
		h.cursor = len(h.buckets) - 1
	}
	if h.cursor < 0 {
		h.cursor = 0
	}
}

// ToggleSelect enters/exits range selection.
func (h *Histogram) ToggleSelect() bool {
	if h.selectMode {
		h.selectMode = false
		return false
	}
	h.selectMode = true
	h.selectFrom = h.cursor
	return true
}

// SelectedRange returns the time range of the current selection (milliseconds).
func (h Histogram) SelectedRange() (startMs, endMs int64) {
	if !h.selectMode || len(h.buckets) == 0 {
		if h.cursor >= 0 && h.cursor < len(h.buckets) {
			return h.buckets[h.cursor].StartTime, h.buckets[h.cursor].EndTime
		}
		return 0, 0
	}
	from, to := h.selectFrom, h.cursor
	if from > to {
		from, to = to, from
	}
	if from < 0 {
		from = 0
	}
	if to >= len(h.buckets) {
		to = len(h.buckets) - 1
	}
	return h.buckets[from].StartTime, h.buckets[to].EndTime
}

// Init is a no-op.
func (h Histogram) Init() tea.Cmd { return nil }

// Update handles resize.
func (h Histogram) Update(msg tea.Msg) (Histogram, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		h.width = wsm.Width
	}
	return h, nil
}

// ViewHeight returns the rendered height (0 if hidden).
func (h Histogram) ViewHeight() int {
	if !h.visible || len(h.buckets) == 0 {
		return 0
	}
	if h.sparkline {
		return 1
	}
	return h.height + 2 // bars + label + info line
}

// View renders the histogram.
func (h Histogram) View() string {
	if !h.visible || len(h.buckets) == 0 || h.width == 0 {
		return ""
	}
	if h.sparkline {
		return h.renderSparkline()
	}
	return h.renderFull()
}

func (h Histogram) renderSparkline() string {
	var b strings.Builder
	maxCount := h.maxCount()
	for i, bk := range h.buckets {
		if i >= h.width-2 {
			break
		}
		ch := barChar(bk.Count, maxCount)
		style := lipgloss.NewStyle().Foreground(h.theme.Primary)
		if i == h.cursor {
			style = style.Foreground(h.theme.Accent).Bold(true)
		}
		b.WriteString(style.Render(string(ch)))
	}
	return b.String()
}

func (h Histogram) renderFull() string {
	maxCount := h.maxCount()
	if maxCount == 0 {
		maxCount = 1
	}

	// Available width for bars
	barWidth := h.width - 8 // leave space for Y-axis labels
	if barWidth < 1 {
		barWidth = 1
	}

	// Determine how many buckets fit
	bucketCount := len(h.buckets)
	if bucketCount > barWidth {
		bucketCount = barWidth
	}

	var lines []string

	// Render bars top-down (row 0 = top)
	for row := h.height - 1; row >= 0; row-- {
		threshold := float64(row+1) / float64(h.height) * float64(maxCount)
		var rowBuf strings.Builder

		// Y-axis label
		if row == h.height-1 {
			rowBuf.WriteString(fmt.Sprintf("%6d ", maxCount))
		} else if row == 0 {
			rowBuf.WriteString(fmt.Sprintf("%6d ", 0))
		} else {
			rowBuf.WriteString("       ")
		}

		for i := 0; i < bucketCount; i++ {
			bk := h.buckets[i]
			ch := ' '
			if float64(bk.Count) >= threshold {
				ch = '█'
			} else if float64(bk.Count) >= threshold-float64(maxCount)/float64(h.height) {
				frac := (float64(bk.Count) - (threshold - float64(maxCount)/float64(h.height))) / (float64(maxCount) / float64(h.height))
				idx := int(frac * float64(len(barChars)-1))
				if idx < 0 {
					idx = 0
				}
				if idx >= len(barChars) {
					idx = len(barChars) - 1
				}
				ch = barChars[idx]
			}

			style := lipgloss.NewStyle().Foreground(h.theme.Primary)
			if h.selectMode {
				from, to := h.selectFrom, h.cursor
				if from > to {
					from, to = to, from
				}
				if i >= from && i <= to {
					style = style.Foreground(h.theme.Accent)
				}
			} else if i == h.cursor {
				style = style.Foreground(h.theme.Accent).Bold(true)
			}
			rowBuf.WriteString(style.Render(string(ch)))
		}
		lines = append(lines, rowBuf.String())
	}

	// X-axis time labels
	if bucketCount > 0 {
		labelLine := "       "
		startLabel := formatTime(h.buckets[0].StartTime)
		endLabel := ""
		if bucketCount > 1 {
			endLabel = formatTime(h.buckets[bucketCount-1].EndTime)
		}
		gap := bucketCount - len(startLabel) - len(endLabel)
		if gap < 1 {
			gap = 1
		}
		labelLine += startLabel + strings.Repeat(" ", gap) + endLabel
		lines = append(lines, lipgloss.NewStyle().Foreground(h.theme.FgDim).Render(labelLine))
	}

	// Info line: cursor bucket details
	if h.cursor >= 0 && h.cursor < len(h.buckets) {
		bk := h.buckets[h.cursor]
		info := fmt.Sprintf("  %s → %s  count: %d",
			formatTime(bk.StartTime), formatTime(bk.EndTime), bk.Count)
		lines = append(lines, lipgloss.NewStyle().Foreground(h.theme.FgDim).Render(info))
	}

	return strings.Join(lines, "\n")
}

func (h Histogram) maxCount() int {
	max := 0
	for _, b := range h.buckets {
		if b.Count > max {
			max = b.Count
		}
	}
	return max
}

func barChar(count, maxCount int) rune {
	if maxCount == 0 || count == 0 {
		return ' '
	}
	frac := float64(count) / float64(maxCount)
	idx := int(math.Round(frac * float64(len(barChars)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(barChars) {
		idx = len(barChars) - 1
	}
	return barChars[idx]
}

func formatTime(ms int64) string {
	if ms == 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	return t.Format("15:04:05")
}
