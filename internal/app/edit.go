package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/diff"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
)

type editBatchMsg struct {
	id     int
	result engine.EditBatchResult
}

// startEditPass begins either a manual-review pass or an approve-all pass.
func (m *Model) startEditPass(approveAll bool) tea.Cmd {
	if m.doc == nil || m.editingClient == nil || m.busy() {
		return nil
	}

	m.doc.SetFrontMatter(m.frontMatter.Value())
	m.doc.SetBody(m.editor.Value())

	instructions := strings.TrimSpace(m.edit.instructions.Value())
	if instructions != "" {
		m.persistInstruction("directed", instructions)
	}

	m.edit.approveAll = approveAll
	m.edit.subTab = editTabDiff
	m.edit.focus = editFocusReview
	m.syncFocus()
	return m.requestEditBatch(instructions)
}

func (m *Model) requestEditBatch(instructions string) tea.Cmd {
	m.edit.requesting = true
	m.edit.requestID++
	m.edit.current = nil
	m.edit.pending = nil
	m.edit.batchTotal = 0
	m.edit.batchPos = 0
	m.edit.reviewScroll = 0

	batchSize := m.cfg.CopyEditBatchSize
	label := "Scanning document for copy edits..."
	if instructions != "" {
		batchSize = m.cfg.DirectedEditBatchSize
		label = "Finding edits for your instructions..."
	}
	if m.edit.approveAll {
		label = "Approve All: " + label
	}
	m.setStatus(label+" · Esc stops", "accent")

	id := m.edit.requestID
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Timeout)
	m.edit.cancel = cancel

	req := engine.EditRequest{
		Body:              m.editor.Value(),
		DocumentSystemMsg: m.doc.SystemMessage,
		ProjectRules:      m.cfg.Rules,
		CopyEditRules:     m.cfg.CopyEditRules,
		Instructions:      instructions,
		BatchSize:         batchSize,
		History:           append([]engine.EditHistoryEntry(nil), m.edit.history...),
		Overrides:         m.cfg.MessageOverrides,
	}
	client := m.editingClient

	return tea.Batch(m.spin.Tick, func() tea.Msg {
		res := engine.FetchEditBatch(ctx, client, req)
		return editBatchMsg{id: id, result: res}
	})
}

func (m *Model) handleEditBatchMsg(msg editBatchMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.edit.requestID {
		return m, nil
	}
	m.edit.requesting = false
	m.edit.cancel = nil

	if msg.result.Err != nil {
		if msg.result.Err == context.Canceled {
			m.setStatus("Edit pass stopped", "info")
		} else {
			m.setStatus("Edit pass failed: "+msg.result.Err.Error(), "error")
		}
		return m, nil
	}

	m.edit.assessment = msg.result.Assessment
	m.edit.moreRounds = msg.result.MoreRoundsNeeded

	if len(msg.result.Edits) == 0 {
		m.edit.current = nil
		m.edit.pending = nil
		m.edit.approveAll = false
		m.setStatus(msg.result.Note, "success")
		return m, nil
	}

	m.edit.pending = append([]document.ResolvedEdit(nil), msg.result.Edits...)
	m.edit.batchTotal = len(m.edit.pending)
	m.edit.batchPos = 0

	if m.edit.approveAll {
		return m, m.applyAllPendingEdits()
	}
	return m, m.advanceEditQueue()
}

func (m *Model) advanceEditQueue() tea.Cmd {
	for len(m.edit.pending) > 0 {
		next := m.edit.pending[0]
		m.edit.pending = m.edit.pending[1:]
		m.edit.batchPos++

		// Re-resolve cleanly against current document in case prior edits shifted lines.
		reResolved, err := document.ResolveEdit(m.editor.Value(), document.EditSpec{
			TargetLine: next.Line,
			Action:     document.ActionReplace,
			OldText:    next.OldText,
			NewText:    next.NewText,
			Reason:     next.Reason,
		})
		if err == nil {
			next = reResolved
		} else if _, ok := document.ApplyResolvedEdit(m.editor.Value(), next); !ok {
			continue
		}
		m.edit.current = &next
		m.edit.reviewScroll = 0
		m.setStatus(fmt.Sprintf("Edit %d/%d (Line %d) · y accept · n skip · a approve remaining", m.edit.batchPos, m.edit.batchTotal, next.Line), "accent")
		return nil
	}

	m.edit.current = nil
	if m.edit.moreRounds {
		return m.requestEditBatch(strings.TrimSpace(m.edit.instructions.Value()))
	}
	m.setStatus("All suggested edits reviewed · Ctrl+R to scan again", "success")
	return nil
}

func (m *Model) acceptCurrentEdit() tea.Cmd {
	if m.edit.current == nil || m.doc == nil {
		return nil
	}
	edit := *m.edit.current
	if !m.applyEdit(edit, "accepted") {
		m.setStatus("Edit no longer matches document; re-scanning...", "warn")
		return m.requestEditBatch(strings.TrimSpace(m.edit.instructions.Value()))
	}
	m.edit.current = nil
	return m.advanceEditQueue()
}

func (m *Model) skipCurrentEdit() tea.Cmd {
	if m.edit.current == nil {
		return nil
	}
	edit := *m.edit.current
	m.edit.history = append(m.edit.history, engine.EditHistoryEntry{
		Action:  "skipped",
		Line:    edit.Line,
		OldText: edit.OldText,
		NewText: edit.NewText,
		Reason:  edit.Reason,
	})
	m.edit.logIndex = len(m.edit.history) - 1
	m.edit.current = nil
	return m.advanceEditQueue()
}

// approveAllRemaining applies the currently shown edit + all pending edits in
// the batch, and continues fetching rounds while moreRounds is true.
func (m *Model) approveAllRemaining() tea.Cmd {
	if m.doc == nil || m.busy() {
		return nil
	}
	m.edit.approveAll = true
	if m.edit.current != nil {
		// Push current back to front of pending so applyAllPendingEdits handles it uniformly.
		m.edit.pending = append([]document.ResolvedEdit{*m.edit.current}, m.edit.pending...)
		m.edit.current = nil
	}
	if len(m.edit.pending) == 0 {
		return m.requestEditBatch(strings.TrimSpace(m.edit.instructions.Value()))
	}
	return m.applyAllPendingEdits()
}

func (m *Model) applyAllPendingEdits() tea.Cmd {
	applied := 0
	for len(m.edit.pending) > 0 {
		next := m.edit.pending[0]
		m.edit.pending = m.edit.pending[1:]
		if m.applyEdit(next, "approved-all") {
			applied++
		}
	}
	if applied > 0 && m.edit.moreRounds {
		return m.requestEditBatch(strings.TrimSpace(m.edit.instructions.Value()))
	}
	m.edit.approveAll = false
	m.setStatus(fmt.Sprintf("Approved & saved %d edit(s) · editing pass complete", applied), "success")
	return nil
}

func (m *Model) applyEdit(edit document.ResolvedEdit, action string) bool {
	before := m.editor.Value()
	updated, ok := document.ApplyResolvedEdit(before, edit)
	if !ok || updated == before {
		return false
	}
	m.pushUndo(before)
	m.edit.appliedStacks = append(m.edit.appliedStacks, before)
	if len(m.edit.appliedStacks) > 50 {
		m.edit.appliedStacks = m.edit.appliedStacks[len(m.edit.appliedStacks)-50:]
	}

	m.editor.SetValue(updated)
	m.doc.SetBody(updated)
	m.lastEditAt = time.Now()
	m.edit.history = append(m.edit.history, engine.EditHistoryEntry{
		Action:  action,
		Line:    edit.Line,
		OldText: edit.OldText,
		NewText: edit.NewText,
		Reason:  edit.Reason,
	})
	m.edit.logIndex = len(m.edit.history) - 1
	if err := m.saveCurrentDocument(); err != nil {
		m.setStatus("Applied edit, save failed: "+err.Error(), "error")
	}
	return true
}

func (m *Model) undoLastAppliedEdit() {
	if len(m.edit.appliedStacks) == 0 || m.doc == nil {
		m.setStatus("No applied edits to undo", "info")
		return
	}
	last := len(m.edit.appliedStacks) - 1
	prev := m.edit.appliedStacks[last]
	m.edit.appliedStacks = m.edit.appliedStacks[:last]
	m.editor.SetValue(prev)
	m.doc.SetBody(prev)
	_ = m.saveCurrentDocument()
	m.setStatus("Undid last applied edit & saved", "success")
}

func (m *Model) stopEditPass() {
	if !m.edit.requesting {
		return
	}
	m.edit.requestID++
	if m.edit.cancel != nil {
		m.edit.cancel()
		m.edit.cancel = nil
	}
	m.edit.requesting = false
	m.edit.approveAll = false
	m.setStatus("Stopped edit pass", "info")
}

func (m *Model) renderDiffBody() string {
	if m.edit.requesting {
		msg := "Scanning document with " + m.activeModelName() + "..."
		if m.edit.approveAll {
			msg = "Approve All running with " + m.activeModelName() + "..."
		}
		return m.spin.View() + " " + lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrangeSoft)).Render(msg)
	}
	if m.edit.current == nil {
		var lines []string
		if m.edit.assessment != "" {
			lines = append(lines, styleTitle.Render("Last pass: ")+styleMuted.Render(m.edit.assessment), "")
		}
		lines = append(lines,
			styleMuted.Render("No active edit suggestion."),
			"",
			styleDim.Render("• Leave instructions blank and press ^G (or click Review) for copy editing."),
			styleDim.Render("• Type custom instructions above for targeted rewrites, renames, or removals."),
			styleDim.Render("• Press ^X (or click Approve All) to auto-apply all validated edits."),
		)
		if strings.TrimSpace(m.cfg.CopyEditRules) != "" {
			lines = append(lines, "", styleDim.Render("Active .goauthorllm copy_edit_rules: "+truncate(strings.ReplaceAll(m.cfg.CopyEditRules, "\n", " "), 90)))
		}
		return strings.Join(lines, "\n")
	}

	e := m.edit.current
	var parts []string

	header := fmt.Sprintf("Edit %d of %d · Line %d · %s", m.edit.batchPos, max(1, m.edit.batchTotal), e.Line, strings.ToUpper(string(e.Action)))
	parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOrangeSoft)).Render(header))
	if e.Reason != "" {
		parts = append(parts, styleMuted.Render("Why: "+e.Reason))
	}
	if e.ContextPre != "" {
		parts = append(parts, "", styleDim.Render("  ... "+e.ContextPre))
	}

	ops := diff.Diff(e.OldText, e.NewText)
	parts = append(parts,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorRed)).Render("− Before"),
		diff.FormatOld(ops, styleDiffCtx, styleDiffDel),
		"",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorGreen)).Render("+ After"),
		diff.FormatNew(ops, styleDiffCtx, styleDiffIns),
	)
	if e.ContextPost != "" {
		parts = append(parts, "", styleDim.Render("  ... "+e.ContextPost))
	}
	return strings.Join(parts, "\n")
}

func (m *Model) renderEditLogBody() string {
	if len(m.edit.history) == 0 {
		return styleMuted.Render("No edits recorded in this session yet.")
	}
	idx := clampInt(m.edit.logIndex, 0, len(m.edit.history)-1)
	entry := m.edit.history[idx]
	ops := diff.Diff(entry.OldText, entry.NewText)

	badgeColor := colorGreenDark
	badgeFg := colorGreenSoft
	if entry.Action == "skipped" {
		badgeColor = colorSurface
		badgeFg = colorMuted
	}

	parts := []string{
		fmt.Sprintf("%s  Entry %d of %d · Line %d  %s",
			renderBadge(entry.Action, badgeColor, badgeFg),
			idx+1, len(m.edit.history), entry.Line,
			styleDim.Render("(←/→ browse)"),
		),
	}
	if entry.Reason != "" {
		parts = append(parts, styleMuted.Render("Why: "+entry.Reason))
	}
	parts = append(parts,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorRed)).Render("− Before"),
		diff.FormatOld(ops, styleDiffCtx, styleDiffDel),
		"",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorGreen)).Render("+ After"),
		diff.FormatNew(ops, styleDiffCtx, styleDiffIns),
	)
	return strings.Join(parts, "\n")
}
