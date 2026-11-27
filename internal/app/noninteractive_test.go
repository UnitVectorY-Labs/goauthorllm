package app

import (
	"bytes"
	"encoding/json/v2"
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
	server := chatServer(t, func(_ int32) string {
		return `{"join_mode":"inline_space","intent":"Add follow-up","content":"More text."}`
	})
	defer server.Close()
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte("# Draft\nExisting text."), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	cfg := nonInteractiveTestConfig(server.URL, path)
	cfg.Mode = "generate"
	cfg.Submode = "sentence"
	cfg.StructuredGeneration = true

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

func TestRunNonInteractiveMultiRoundSections(t *testing.T) {
	server := chatServer(t, func(req int32) string {
		switch req {
		case 1:
			return `{"steps":[{"title":"Alpha","goal":"Write Alpha"},{"title":"Beta","goal":"Write Beta"}]}`
		case 2:
			return `{"join_mode":"new_section","intent":"Alpha","content":"## Alpha\n\nAlpha body."}`
		default:
			return `{"join_mode":"new_section","intent":"Beta","content":"## Beta\n\nBeta body."}`
		}
	})
	defer server.Close()
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte("# Doc\n\nIntro.\n"), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	cfg := nonInteractiveTestConfig(server.URL, path)
	cfg.Mode = "generate"
	cfg.Submode = "section"
	cfg.Rounds = 2
	cfg.StructuredGeneration = true

	var output bytes.Buffer
	result, err := RunNonInteractive(cfg, &output)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Changed != 2 {
		t.Fatalf("expected 2 changed rounds, got %d", result.Changed)
	}
	written, _ := os.ReadFile(path)
	if !strings.Contains(string(written), "## Alpha") || !strings.Contains(string(written), "## Beta") {
		t.Fatalf("expected both sections saved, got %q", written)
	}
}

func TestRunNonInteractiveApproveAllAppliesAndLogsEdit(t *testing.T) {
	server := chatServer(t, func(_ int32) string {
		return `{"assessment":"Fixed typo","edits":[{"reason":"Typo","target_line":1,"action":"replace","old_text":"teh","new_text":"the"}],"more_rounds_needed":false}`
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

func TestRunNonInteractiveHonorsMaxEdits(t *testing.T) {
	var requests atomic.Int32
	server := chatServer(t, func(request int32) string {
		requests.Store(request)
		return `{"assessment":"Two fixes","edits":[{"reason":"One","target_line":1,"action":"replace","old_text":"one","new_text":"ONE"},{"reason":"Two","target_line":1,"action":"replace","old_text":"two","new_text":"TWO"}],"more_rounds_needed":true}`
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
		Rounds:                1,
		CopyEditBatchSize:     5,
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
		if err := json.MarshalWrite(w, response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}
