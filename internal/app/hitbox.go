package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// rect is an inclusive 0-based terminal screen coordinate box.
type rect struct {
	x1, y1, x2, y2 int
}

func (r rect) contains(x, y int) bool {
	return x >= r.x1 && x <= r.x2 && y >= r.y1 && y <= r.y2
}

// hitTarget identifies any clickable or hoverable region in the current frame.
type hitTarget struct {
	ID    string
	Index int
	Rect  rect
}

// hitMap holds all interactive regions registered during a single View() pass.
// Because every region is registered from the exact row index (y) and rendered
// lipgloss width as lines are built, mouse clicks and hover states match the
// rendered screen with zero coordinate drift.
type hitMap struct {
	targets []hitTarget
	panes   map[string]rect
}

func newHitMap() hitMap {
	return hitMap{
		panes: make(map[string]rect, 8),
	}
}

func (h *hitMap) add(id string, index int, r rect) {
	h.targets = append(h.targets, hitTarget{ID: id, Index: index, Rect: r})
}

func (h *hitMap) setPane(name string, r rect) {
	if h.panes == nil {
		h.panes = make(map[string]rect, 8)
	}
	h.panes[name] = r
}

func (h *hitMap) hit(x, y int) (hitTarget, bool) {
	// Reverse scan so later/overlay targets win if anything overlaps.
	for i := len(h.targets) - 1; i >= 0; i-- {
		if h.targets[i].Rect.contains(x, y) {
			return h.targets[i], true
		}
	}
	return hitTarget{}, false
}

func (h *hitMap) paneAt(x, y int) string {
	for name, r := range h.panes {
		if r.contains(x, y) {
			return name
		}
	}
	return ""
}

// frameBuilder assembles a screen top-to-bottom while keeping exact line indices.
type frameBuilder struct {
	lines []string
	hits  hitMap
}

func newFrameBuilder() *frameBuilder {
	return &frameBuilder{hits: newHitMap()}
}

func (f *frameBuilder) y() int {
	return len(f.lines)
}

func (f *frameBuilder) addBlock(rendered string) rect {
	startY := len(f.lines)
	blockLines := strings.Split(rendered, "\n")
	maxW := 0
	for _, ln := range blockLines {
		if w := lipgloss.Width(ln); w > maxW {
			maxW = w
		}
	}
	f.lines = append(f.lines, blockLines...)
	endY := max(startY, len(f.lines)-1)
	return rect{x1: 0, y1: startY, x2: max(0, maxW-1), y2: endY}
}

func (f *frameBuilder) addBlank() {
	f.lines = append(f.lines, "")
}

// rowItem is one clickable inline pill/tab/button placed on a horizontal strip.
type rowItem struct {
	ID      string
	Index   int
	Content string
	Gap     int // spaces after item (default 1 if 0)
}

// addRow renders horizontal items (wrapping cleanly if maxWidth is exceeded)
// and registers exact hitboxes for every item with an ID.
func (f *frameBuilder) addRow(items []rowItem, maxWidth int) {
	if maxWidth < 10 {
		maxWidth = 10
	}
	var rowParts []string
	x := 0
	flush := func() {
		if len(rowParts) == 0 {
			return
		}
		f.lines = append(f.lines, strings.Join(rowParts, ""))
		rowParts = nil
		x = 0
	}
	for _, item := range items {
		w := lipgloss.Width(item.Content)
		gap := item.Gap
		if gap == 0 {
			gap = 1
		}
		if len(rowParts) > 0 && x+w > maxWidth {
			flush()
		}
		y := len(f.lines)
		if item.ID != "" {
			f.hits.add(item.ID, item.Index, rect{x1: x, y1: y, x2: x + max(1, w) - 1, y2: y})
		}
		rowParts = append(rowParts, item.Content)
		x += w
		if gap > 0 {
			rowParts = append(rowParts, strings.Repeat(" ", gap))
			x += gap
		}
	}
	flush()
}

func (f *frameBuilder) String() string {
	return strings.Join(f.lines, "\n")
}
