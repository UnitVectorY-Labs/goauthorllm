package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"goauthorllm/internal/config"
	"goauthorllm/internal/document"
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
	var cmd tea.Cmd
	input, cmd = input.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	_ = cmd

	model.scrollTextArea(&input, 1, 3)

	if got := input.Line(); got != 3 {
		t.Fatalf("expected cursor to move down like arrow keys to line index 3, got %d", got)
	}

	model.scrollTextArea(&input, -1, 2)

	if got := input.Line(); got != 1 {
		t.Fatalf("expected cursor to move up like arrow keys to line index 1, got %d", got)
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

func TestNormalizeGenerationStartNewSectionUsesBlankLineAndTrimsLeadingWhitespace(t *testing.T) {
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

	for i := 0; i < 40; i++ {
		view := model.View()
		for _, line := range strings.Split(view, "\n") {
			if width := lipgloss.Width(line); width > model.width {
				t.Fatalf("rendered line wider than window: got %d want <= %d, line=%q", width, model.width, line)
			}
		}

		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = *next.(*Model)
	}
}

func TestEditorViewHeightStaysStableWhileMovingCursor(t *testing.T) {
	input := newTextarea("", true)
	input.SetWidth(60)
	input.SetHeight(10)
	input.SetValue("This is a deliberately long paragraph that should wrap across multiple visual rows in the textarea component while the cursor moves through it.\n\nSecond paragraph with enough text to keep wrapping active as we move.")
	input.Focus()

	expected := lineCount(input.View())
	for i := 0; i < 50; i++ {
		if got := lineCount(input.View()); got != expected {
			t.Fatalf("expected stable editor height %d, got %d on step %d", expected, got, i)
		}
		var cmd tea.Cmd
		input, cmd = input.Update(tea.KeyMsg{Type: tea.KeyDown})
		_ = cmd
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
	if updated.focus != focusModeGenerate {
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
