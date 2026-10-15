package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) compact() bool { return m.width < 70 || m.height < 25 }

func (m *Model) resizeCompact() {
	width := max(1, m.width-2)
	height := max(1, m.height-6)
	m.editor.SetWidth(width)
	m.editor.SetHeight(height)
	m.prompt.SetWidth(width)
	m.prompt.SetHeight(height)
	m.frontMatter.SetWidth(width)
	m.frontMatter.SetHeight(height)
	m.edit.instructions.SetWidth(width)
	m.edit.instructions.SetHeight(height)
	m.chooser.input.Width = width
}

// Compact layouts reserve space for actions and keep text fields visible when
// a phone keyboard shrinks the terminal. Coordinates come from the same rows
// used to draw the controls.
func (m *Model) renderCompact() string {
	m.layout = layoutState{}
	var lines []string
	addButton := func(label string, action buttonAction) {
		y := len(lines)
		lines = append(lines, label)
		m.layout.buttons = append(m.layout.buttons, buttonRegion{Action: action, Rect: rect{0, y, max(0, m.width-1), y}})
	}
	inputRect := func() rect { return rect{0, len(lines), max(0, m.width-1), len(lines) + max(1, m.height-6) - 1} }
	addInput := func(body string) { lines = append(lines, strings.Split(body, "\n")...) }
	if m.showFrontMatter {
		lines = append(lines, "Document Instructions · Esc closes")
		m.layout.frontMatter = inputRect()
		addInput(m.frontMatter.View())
		lines = append(lines, m.renderStatusBar())
		addButton("Save [Ctrl+S]", actionSave)
		return strings.Join(lines, "\n")
	}
	switch m.screen {
	case screenWorkspace:
		name := ""
		if m.doc != nil {
			name = filepath.Base(m.doc.Path)
		}
		if m.mode == workspaceGenerate && m.workspaceTab == 1 {
			lines = append(lines, "Enter sends · Alt+Enter newline")
		} else {
			lines = append(lines, m.mode.label()+" · "+name)
		}
		labels := []string{"Document"}
		if m.mode == workspaceGenerate {
			labels = append(labels, "Guidance")
		}
		if m.mode == workspaceEdit {
			labels = []string{"Edit ↕", "History", "Document"}
			if m.width < 30 {
				labels = []string{"Edit ↕", "Log", "Doc"}
			}
		}
		lines = append(lines, m.renderTabs(labels, len(lines)))
		region := inputRect()
		switch {
		case m.mode == workspaceGenerate && m.workspaceTab == 1:
			m.layout.prompt = region
			addInput(m.prompt.View())
		case m.mode == workspaceEdit && m.workspaceTab < 2:
			body := m.renderEditHistoryBody()
			if m.workspaceTab == 0 {
				body = m.renderSuggestionBody()
			}
			addInput(m.reviewWindow(body, m.width, max(1, m.height-6)))

		default:
			m.layout.editor = region
			addInput(m.editor.View())
		}
		lines = append(lines, m.renderStatusBar())
		if m.busy() {
			addButton("Cancel [Esc]", actionCancelBusy)
		} else if m.mode == workspaceGenerate {
			addButton("Continue [Ctrl+G]", actionContinue)
			addButton("New section [Ctrl+N]", actionNewSection)
			addButton("Undo [Ctrl+Z] · Alt+K: delete to end", actionUndo)
		} else if m.mode == workspaceEdit {
			if m.edit.suggestion != nil {
				addButton("Accept [Ctrl+A]", actionAcceptSuggestion)
				addButton("Skip [Ctrl+K]", actionSkipSuggestion)
			} else {
				addButton("Refresh [Ctrl+R]", actionRefreshSuggestion)
			}
		} else {
			addButton("Save [Ctrl+S] · Back [Esc]", actionSave)
		}
	case screenEditOptions:
		if m.edit.kind == editKindDirected {
			lines = append(lines, "Directed instructions · Alt+↑/↓ recall")
			m.layout.editInstructions = inputRect()
			addInput(m.edit.instructions.View())
			lines = append(lines, "Enter: next · Alt+Enter: newline", m.renderStatusBar())
			addButton("Next [Enter]", actionEditOptionsNext)
			addButton("Back [Esc]", actionBack)
		} else {
			lines = append(lines, "Choose an Editor · ↑/↓ Enter")
			m.compactChoices(&lines, m.editorChoices)
			lines = append(lines, m.renderStatusBar())
		}
	case screenModePicker:
		lines = append(lines, "Choose a Mode · ↑/↓ Enter")
		m.compactChoices(&lines, m.modeChoices)
		lines = append(lines, m.renderStatusBar())
	case screenApprovalPicker:
		lines = append(lines, "Choose Approval · ↑/↓ Enter")
		m.compactChoices(&lines, m.approvalChoices)
		lines = append(lines, m.renderStatusBar())
	case screenChooser:
		lines = append(lines, "Choose a document · Tab: type name")
		room := max(0, m.height-7)
		start := max(0, m.chooser.selected-room+1)
		for i := start; i < len(m.chooser.files) && i < start+room; i++ {
			prefix := "  "
			if i == m.chooser.selected {
				prefix = "› "
			}
			y := len(lines)
			lines = append(lines, prefix+m.chooser.files[i])
			m.layout.files = append(m.layout.files, fileRegion{Index: i, Rect: rect{0, y, max(0, m.width-1), y}})
		}
		lines = append(lines, "New document:")
		m.layout.chooserInput = rect{0, len(lines), max(0, m.width-1), len(lines)}
		lines = append(lines, m.chooser.input.View(), m.renderStatusBar())
		addButton("Use selected [Enter]", actionChooseSelected)
		addButton("Use typed [Tab, Enter]", actionChooseTyped)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) compactChoices(lines *[]string, choices list.Model) {
	for i, item := range choices.Items() {
		prefix := "  "
		if i == choices.Index() {
			prefix = "› "
		}
		y := len(*lines)
		*lines = append(*lines, fmt.Sprint(prefix, item.(choiceItem).title))
		m.layout.choices = append(m.layout.choices, choiceRegion{Index: i, Rect: rect{0, y, max(0, m.width-1), y}})
	}
}

// Fit normal desktop layouts using their actual rendered heights. Textarea
// height is reduced before the final frame is rendered, preserving footer rows.
func (m *Model) fitWorkspace() {
	if m.screen != screenWorkspace || m.compact() || m.showFrontMatter {
		return
	}
	offset := m.reviewOffset
	rendered := m.renderWorkspace()
	m.reviewOffset = offset
	extra := lipgloss.Height(rendered) - m.height
	if extra <= 0 {
		return
	}
	m.editor.SetHeight(max(1, m.editor.Height()-extra))
	m.prompt.SetHeight(max(1, m.prompt.Height()-extra))
}

func (m *Model) reviewWindow(body string, width, height int) string {
	lines := strings.Split(ansi.Hardwrap(body, max(1, width), true), "\n")
	m.reviewOffset = clamp(m.reviewOffset, 0, max(0, len(lines)-height))
	visible := append([]string(nil), lines[m.reviewOffset:min(len(lines), m.reviewOffset+height)]...)
	for len(visible) < height {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}
