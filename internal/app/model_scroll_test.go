package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

func TestRawWheelDirectionRecognizesWheelRunes(t *testing.T) {
	model := &Model{}

	if direction, ok := model.rawWheelDirection(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[<64;48;17M")}); !ok || direction != -1 {
		t.Fatalf("expected wheel up to be recognized, got direction=%d ok=%v", direction, ok)
	}

	if direction, ok := model.rawWheelDirection(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<65;48;17M")}); !ok || direction != 1 {
		t.Fatalf("expected wheel down to be recognized, got direction=%d ok=%v", direction, ok)
	}
}

func TestScrollTextAreaMatchesArrowMovement(t *testing.T) {
	model := &Model{}
	input := newTextarea("", true)
	input.SetWidth(40)
	input.SetHeight(5)
	input.SetValue("line 1\nline 2\nline 3\nline 4\nline 5\nline 6")
	input.Focus()
	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})

	model.scrollTextArea(&input, 1, 3)

	if got := input.Line(); got != 3 {
		t.Fatalf("expected cursor to move down to line index 3, got %d", got)
	}

	model.scrollTextArea(&input, -1, 2)

	if got := input.Line(); got != 1 {
		t.Fatalf("expected cursor to move up to line index 1, got %d", got)
	}
}

func TestGenerationDeltaAppendsAfterCursorWasScrolled(t *testing.T) {
	model := &Model{
		doc:               &document.Document{Body: "line 1\nline 2\nline 3\nline 4"},
		generating:        true,
		generationStarted: true,
		editor:            newTextarea("", true),
	}
	model.editor.SetWidth(40)
	model.editor.SetHeight(2)
	model.editor.SetValue(model.doc.Body)
	model.editor.Focus()
	model.editor, _ = model.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})

	model.scrollTextArea(&model.editor, -1, 2)
	model.applyGenerationDelta("\nline 5")

	if got, want := model.doc.Body, "line 1\nline 2\nline 3\nline 4\nline 5"; got != want {
		t.Fatalf("generation delta was inserted at the cursor: got %q want %q", got, want)
	}
}

func TestWheelDoesNotMoveCursorDuringGeneration(t *testing.T) {
	model := &Model{
		screen:     screenWorkspace,
		focus:      focusEditor,
		generating: true,
		editor:     newTextarea("", true),
		layout:     layoutState{editor: rect{x1: 0, y1: 0, x2: 80, y2: 10}},
	}
	model.editor.SetWidth(40)
	model.editor.SetHeight(2)
	model.editor.SetValue("line 1\nline 2\nline 3\nline 4")
	model.editor.Focus()
	model.editor, _ = model.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})

	model.handleMouse(tea.MouseMsg{X: 2, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})

	if got := model.editor.Line(); got != 3 {
		t.Fatalf("wheel moved editor cursor during generation: got line %d want 3", got)
	}
}

func TestEnterAddsNewlineInEditor(t *testing.T) {
	model, err := NewModel(config.Config{
		BaseURL: "http://example.com",
		Model:   "test-model",
		Timeout: 1,
	}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}

	model.width = 100
	model.height = 40
	model.screen = screenWorkspace
	model.focus = focusEditor
	model.resize()
	model.syncFocus()
	model.doc = &document.Document{Path: "test.md", Body: "", LastSavedAt: time.Now()}
	model.editor.SetValue("hello")
	model.editor.Focus()
	model.focus = focusEditor

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*Model).editor.Value()
	if got != "hello\n" {
		t.Fatalf("expected enter to add newline, got %q", got)
	}
}

func TestEditorAllowsNewlinesBeyondTwoHundredLines(t *testing.T) {
	model, err := NewModel(config.Config{
		BaseURL: "http://example.com",
		Model:   "test-model",
		Timeout: 1,
	}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}

	model.width = 100
	model.height = 40
	model.screen = screenWorkspace
	model.focus = focusEditor
	model.resize()
	model.syncFocus()
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	model.doc = &document.Document{Path: "test.md", Body: strings.Join(lines, "\n"), LastSavedAt: time.Now()}
	model.editor.SetValue(model.doc.Body)
	model.editor.Focus()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	updated, _ = updated.(*Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*Model).editor.Value()
	if want := strings.Join(lines, "\n") + "\n"; got != want {
		t.Fatalf("expected newline to be inserted after 200 lines, got %q", got)
	}
}

func TestNormalizeGenerationStartContinueAddsMissingSpace(t *testing.T) {
	got := normalizeGenerationStart("The hero said", "nothing more.", modeContinue)
	if got != " nothing more." {
		t.Fatalf("expected leading space to be inserted, got %q", got)
	}
}

func TestNormalizeGenerationStartContinueAddsParagraphAfterHeading(t *testing.T) {
	got := normalizeGenerationStart("# Scene", "The room was quiet.", modeContinue)
	if got != "\n\nThe room was quiet." {
		t.Fatalf("expected paragraph break after heading, got %q", got)
	}
}

func TestNormalizeGenerationStartContinueKeepsPunctuationTight(t *testing.T) {
	got := normalizeGenerationStart("Wait", "...", modeContinue)
	if got != "..." {
		t.Fatalf("expected punctuation to stay tight, got %q", got)
	}
}

func TestNormalizeGenerationStartNewSectionUsesBlankLine(t *testing.T) {
	got := normalizeGenerationStart("Existing text", "\n\n# Next\nBody", modeNewSection)
	if got != "\n\n# Next\nBody" {
		t.Fatalf("expected normalized section boundary, got %q", got)
	}
}

func TestViewIsClampedToWindowHeight(t *testing.T) {
	model, err := NewModel(config.Config{
		BaseURL: "http://example.com",
		Model:   "test-model",
		Timeout: 1,
	}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}

	model.width = 100
	model.height = 20
	model.screen = screenWorkspace
	model.focus = focusEditor
	model.doc = &document.Document{
		Path:        "test.md",
		Body:        "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10",
		LastSavedAt: time.Now(),
	}
	model.editor.SetValue(model.doc.Body)
	model.prompt.SetValue("prompt")
	model.resize()
	model.syncFocus()

	view := model.View()
	if got := lineCount(view); got != model.height {
		t.Fatalf("expected view height %d, got %d", model.height, got)
	}
}

func TestRenderWidthStaysWithinWindowWhileMovingCursor(t *testing.T) {
	model, err := NewModel(config.Config{
		BaseURL: "http://example.com",
		Model:   "test-model",
		Timeout: 1,
	}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}

	model.width = 80
	model.height = 24
	model.screen = screenWorkspace
	model.focus = focusEditor
	model.doc = &document.Document{
		Path: "test.md",
		Body: "This is a deliberately long paragraph that should wrap across multiple visual rows in the textarea component while the cursor moves through it.\n\nSecond paragraph with enough text to keep wrapping active as we move.",
	}
	model.editor.SetValue(model.doc.Body)
	model.prompt.SetValue("prompt")
	model.resize()
	model.syncFocus()
	model.editor.Focus()

	for range 40 {
		view := model.View()
		for line := range strings.SplitSeq(view, "\n") {
			if width := lipgloss.Width(line); width > model.width {
				t.Fatalf("rendered line wider than window: got %d want <= %d, line=%q", width, model.width, line)
			}
		}

		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = *next.(*Model)
	}
}

func TestEditorViewHeightStaysStable(t *testing.T) {
	input := newTextarea("", true)
	input.SetWidth(60)
	input.SetHeight(10)
	input.SetValue("This is a deliberately long paragraph that should wrap across multiple visual rows in the textarea component while the cursor moves through it.\n\nSecond paragraph with enough text to keep wrapping active as we move.")
	input.Focus()

	expected := lineCount(input.View())
	for i := range 50 {
		if got := lineCount(input.View()); got != expected {
			t.Fatalf("expected stable editor height %d, got %d on step %d", expected, got, i)
		}
		input, _ = input.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
}

func TestBackFromWorkspaceReturnsToModePicker(t *testing.T) {
	model, err := NewModel(config.Config{
		BaseURL: "http://example.com",
		Model:   "test-model",
		Timeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}

	model.screen = screenWorkspace
	model.screenPath = []screenState{screenChooser, screenModePicker}
	model.mode = workspaceEdit
	model.doc = &document.Document{Path: "test.md", Body: "Hello", LastSavedAt: time.Now()}
	model.editor.SetValue("Hello")

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(*Model)

	if updated.screen != screenModePicker {
		t.Fatalf("expected to return to mode picker, got %v", updated.screen)
	}
	if updated.focus != focusModeDocument {
		t.Fatalf("expected mode picker focus to reset, got %v", updated.focus)
	}
}

func TestAcceptSuggestionReplacesUniqueMatch(t *testing.T) {
	model, err := NewModel(config.Config{
		BaseURL: "http://example.com",
		Model:   "test-model",
		Timeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "draft.md")
	if err := os.WriteFile(path, []byte("Alpha beta.\n"), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	doc, err := document.Load(path)
	if err != nil {
		t.Fatalf("load draft: %v", err)
	}

	model.screen = screenWorkspace
	model.mode = workspaceEdit
	model.doc = doc
	model.editor.SetValue(doc.Body)
	model.edit.suggestion = &editSuggestion{
		OldText: "beta",
		NewText: "delta",
	}

	cmd := model.acceptSuggestion()
	if cmd != nil {
		t.Fatal("expected no follow-up command without an LLM client")
	}

	if got := model.doc.Body; got != "Alpha delta.\n" {
		t.Fatalf("unexpected updated body: %q", got)
	}
	if model.doc.Dirty {
		t.Fatal("expected accepted suggestion to be saved")
	}
	if len(model.edit.history) != 1 || model.edit.history[0].Action != "accepted" {
		t.Fatalf("expected accepted suggestion history, got %#v", model.edit.history)
	}
}

func TestEditHistoryRetainsEverySessionEntry(t *testing.T) {
	model := Model{}
	for i := range 25 {
		model.appendEditHistory("accepted", editSuggestion{OldText: fmt.Sprintf("old-%d", i), NewText: fmt.Sprintf("new-%d", i)})
	}
	if len(model.edit.history) != 25 {
		t.Fatalf("expected complete history, got %d entries", len(model.edit.history))
	}
	if model.edit.history[0].OldText != "old-0" || model.edit.historyIndex != 24 {
		t.Fatalf("unexpected retained history: %#v", model.edit)
	}
}

func TestCopyEditorSelectionAdvancesWithEnter(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.screen = screenEditOptions
	model.focus = focusEditDefault
	model.editorChoices.Select(0)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(*Model)
	if updated.screen != screenApprovalPicker {
		t.Fatalf("expected Enter to advance to approval picker, got %v", updated.screen)
	}
}

func TestAutomaticReviewKeepsSuggestionVisibleWhileChecking(t *testing.T) {
	client := llm.NewClient("http://example.com", "test-model", "", time.Second)
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, client)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.width = 100
	model.height = 30
	model.screen = screenWorkspace
	model.mode = workspaceEdit
	model.focus = focusWorkspaceTabs
	model.doc = &document.Document{Path: "test.md", Body: "Lena walk home."}
	model.editor.SetValue(model.doc.Body)
	model.edit.approval = approvalAutomatic
	model.edit.requestID = 7

	suggestion := editSuggestion{OldText: "Lena walk", NewText: "Lena walks", RemainingRounds: 1}
	_, cmd := model.handleEditMsg(editMsg{id: 7, result: editSuggestionResult{Suggestions: []editSuggestion{suggestion}, RemainingRounds: 1}})
	if cmd == nil {
		t.Fatal("expected automatic approval command")
	}
	if model.edit.suggestion == nil || model.edit.suggestion.OldText != "Lena walk" {
		t.Fatalf("suggestion was cleared before approval: %#v", model.edit.suggestion)
	}
	if !model.edit.reviewing {
		t.Fatal("expected automatic review to be active")
	}
	model.resize()
	view := model.View()
	if !strings.Contains(view, "Lena walk") || !strings.Contains(view, "Lena walks") {
		t.Fatalf("active suggestion is not visible during automatic review:\n%s", view)
	}
}

func TestRejectedAutomaticReviewRetainsSuggestion(t *testing.T) {
	modelValue, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model := &modelValue
	model.mode = workspaceEdit
	model.edit.requestID = 3
	model.edit.reviewing = true
	model.edit.suggestion = &editSuggestion{OldText: "old", NewText: "new"}
	model.handleEditApprovalMsg(editApprovalMsg{id: 3, approved: false})
	if model.edit.suggestion == nil {
		t.Fatal("rejected automatic review should keep the suggestion for manual review")
	}
	if model.edit.reviewing {
		t.Fatal("automatic review should be complete")
	}
}

func TestHistoryPanePagesBackward(t *testing.T) {
	model := &Model{
		screen:       screenWorkspace,
		mode:         workspaceEdit,
		focus:        focusHistoryPane,
		workspaceTab: 1,
		edit: editState{
			historyIndex: 2,
			history: []editHistoryEntry{
				{Action: "auto-accepted", OldText: "one", NewText: "1"},
				{Action: "auto-accepted", OldText: "two", NewText: "2"},
				{Action: "auto-accepted", OldText: "three", NewText: "3"},
			},
		},
	}

	model.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if model.edit.historyIndex != 1 {
		t.Fatalf("expected previous history entry, got index %d", model.edit.historyIndex)
	}
}

func TestTabSwitchesGenerateWorkspaceTabs(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.screen = screenWorkspace
	model.mode = workspaceGenerate
	model.workspaceTab = 0
	model.focus = focusEditor

	model.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if model.workspaceTab != 1 || model.focus != focusPrompt {
		t.Fatalf("expected Guidance tab, got tab=%d focus=%v", model.workspaceTab, model.focus)
	}
	model.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if model.workspaceTab != 0 || model.focus != focusEditor {
		t.Fatalf("expected Document tab, got tab=%d focus=%v", model.workspaceTab, model.focus)
	}
}

func TestTabSwitchesEditWorkspaceTabs(t *testing.T) {
	modelValue, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model := &modelValue
	model.screen = screenWorkspace
	model.mode = workspaceEdit
	model.workspaceTab = 0
	model.focus = focusWorkspaceTabs
	model.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if model.workspaceTab != 1 || model.focus != focusHistoryPane {
		t.Fatalf("expected History tab, got tab=%d focus=%v", model.workspaceTab, model.focus)
	}
	model.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if model.workspaceTab != 2 || model.focus != focusEditor {
		t.Fatalf("expected Document tab, got tab=%d focus=%v", model.workspaceTab, model.focus)
	}
}

func TestModePickerClickOpensEditOptions(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.width = 90
	model.height = 28
	model.screen = screenModePicker
	model.pendingName = "test.md"
	model.resize()
	model.View()
	if len(model.layout.choices) != 3 {
		t.Fatalf("expected three clickable mode choices, got %d", len(model.layout.choices))
	}
	region := model.layout.choices[2].Rect
	model.handleMouse(tea.MouseMsg{X: region.x1 + 2, Y: region.y1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if model.screen != screenEditOptions {
		t.Fatalf("expected edit options after clicking Edit with AI, got %v", model.screen)
	}
}

func TestEditOptionClickSelectsDirectedEditor(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.width = 90
	model.height = 30
	model.screen = screenEditOptions
	model.resize()
	model.View()
	region := model.layout.choices[1].Rect
	model.handleMouse(tea.MouseMsg{X: region.x1 + 2, Y: region.y1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if model.edit.kind != editKindDirected || model.focus != focusEditInstructions {
		t.Fatalf("expected directed editor instructions after click, got kind=%v focus=%v", model.edit.kind, model.focus)
	}
}

func TestEditFooterOnlyShowsAvailableActions(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.width = 100
	model.height = 30
	model.screen = screenWorkspace
	model.mode = workspaceEdit
	model.doc = &document.Document{Path: "test.md", Body: "Body"}
	model.editor.SetValue("Body")
	model.edit.requesting = true
	model.resize()
	view := model.View()
	if strings.Contains(view, "Accept [") || strings.Contains(view, "Skip [") || strings.Contains(view, "Refresh [") {
		t.Fatalf("busy edit footer showed unavailable suggestion actions:\n%s", view)
	}
	if !strings.Contains(view, "Cancel [Esc]") || !strings.Contains(view, "History [Alt+H]") {
		t.Fatalf("busy edit footer omitted available actions:\n%s", view)
	}
}

func TestClickableTextAreaTracksHoverMotion(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.width = 90
	model.height = 28
	model.screen = screenWorkspace
	model.mode = workspaceDocument
	model.doc = &document.Document{Path: "test.md", Body: "Body"}
	model.editor.SetValue("Body")
	model.resize()
	model.View()

	region := model.layout.editor
	handled, _ := model.handleMouse(tea.MouseMsg{X: region.x1 + 2, Y: region.y1 + 1, Action: tea.MouseActionMotion})
	if !handled || model.hover != focusEditor {
		t.Fatalf("expected editor hover, got handled=%v hover=%v", handled, model.hover)
	}

	handled, _ = model.handleMouse(tea.MouseMsg{X: model.width - 1, Y: model.height - 1, Action: tea.MouseActionMotion})
	if !handled || model.hover != focusTarget(-1) {
		t.Fatalf("expected hover to clear, got handled=%v hover=%v", handled, model.hover)
	}
}

func TestCustomInstructionsCanBeHoveredAndClicked(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.width = 90
	model.height = 32
	model.screen = screenEditOptions
	model.edit.kind = editKindDirected
	model.focus = focusEditDefault
	model.resize()
	model.View()
	region := model.layout.editInstructions

	model.handleMouse(tea.MouseMsg{X: region.x1 + 2, Y: region.y1 + 1, Action: tea.MouseActionMotion})
	if model.hover != focusEditInstructions {
		t.Fatalf("expected custom instructions hover, got %v", model.hover)
	}
	model.handleMouse(tea.MouseMsg{X: region.x1 + 2, Y: region.y1 + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if model.focus != focusEditInstructions {
		t.Fatalf("expected click to focus custom instructions, got %v", model.focus)
	}
}

func TestAutomaticBatchPreservesHistoryTabUntilDecisionRequired(t *testing.T) {
	client := llm.NewClient("http://example.com", "test-model", "", time.Second)
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, client)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.screen = screenWorkspace
	model.mode = workspaceEdit
	model.workspaceTab = 1
	model.focus = focusHistoryPane
	model.doc = &document.Document{Path: "test.md", Body: "First old. Second old."}
	model.editor.SetValue(model.doc.Body)
	model.edit.approval = approvalAutomatic
	model.edit.kind = editKindDirected
	model.edit.requestID = 4

	suggestions := []editSuggestion{{OldText: "First old", NewText: "First new"}, {OldText: "Second old", NewText: "Second new"}}
	_, cmd := model.handleEditMsg(editMsg{id: 4, result: editSuggestionResult{Suggestions: suggestions, RemainingRounds: 0}})
	if cmd == nil || model.workspaceTab != 1 {
		t.Fatalf("automatic batch should review without leaving History: cmd=%v tab=%d", cmd != nil, model.workspaceTab)
	}
	model.handleEditApprovalMsg(editApprovalMsg{id: 4, approved: false})
	if model.workspaceTab != 0 {
		t.Fatalf("rejected automatic suggestion should open Suggestion tab, got %d", model.workspaceTab)
	}
}

func TestApproveAllAppliesEntireDirectedBatch(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "directed.md")
	if err := os.WriteFile(path, []byte("First old. Second old."), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	doc, err := document.Load(path)
	if err != nil {
		t.Fatalf("load document: %v", err)
	}
	model.screen = screenWorkspace
	model.mode = workspaceEdit
	model.workspaceTab = 1
	model.doc = doc
	model.editor.SetValue(doc.Body)
	model.edit.kind = editKindDirected
	model.edit.approval = approvalAll
	model.edit.requestID = 2
	suggestions := []editSuggestion{{OldText: "First old", NewText: "First new"}, {OldText: "Second old", NewText: "Second new"}}

	_, cmd := model.handleEditMsg(editMsg{id: 2, result: editSuggestionResult{Suggestions: suggestions, RemainingRounds: 0}})
	if cmd != nil {
		t.Fatal("expected completed directed batch without another request")
	}
	if model.doc.Body != "First new. Second new." || len(model.edit.history) != 2 {
		t.Fatalf("batch was not fully applied: body=%q history=%#v", model.doc.Body, model.edit.history)
	}
	if model.workspaceTab != 1 {
		t.Fatalf("approve-all batch should preserve History tab, got %d", model.workspaceTab)
	}
}

func TestSkippingOneDirectedBatchItemDoesNotMarkTaskComplete(t *testing.T) {
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "test-model", Timeout: time.Second}, nil)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "directed.md")
	if err := os.WriteFile(path, []byte("First old. Second old."), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	model.doc, err = document.Load(path)
	if err != nil {
		t.Fatalf("load document: %v", err)
	}
	model.editor.SetValue(model.doc.Body)
	model.screen = screenWorkspace
	model.mode = workspaceEdit
	model.edit.kind = editKindDirected
	model.edit.approval = approvalManual
	model.edit.requestID = 3

	suggestions := []editSuggestion{{OldText: "First old", NewText: "First new"}, {OldText: "Second old", NewText: "Second new"}}
	model.handleEditMsg(editMsg{id: 3, result: editSuggestionResult{Suggestions: suggestions, RemainingRounds: 0}})
	model.runAction(actionSkipSuggestion)
	if model.edit.suggestion == nil || model.edit.suggestion.OldText != "Second old" {
		t.Fatalf("expected the second queued suggestion after skip, got %#v", model.edit.suggestion)
	}
	if !model.applySuggestion(*model.edit.suggestion, "accepted") {
		t.Fatal("expected the remaining suggestion to apply")
	}
	model.finishEditBatch("Reviewed batch", false)
	if strings.Contains(model.statusText, "task complete") {
		t.Fatal("a batch with a skipped directed change must not mark the directed task complete")
	}
}

func TestSpecializedModelClientsAndLabels(t *testing.T) {
	defaultClient := llm.NewClient("http://example.com", "default-model", "", time.Second)
	model, err := NewModel(config.Config{BaseURL: "http://example.com", Model: "default-model", GenerationModel: "generation-model", EditingModel: "editing-model", Timeout: time.Second}, defaultClient)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	if model.generationClient == defaultClient || model.editingClient == defaultClient || model.generationClient == model.editingClient {
		t.Fatal("expected separate clients for specialized models")
	}
	model.screen = screenWorkspace
	model.mode = workspaceGenerate
	if got := model.activeModelName(); got != "generation-model" {
		t.Fatalf("unexpected generation model label: %q", got)
	}
	model.mode = workspaceEdit
	if got := model.activeModelName(); got != "editing-model" {
		t.Fatalf("unexpected editing model label: %q", got)
	}
}
