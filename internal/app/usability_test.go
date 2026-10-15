package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/history"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func usabilityModel(t *testing.T) *Model {
	t.Helper()
	value, err := NewModel(config.Config{Timeout: time.Second, DirectedEditBatchSize: 5}, nil)
	if err != nil {
		t.Fatal(err)
	}
	value.doc = &document.Document{Path: filepath.Join(t.TempDir(), "draft.md"), Body: "Original\n"}
	value.pendingPath = value.doc.Path
	value.editor.SetValue(value.doc.Body)
	value.screen = screenWorkspace
	value.focus = focusEditor
	return &value
}

func TestGuidanceEnterAndMultiline(t *testing.T) {
	m := usabilityModel(t)
	m.mode = workspaceGenerate
	m.workspaceTab = 1
	m.focus = focusPrompt
	m.syncFocus()
	m.generationClient = llm.NewClient("http://127.0.0.1:1", "test", "", time.Second)
	m.prompt.SetValue("Write more")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	if !strings.Contains(m.prompt.Value(), "\n") || m.generating {
		t.Fatal("Alt+Enter should insert a newline")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !m.generating || m.workspaceTab != 0 {
		t.Fatal("Enter should start generation and show document")
	}
	m.cancelBusyRequest()
	entries, err := history.Load(m.doc.Path, "generation")
	if err != nil || len(entries) != 1 {
		t.Fatalf("guidance was not saved: %v %v", entries, err)
	}
}

func TestDirectedEnterSavesAndRecallRestoresDraft(t *testing.T) {
	m := usabilityModel(t)
	m.screen = screenEditOptions
	m.focus = focusEditInstructions
	m.edit.kind = editKindDirected
	m.syncFocus()
	m.edit.instructions.SetValue("First\nrequest")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenApprovalPicker {
		t.Fatal("Enter should advance from instructions")
	}
	m.screen = screenEditOptions
	m.focus = focusEditInstructions
	m.edit.instructions.SetValue("Unsubmitted draft")
	m.recallInstruction(true)
	if m.edit.instructions.Value() != "First\nrequest" {
		t.Fatal("saved instructions not recalled")
	}
	m.recallInstruction(false)
	if m.edit.instructions.Value() != "Unsubmitted draft" {
		t.Fatal("draft lost while browsing history")
	}
}

func TestDeleteToEndUnicodeWrappedAndUndo(t *testing.T) {
	m := usabilityModel(t)
	m.editor.SetWidth(12)
	body := "prefix\n你好 world with more text\nremove this\nand this"
	m.editor.SetValue(body)
	m.doc.SetBody(body)
	m.editor.Focus()
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	m.editor.CursorDown()
	m.editor.SetCursor(8)
	m.deleteToEnd()
	if m.doc.Body != "prefix\n你好 world" {
		t.Fatalf("unexpected deletion: %q", m.doc.Body)
	}
	m.undoBody()
	if m.doc.Body != body {
		t.Fatal("undo did not restore full passage")
	}
}

func TestPhoneResizeKeepsActionsAndCursorVisible(t *testing.T) {
	for _, size := range [][2]int{{24, 8}, {32, 10}, {40, 16}, {65, 24}, {70, 25}, {70, 26}, {100, 30}} {
		for _, screen := range []screenState{screenWorkspace, screenEditOptions, screenChooser, screenModePicker, screenApprovalPicker} {
			for _, mode := range []workspaceMode{workspaceGenerate, workspaceDocument, workspaceEdit} {
				t.Run(fmt.Sprint(size, screen, mode), func(t *testing.T) {
					m := usabilityModel(t)
					m.screen = screen
					m.mode = mode
					m.edit.kind = editKindDirected
					m.chooser.files = []string{"one.md", "two.md"}
					m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
					view := m.View()
					if lipgloss.Height(view) != size[1] || lipgloss.Width(view) > size[0] {
						t.Fatalf("frame exceeds terminal: %dx%d", lipgloss.Width(view), lipgloss.Height(view))
					}
					for _, button := range m.layout.buttons {
						if button.Rect.y1 >= size[1] {
							t.Fatalf("button %s is offscreen at row %d", button.Action, button.Rect.y1)
						}
					}
					if screen == screenWorkspace && mode == workspaceGenerate && m.layout.editor.y2 >= size[1] {
						t.Fatal("editor extends below screen")
					}
				})
			}
		}
	}
}

func TestInvalidBatchRetriesInsteadOfReportingComplete(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := `{"remaining_rounds":0}`
		if calls > 1 {
			content = `{"suggestions":[{"old_text":"old","new_text":"new"}],"remaining_rounds":0}`
		}
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, content)
	}))
	defer server.Close()
	result := fetchEditSuggestion(context.Background(), llm.NewClient(server.URL, "test", "", time.Second), "old", "", nil, nil, editKindDirected, "replace old with new", 5)
	if result.Err != nil || len(result.Suggestions) != 1 || calls != 2 {
		t.Fatalf("invalid response was not retried: %+v calls=%d", result, calls)
	}
}

func TestDecodeEditBatchRejectsMissingFields(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"suggestions":null,"remaining_rounds":0}`, `{"suggestions":[],"remaining_rounds":-1}`, `{"suggestions":[{"old_text":"old"}],"remaining_rounds":0}`} {
		if _, err := decodeEditBatch(raw, 5); err == nil {
			t.Fatalf("accepted invalid response: %s", raw)
		}
	}
	if batch, err := decodeEditBatch("```json\n{\"suggestions\":[],\"remaining_rounds\":0}\n```", 5); err != nil || len(batch.Suggestions) != 0 {
		t.Fatalf("valid completion rejected: %v", err)
	}
}

func TestLargeSuggestionCanScrollWithoutHidingActions(t *testing.T) {
	for _, size := range [][2]int{{32, 10}, {100, 30}} {
		m := usabilityModel(t)
		m.mode = workspaceEdit
		m.focus = focusWorkspaceTabs
		m.edit.suggestion = &editSuggestion{OldText: strings.Repeat("old paragraph\n", 40), NewText: "replacement at bottom"}
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.View()
		for _, button := range m.layout.buttons {
			if button.Rect.y1 >= size[1] {
				t.Fatalf("large suggestion hid %s", button.Action)
			}
		}
		for range 100 {
			m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
		}
		if view := m.View(); !strings.Contains(view, "replacement at bottom") {
			t.Fatalf("could not scroll to replacement: %s", view)
		}
	}
}

func TestResizeWhileTypingKeepsActivePaneVisible(t *testing.T) {
	m := usabilityModel(t)
	m.mode = workspaceGenerate
	m.workspaceTab = 1
	m.focus = focusPrompt
	m.syncFocus()
	m.prompt.SetValue(strings.Repeat("Long guidance line\n", 20))
	for _, size := range [][2]int{{100, 40}, {32, 10}, {40, 16}, {70, 25}, {100, 30}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.View()
		if m.layout.prompt.y2 >= size[1] {
			t.Fatal("guidance pane is offscreen after resize")
		}
		for _, button := range m.layout.buttons {
			if button.Rect.y1 >= size[1] {
				t.Fatalf("guidance hid %s", button.Action)
			}
		}
		if !strings.Contains(m.prompt.Value(), "Long guidance line") {
			t.Fatal("resize lost input")
		}
	}
}

func TestUndoCanceledGenerationThenManualTyping(t *testing.T) {
	m := usabilityModel(t)
	original := m.doc.Body
	m.pushUndo(original)
	m.generating = true
	m.applyGenerationDelta("Generated addition\nwith more text")
	m.handleStreamMsg(streamMsg{id: m.generationID, event: llm.StreamEvent{Err: context.Canceled}})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if m.doc.Body != original {
		t.Fatal("partial generation was not undone")
	}
	m.syncFocus()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("manual text")})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if m.doc.Body != original {
		t.Fatal("typing after undo was not undoable")
	}
}

func TestRecallPreservesNewDraftAfterReturningFromHistory(t *testing.T) {
	m := usabilityModel(t)
	m.focus = focusPrompt
	m.persistInstruction("generation", "saved")
	m.prompt.SetValue("draft one")
	m.recallInstruction(true)
	m.recallInstruction(false)
	m.prompt.SetValue("draft two")
	m.recallInstruction(true)
	m.recallInstruction(false)
	if m.prompt.Value() != "draft two" {
		t.Fatal("history recall lost the updated draft")
	}
}

func TestDirectedWhitespaceDeletionIsValid(t *testing.T) {
	suggestions, feedback, err := validateDirectedSuggestions(context.Background(), nil, "one\n\n\nthree", []editSuggestion{{OldText: "\n\n\n", NewText: ""}}, nil)
	if err != nil || feedback != "" || len(suggestions) != 1 {
		t.Fatalf("whitespace deletion rejected: %s %v", feedback, err)
	}
}
