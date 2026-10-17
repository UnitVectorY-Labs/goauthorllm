package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/prompts"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestTUIGenerationRequestFromLocalConfig(t *testing.T) {
	var request struct {
		Model    string        `json:"model"`
		Messages []llm.Message `json:"messages"`
		Stream   bool          `json:"stream"`
	}
	requests := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		var payload json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		requests <- payload
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" A bird sang.\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	t.Chdir(t.TempDir())
	for _, name := range []string{"GOAUTHORLLM_BASE_URL", "OPENAI_BASE_URL", "GOAUTHORLLM_MODEL", "OPENAI_MODEL", "GOAUTHORLLM_GENERATION_MODEL"} {
		t.Setenv(name, "")
	}
	localConfig := fmt.Sprintf("base_url: %s/v1/\nmodel: gemma4-31b-it-q5kxl-instruct\ngenerate_prompt:\n  append: |\n    You are the author of content.\n", server.URL)
	if err := os.WriteFile(".goauthorllm", []byte(localConfig), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	m := usabilityModel(t)
	m.cfg = cfg
	m.generationClient = llm.NewClient(cfg.BaseURL, cfg.GenerationModel, "", time.Second)
	m.mode = workspaceGenerate
	m.frontMatter.SetValue("")
	body := "# Forest\n\nThe trees swayed.\n\n# Trail\n\nA traveler paused."
	m.editor.SetValue(body)
	m.prompt.SetValue("Write one more sentence.")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if cmd == nil || !m.generating {
		t.Fatalf("TUI did not start generation: %s", m.statusText)
	}
	for event := range m.generationCh {
		m.Update(streamMsg{id: m.generationID, event: event})
	}
	if m.statusLevel == "error" || !strings.HasSuffix(m.doc.Body, " A bird sang.") {
		t.Fatalf("generation failed: %s, body=%q", m.statusText, m.doc.Body)
	}
	if err := json.Unmarshal(<-requests, &request); err != nil {
		t.Fatal(err)
	}
	if request.Model != cfg.Model || !request.Stream || len(request.Messages) != 8 {
		t.Fatalf("unexpected request: %+v", request)
	}
	if request.Messages[2].Role != "assistant" || request.Messages[2].Content != "# Forest\n\nThe trees swayed." ||
		request.Messages[4].Role != "assistant" || request.Messages[4].Content != "# Trail\n\nA traveler paused." ||
		request.Messages[6].Role != "user" || request.Messages[6].Content != request.Messages[4].Content ||
		!strings.Contains(request.Messages[7].Content, "Write one more sentence.") {
		t.Fatalf("document context or guidance changed: %+v", request.Messages)
	}
	if !strings.HasSuffix(request.Messages[0].Content, "You are the author of content.") {
		t.Fatal("local system prompt append missing")
	}
	encoded, _ := json.MarshalIndent(request.Messages, "", "  ")
	t.Logf("TUI messages:\n%s", encoded)
}

func TestTUIErrorDetailsPreserveFullError(t *testing.T) {
	for _, size := range [][2]int{{32, 10}, {100, 30}} {
		for _, partial := range []bool{false, true} {
			m := usabilityModel(t)
			m.mode = workspaceGenerate
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.generating = true
			m.generationStarted = partial
			detail := strings.Repeat("long server error detail ", 100) + "UPSTREAM_FAILURE_END"
			m.Update(streamMsg{id: m.generationID, event: llm.StreamEvent{Err: fmt.Errorf("%s", detail)}})
			if !strings.Contains(m.lastError, detail) {
				t.Fatal("server error details were lost")
			}
			m.setStatus("Autosaved", "success")
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}, Alt: true})
			if !m.showError {
				t.Fatal("Alt+E did not open the saved error")
			}
			m.Update(tea.KeyMsg{Type: tea.KeyEnd})
			view := m.View()
			if !strings.Contains(strings.ReplaceAll(view, "\n", ""), "UPSTREAM_FAILURE_END") || lipgloss.Width(view) > size[0] || lipgloss.Height(view) != size[1] {
				t.Fatalf("error details were clipped: %s", view)
			}
			m.Update(tea.KeyMsg{Type: tea.KeyHome})
			if !strings.Contains(m.View(), "Generation") {
				t.Fatal("could not scroll back to the start")
			}
			m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			if m.showError || m.screen != screenWorkspace {
				t.Fatal("Esc did not return to workspace")
			}
		}
	}
}

// Run explicitly against an endpoint using only this synthetic document.
func TestLiveTUIGeneration(t *testing.T) {
	base := os.Getenv("GOAUTHORLLM_TEST_BASE_URL")
	if base == "" {
		t.Skip("set GOAUTHORLLM_TEST_BASE_URL and GOAUTHORLLM_TEST_MODEL")
	}
	m := usabilityModel(t)
	m.doc.Path = filepath.Join(t.TempDir(), "forest.md")
	m.cfg.MessageOverrides = prompts.Overrides{prompts.GeneratePrompt: {Append: "You are the author of content."}}
	m.generationClient = llm.NewClient(base, os.Getenv("GOAUTHORLLM_TEST_MODEL"), "", 90*time.Second)
	m.mode = workspaceGenerate
	m.frontMatter.SetValue("")
	m.editor.SetValue("# Forest\n\nThe trees swayed.\n\n# Trail\n\nA traveler paused.")
	m.prompt.SetValue("Write one short sentence to continue the story.")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if cmd == nil || !m.generating {
		t.Fatalf("generation did not start: %s", m.statusText)
	}
	t.Cleanup(m.cancelBusyRequest)
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	events := m.generationCh
	for {
		select {
		case event, ok := <-events:
			if !ok {
				if m.statusLevel == "error" {
					t.Fatal(m.lastError)
				}
				t.Logf("%s\n%s", m.statusText, m.doc.Body)
				return
			}
			m.Update(streamMsg{id: m.generationID, event: event})
		case <-deadline.C:
			t.Fatal("generation timed out")
		}
	}
}
