package app

import (
	"strings"
	"time"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/history"
	tea "github.com/charmbracelet/bubbletea"
)

type instructionRecall struct {
	path, kind, draft string
	entries           []string
	index             int
}

func (m *Model) instructionPath() string {
	if m.screen == screenEditOptions {
		return m.pendingPath
	}
	if m.doc != nil {
		return m.doc.Path
	}
	return m.pendingPath
}

func (m *Model) persistInstruction(kind, value string) bool {
	if err := history.Save(m.instructionPath(), kind, value); err != nil {
		m.setStatus("Save instructions failed: "+err.Error(), "error")
		return false
	}
	if strings.TrimSpace(value) != "" {
		m.instructionRecall = instructionRecall{index: -1}
	}
	return true
}

func (m *Model) recallInstruction(older bool) {
	kind := "generation"
	input := &m.prompt
	if m.focus == focusEditInstructions {
		kind = "directed"
		input = &m.edit.instructions
	}
	path := m.instructionPath()
	r := &m.instructionRecall
	if r.path != path || r.kind != kind || r.entries == nil || (r.index >= 0 && input.Value() != r.entries[r.index]) {
		entries, err := history.Load(path, kind)
		if err != nil {
			m.setStatus("Load instructions failed: "+err.Error(), "error")
			return
		}
		*r = instructionRecall{path: path, kind: kind, draft: input.Value(), entries: entries, index: -1}
	}
	if len(r.entries) == 0 {
		m.setStatus("No saved instructions for this file yet", "muted")
		return
	}
	if r.index < 0 {
		r.draft = input.Value()
	}
	if older {
		r.index = min(r.index+1, len(r.entries)-1)
	} else {
		r.index = max(-1, r.index-1)
	}
	if r.index < 0 {
		input.SetValue(r.draft)
	} else {
		input.SetValue(r.entries[r.index])
	}
	input.CursorEnd()
	m.setStatus("Recalled instructions · Alt+↑/↓ browses history", "info")
}

func (m *Model) undoBody() {
	if len(m.undoBodies) == 0 {
		m.setStatus("Nothing to undo", "muted")
		return
	}
	last := len(m.undoBodies) - 1
	m.doc.SetBody(m.undoBodies[last])
	m.undoBodies = m.undoBodies[:last]
	m.editor.SetValue(m.doc.Body)
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.lastEditAt = time.Now()
	m.lastUndoEditAt = time.Time{}
	m.setStatus("Undid document change", "success")
}

func (m *Model) deleteToEnd() {
	if m.doc == nil {
		return
	}
	body := m.editor.Value()
	lines := strings.Split(body, "\n")
	line := m.editor.Line()
	// Bubbles ColumnOffset and StartColumn index runes, including soft wraps.
	info := m.editor.LineInfo()
	col := info.StartColumn + info.ColumnOffset
	lines[line] = string([]rune(lines[line])[:col])
	updated := strings.Join(lines[:line+1], "\n")
	if updated == body {
		return
	}
	m.pushUndo(body)
	m.editor.SetValue(updated)
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.doc.SetBody(updated)
	m.lastEditAt = time.Now()
	m.setStatus("Deleted from cursor to end · Ctrl+Z restores", "success")
}

func (m *Model) pushUndo(body string) {
	m.undoBodies = append(m.undoBodies, body)
	if len(m.undoBodies) > 100 {
		m.undoBodies = append([]string(nil), m.undoBodies[len(m.undoBodies)-100:]...)
	}
	m.lastUndoEditAt = time.Time{}
}
