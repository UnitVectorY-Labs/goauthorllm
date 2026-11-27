package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

func newTestStudio(t *testing.T, content string) (*Model, string) {
	t.Helper()
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	docPath := filepath.Join(dir, "draft.md")
	if err := os.WriteFile(docPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		ConfigPath:            filepath.Join(dir, config.LocalConfigFilename),
		FilePath:              docPath,
		BaseURL:               "http://localhost:11434/v1",
		Model:                 "test-model",
		DefaultScope:          "paragraph",
		StructuredGeneration:  true,
		Rounds:                1,
		CopyEditBatchSize:     5,
		DirectedEditBatchSize: 10,
		Timeout:               5 * time.Second,
	}
	m, err := NewModel(cfg, nil)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	_ = m.View()
	return &m, dir
}

func clickTarget(t *testing.T, m *Model, id string, index int) {
	t.Helper()
	_ = m.View()
	for _, target := range m.hits.targets {
		if target.ID == id && target.Index == index {
			cx := (target.Rect.x1 + target.Rect.x2) / 2
			cy := (target.Rect.y1 + target.Rect.y2) / 2
			m.Update(tea.MouseMsg{X: cx, Y: cy, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
			_ = m.View()
			return
		}
	}
	t.Fatalf("clickable target %q (index %d) not found in rendered frame", id, index)
}

func TestStudioMouseClickSwitchesTabsScopesRoundsAndSpeed(t *testing.T) {
	m, _ := newTestStudio(t, "# Hello\n\nWorld.\n")

	if m.view != viewWrite {
		t.Fatalf("expected direct landing in Write view, got %v", m.view)
	}

	// Click "Section" scope pill (index 2)
	clickTarget(t, m, "scope", 2)
	if m.gen.scope != engine.ScopeSection {
		t.Fatalf("scope = %v, want section", m.gen.scope)
	}

	// Click "+" rounds button twice -> ×3
	clickTarget(t, m, "rounds-inc", 1)
	clickTarget(t, m, "rounds-inc", 1)
	if m.gen.rounds != 3 {
		t.Fatalf("rounds = %d, want 3", m.gen.rounds)
	}

	// Click Speed pill -> switches from Max (0) to first preset (35)
	clickTarget(t, m, "speed", 0)
	if m.cfg.StreamSpeed != speedPresets[1] {
		t.Fatalf("stream speed = %d, want %d", m.cfg.StreamSpeed, speedPresets[1])
	}

	// Click Edit tab, Config tab, Files tab
	clickTarget(t, m, "tab", int(viewEdit))
	if m.view != viewEdit {
		t.Fatalf("expected viewEdit, got %v", m.view)
	}
	clickTarget(t, m, "tab", int(viewConfig))
	if m.view != viewConfig {
		t.Fatalf("expected viewConfig, got %v", m.view)
	}
	clickTarget(t, m, "tab", int(viewFiles))
	if m.view != viewFiles {
		t.Fatalf("expected viewFiles, got %v", m.view)
	}
}

func TestPacedStreamingAndStopDiscardsUnrevealedBuffer(t *testing.T) {
	m, _ := newTestStudio(t, "# Story\n\nIt began.")
	m.cfg.StreamSpeed = 20 // 20 chars/sec
	m.gen.active = true
	m.gen.runID = 1
	m.gen.currentRound = 1
	m.gen.totalRounds = 1
	m.gen.scope = engine.ScopeSentence
	m.gen.currentJoinMode = document.JoinInlineSpace
	m.gen.pacer.Reset()

	// Fast LLM dumps a long response in one chunk.
	longText := "The wind howled across the canyon while three hundred extra characters kept arriving from a fast local server."
	m.handleStreamChunk(streamChunkMsg{
		runID: 1,
		round: 1,
		event: llm.StreamEvent{Delta: longText},
	})
	m.handleStreamChunk(streamChunkMsg{
		runID: 1,
		round: 1,
		event: llm.StreamEvent{Done: true},
	})

	// Simulate 100ms passing (~2 chars revealed at 20 c/s)
	m.handlePacerTick(pacerTickMsg{
		runID: 1,
		round: 1,
		at:    time.Now().Add(150 * time.Millisecond),
	})

	visibleBeforeStop := m.editor.Value()
	if strings.Contains(visibleBeforeStop, "fast local server") {
		t.Fatalf("pacer should not have revealed entire buffered response yet: %q", visibleBeforeStop)
	}

	// User presses Esc to stop right where they read up to!
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.gen.active {
		t.Fatal("expected generation to stop on Esc")
	}
	if m.gen.pacer.HasPending() {
		t.Fatal("expected unrevealed pacer buffer to be discarded on stop")
	}
	if m.editor.Value() != visibleBeforeStop {
		t.Fatalf("document changed after stop: got %q, want %q", m.editor.Value(), visibleBeforeStop)
	}
}

func TestInAppConfigEditorSavesAndHotReloads(t *testing.T) {
	m, dir := newTestStudio(t, "# Doc\n")

	m.switchView(viewConfig)
	m.cfgUI.model.SetValue("my-custom-model")
	m.cfgUI.streamSpeed.SetValue("55")
	m.cfgUI.rules.SetValue("Write crisp markdown.")
	m.cfgUI.copyEditRules.SetValue("Remove filler words.")

	// Save & apply via Ctrl+S
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.cfg.Model != "my-custom-model" || m.cfg.StreamSpeed != 55 {
		t.Fatalf("expected live hot-reload, got model=%q speed=%d", m.cfg.Model, m.cfg.StreamSpeed)
	}
	if m.cfg.Rules != "Write crisp markdown." || m.cfg.CopyEditRules != "Remove filler words." {
		t.Fatalf("expected live rules reload, got rules=%q copy=%q", m.cfg.Rules, m.cfg.CopyEditRules)
	}
	if m.generationClient.Model() != "my-custom-model" {
		t.Fatalf("expected rebuilt client model, got %q", m.generationClient.Model())
	}

	raw, err := os.ReadFile(filepath.Join(dir, config.LocalConfigFilename))
	if err != nil {
		t.Fatalf("expected .goauthorllm file written: %v", err)
	}
	if !strings.Contains(string(raw), "my-custom-model") || !strings.Contains(string(raw), "stream_chars_per_sec: 55") {
		t.Fatalf("unexpected .goauthorllm contents:\n%s", string(raw))
	}
}

func TestEditReviewAcceptSkipApproveRemainingAndUndo(t *testing.T) {
	m, _ := newTestStudio(t, "# Draft\n\nFix teh first typo.\n\nFix anothr typo.\n")
	m.switchView(viewEdit)

	e1, err := document.ResolveEdit(m.editor.Value(), document.EditSpec{
		TargetLine: 3, Action: document.ActionReplace, OldText: "teh", NewText: "the", Reason: "Typo 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	e2, err := document.ResolveEdit(m.editor.Value(), document.EditSpec{
		TargetLine: 5, Action: document.ActionReplace, OldText: "anothr", NewText: "another", Reason: "Typo 2",
	})
	if err != nil {
		t.Fatal(err)
	}

	m.edit.requestID = 1
	m.edit.requesting = true
	m.handleEditBatchMsg(editBatchMsg{
		id: 1,
		result: engine.EditBatchResult{
			Assessment: "Found 2 typos",
			Edits:      []document.ResolvedEdit{e1, e2},
		},
	})

	if m.edit.current == nil || m.edit.current.OldText != "teh" {
		t.Fatalf("expected first edit active, got %#v", m.edit.current)
	}

	// Accept first via keyboard 'y'
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !strings.Contains(m.editor.Value(), "Fix the first typo.") {
		t.Fatalf("expected first edit applied: %q", m.editor.Value())
	}
	if m.edit.current == nil || m.edit.current.OldText != "anothr" {
		t.Fatalf("expected second edit active, got %#v", m.edit.current)
	}

	// Approve remaining via 'a'
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !strings.Contains(m.editor.Value(), "Fix another typo.") {
		t.Fatalf("expected second edit applied: %q", m.editor.Value())
	}

	// Undo last via 'u'
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if !strings.Contains(m.editor.Value(), "Fix anothr typo.") {
		t.Fatalf("expected undo to restore second edit: %q", m.editor.Value())
	}
}

func TestCompactTerminalExpandsPromptOnTab(t *testing.T) {
	m, _ := newTestStudio(t, "# Small\n\nBody.\n")
	m.Update(tea.WindowSizeMsg{Width: 64, Height: 18})
	_ = m.View()

	if m.writeFocus != writeFocusEditor {
		t.Fatalf("expected editor focused initially")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	view := m.View()
	if m.writeFocus != writeFocusPrompt {
		t.Fatalf("expected prompt focused on Tab")
	}
	if !strings.Contains(view, "Guidance Prompt") {
		t.Fatalf("expected expanded Guidance Prompt on small terminal, got:\n%s", view)
	}
}
