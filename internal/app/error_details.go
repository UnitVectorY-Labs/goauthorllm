package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m *Model) renderErrorDetails() string {
	// Clear workspace hit targets while the details cover the screen.
	m.layout = layoutState{}
	lines := strings.Split(ansi.Hardwrap(m.lastError, max(1, m.width), true), "\n")
	height := max(1, m.height-2)
	m.errorOffset = clamp(m.errorOffset, 0, max(0, len(lines)-height))
	return "Error details · Esc closes\n" +
		strings.Join(lines[m.errorOffset:min(len(lines), m.errorOffset+height)], "\n") +
		"\n↑/↓ PgUp/PgDn scroll"
}
