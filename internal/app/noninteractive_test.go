package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
)

func TestRunNonInteractiveGenerationWritesAndLogsContent(t *testing.T) {
	server := chatServer(t, func(_ int32) string { return "More text." })
	defer server.Close()
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte("# Draft\nExisting text."), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	cfg := nonInteractiveTestConfig(server.URL, path)
	cfg.Mode = "generate"
	cfg.Submode = "continue"

	var output bytes.Buffer
	result, err := RunNonInteractive(cfg, &output)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Changed != 1 || !strings.Contains(output.String(), "More text.") {
		t.Fatalf("unexpected result/output: %#v %q", result, output.String())
	}
	written, _ := os.ReadFile(path)
	if !strings.Contains(string(written), "Existing text. More text.") {
		t.Fatalf("generated content was not saved: %q", written)
	}
}

func TestRunNonInteractiveApproveAllAppliesAndLogsEdit(t *testing.T) {
	server := chatServer(t, func(_ int32) string {
		return `{"suggestions":[{"old_text":"teh","new_text":"the"}],"remaining_rounds":0}`
	})
	defer server.Close()
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte("Fix teh typo.\n"), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	cfg := nonInteractiveTestConfig(server.URL, path)
	cfg.Mode = "edit"
	cfg.Submode = "copy"
	cfg.Approval = "approve-all"

	var output bytes.Buffer
	result, err := RunNonInteractive(cfg, &output)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Changed != 1 || !strings.Contains(output.String(), "--- old\nteh\n+++ new\nthe") {
		t.Fatalf("unexpected result/output: %#v %q", result, output.String())
	}
	written, _ := os.ReadFile(path)
	if string(written) != "Fix the typo.\n" {
		t.Fatalf("unexpected document: %q", written)
	}
}

func TestRunNonInteractiveLLMApprovedReturnsNoChangesWhenRejected(t *testing.T) {
	server := chatServer(t, func(request int32) string {
		if request == 1 {
			return `{"suggestions":[{"old_text":"word","new_text":"term"}],"remaining_rounds":0}`
		}
		return `{"approve":false}`
	})
	defer server.Close()
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte("A word.\n"), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	cfg := nonInteractiveTestConfig(server.URL, path)
	cfg.Mode = "edit"
	cfg.Submode = "copy"
	cfg.Approval = "llm-approved"

	var output bytes.Buffer
	result, err := RunNonInteractive(cfg, &output)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Changed != 0 || output.Len() != 0 {
		t.Fatalf("unexpected result/output: %#v %q", result, output.String())
	}
}

func TestRunNonInteractiveHonorsMaxEdits(t *testing.T) {
	var requests atomic.Int32
	server := chatServer(t, func(request int32) string {
		requests.Store(request)
		return `{"suggestions":[{"old_text":"one","new_text":"ONE"},{"old_text":"two","new_text":"TWO"}],"remaining_rounds":1}`
	})
	defer server.Close()
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte("one two\n"), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	cfg := nonInteractiveTestConfig(server.URL, path)
	cfg.Mode = "edit"
	cfg.Submode = "copy"
	cfg.Approval = "approve-all"
	cfg.CopyEditBatchSize = 2
	cfg.MaxEdits = 1

	result, err := RunNonInteractive(cfg, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	written, _ := os.ReadFile(path)
	if result.Changed != 1 || string(written) != "ONE two\n" || requests.Load() != 1 {
		t.Fatalf("maximum edit count not honored: %#v %q requests=%d", result, written, requests.Load())
	}
}

func nonInteractiveTestConfig(serverURL, path string) config.Config {
	return config.Config{
		FilePath:              path,
		BaseURL:               serverURL + "/v1",
		Model:                 "test-model",
		GenerationModel:       "test-model",
		EditingModel:          "test-model",
		Timeout:               time.Second,
		CopyEditBatchSize:     1,
		DirectedEditBatchSize: 10,
	}
}

func chatServer(t *testing.T, content func(int32) string) *httptest.Server {
	t.Helper()
	var requests atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		response := map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": content(requests.Add(1))},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}
