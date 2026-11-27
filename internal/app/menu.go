package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
)

type menuEntry struct {
	Key   string
	Label string
	Hint  string
	Run   func(m *Model) tea.Cmd
}

func (m *Model) menuEntries() []menuEntry {
	var items []menuEntry

	if m.doc != nil {
		items = append(items,
			menuEntry{
				Key:   "1",
				Label: "Write & Generate",
				Hint:  "^T cycles views",
				Run:   func(m *Model) tea.Cmd { m.switchView(viewWrite); return nil },
			},
			menuEntry{
				Key:   "2",
				Label: "Copy Edit & Custom Edit",
				Hint:  "Diff review or Approve All",
				Run:   func(m *Model) tea.Cmd { m.switchView(viewEdit); return nil },
			},
		)
	}

	items = append(items,
		menuEntry{
			Key:   "3",
			Label: "Config (.goauthorllm)",
			Hint:  "Models, stream speed, writing & copy-edit rules",
			Run:   func(m *Model) tea.Cmd { m.switchView(viewConfig); return nil },
		},
		menuEntry{
			Key:   "4",
			Label: "Files — Open or Create Markdown",
			Hint:  "^O",
			Run:   func(m *Model) tea.Cmd { m.switchView(viewFiles); return nil },
		},
	)

	if m.doc != nil {
		items = append(items,
			menuEntry{
				Key:   "b",
				Label: func() string {
					if m.promptOpen {
						return "Hide Guidance Prompt"
					}
					return "Show Guidance Prompt"
				}(),
				Hint: "^B toggles",
				Run: func(m *Model) tea.Cmd {
					m.view = viewWrite
					m.togglePromptBox()
					return nil
				},
			},
			menuEntry{
				Key:   "m",
				Label: "Document Instructions (Front Matter)",
				Hint:  "YAML system_message for this file",
				Run: func(m *Model) tea.Cmd {
					m.overlay = overlayMeta
					m.syncFocus()
					return nil
				},
			},
			menuEntry{
				Key:   "s",
				Label: "Length: Sentence",
				Hint:  scopeActiveTag(m.gen.scope == engine.ScopeSentence),
				Run:   func(m *Model) tea.Cmd { m.setScopeAndReturn(engine.ScopeSentence); return nil },
			},
			menuEntry{
				Key:   "p",
				Label: "Length: Paragraph",
				Hint:  scopeActiveTag(m.gen.scope == engine.ScopeParagraph),
				Run:   func(m *Model) tea.Cmd { m.setScopeAndReturn(engine.ScopeParagraph); return nil },
			},
			menuEntry{
				Key:   "c",
				Label: "Length: Section",
				Hint:  scopeActiveTag(m.gen.scope == engine.ScopeSection),
				Run:   func(m *Model) tea.Cmd { m.setScopeAndReturn(engine.ScopeSection); return nil },
			},
			menuEntry{
				Key:   "f",
				Label: "Length: Free",
				Hint:  scopeActiveTag(m.gen.scope == engine.ScopeFree),
				Run:   func(m *Model) tea.Cmd { m.setScopeAndReturn(engine.ScopeFree); return nil },
			},
			menuEntry{
				Key:   "+",
				Label: fmt.Sprintf("Increase Rounds (currently ×%d)", m.gen.rounds),
				Hint:  "Up to ×20 auto-planned rounds",
				Run:   func(m *Model) tea.Cmd { m.adjustRounds(1); return nil },
			},
			menuEntry{
				Key:   "-",
				Label: fmt.Sprintf("Decrease Rounds (currently ×%d)", m.gen.rounds),
				Hint:  "Minimum ×1",
				Run:   func(m *Model) tea.Cmd { m.adjustRounds(-1); return nil },
			},
		)
	}

	items = append(items, menuEntry{
		Key:   "?",
		Label: "Keyboard & Mouse Help Guide",
		Hint:  "All cross-platform shortcuts",
		Run: func(m *Model) tea.Cmd {
			m.overlay = overlayHelp
			m.helpScroll = 0
			m.syncFocus()
			return nil
		},
	})

	if m.lastError != "" {
		items = append(items, menuEntry{
			Key:   "e",
			Label: "View Last Error Details",
			Hint:  truncate(m.lastError, 32),
			Run: func(m *Model) tea.Cmd {
				m.overlay = overlayError
				m.errorScroll = 0
				m.syncFocus()
				return nil
			},
		})
	}

	items = append(items, menuEntry{
		Key:   "q",
		Label: "Save & Quit",
		Hint:  "^Q",
		Run: func(m *Model) tea.Cmd {
			if err := m.saveBeforeLeave(); err != nil {
				m.setStatus("Save failed: "+err.Error(), "error")
				return nil
			}
			return tea.Quit
		},
	})

	return items
}

func scopeActiveTag(active bool) string {
	if active {
		return "● active (^L cycles)"
	}
	return "^L cycles"
}

func (m *Model) setScopeAndReturn(sc engine.Scope) {
	m.gen.scope = sc
	m.overlay = overlayNone
	if m.doc != nil {
		m.view = viewWrite
	}
	m.setStatus("Generation length: "+sc.Label()+" (^L cycles)", "accent")
	m.syncFocus()
}

func (m *Model) renderMenuOverlay(fb *frameBuilder) {
	fb.addRow([]rowItem{
		{Content: styleBrand.Render("◆ Command Menu"), Gap: 2},
		{Content: styleDim.Render("↑/↓ + Enter · press key · Esc closes"), Gap: 2},
		{ID: "close-overlay", Index: 0, Content: renderSecondaryBtn("Close [Esc]", m.isHovered("close-overlay", 0)), Gap: 0},
	}, m.width)

	entries := m.menuEntries()
	m.menuIndex = clampInt(m.menuIndex, 0, max(0, len(entries)-1))

	room := max(4, m.height-fb.y()-3)
	start := max(0, m.menuIndex-room+1)
	end := min(len(entries), start+room)

	for i := start; i < end; i++ {
		item := entries[i]
		selected := i == m.menuIndex
		hovered := m.isHovered("menu-item", i)

		keyTag := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorOrangeLight)).
			Background(lipgloss.Color(colorOrangeDark)).
			Bold(true).
			Padding(0, 1).
			Render(item.Key)

		prefix := "  "
		rowBg := ""
		labelFg := colorBody
		if hovered {
			rowBg = colorSurface
			labelFg = colorOrangeSoft
		}
		if selected {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Bold(true).Render("▸ ")
			rowBg = colorOrangeDeep
			labelFg = colorText
		}

		label := lipgloss.NewStyle().Foreground(lipgloss.Color(labelFg)).Bold(selected).Render(item.Label)
		hint := ""
		if item.Hint != "" && m.width >= 56 {
			hint = "  " + styleDim.Render(item.Hint)
		}

		line := prefix + keyTag + " " + label + hint
		if rowBg != "" {
			line = lipgloss.NewStyle().Background(lipgloss.Color(rowBg)).Width(max(12, m.width-2)).Render(truncate(line, max(12, m.width-2)))
		} else {
			line = truncate(line, max(12, m.width-2))
		}

		y := fb.y()
		fb.lines = append(fb.lines, line)
		fb.hits.add("menu-item", i, rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y})
	}

	m.renderFooter(fb, "↑/↓ Move · Enter Select · Press bracketed key · Esc Close")
}

func (m *Model) runMenuIndex(idx int) tea.Cmd {
	entries := m.menuEntries()
	if idx < 0 || idx >= len(entries) {
		return nil
	}
	entry := entries[idx]
	// Stay open on +/- so user can tap multiple times; close for all other actions.
	if entry.Key != "+" && entry.Key != "-" {
		m.overlay = overlayNone
	}
	cmd := entry.Run(m)
	m.syncFocus()
	return cmd
}

func (m *Model) runMenuByKey(k string) (bool, tea.Cmd) {
	lower := strings.ToLower(k)
	entries := m.menuEntries()
	for i, item := range entries {
		if strings.ToLower(item.Key) == lower || (item.Key == "?" && lower == "h") {
			m.menuIndex = i
			return true, m.runMenuIndex(i)
		}
	}
	return false, nil
}
