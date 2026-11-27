package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestJSONStreamExtractorStreamsContentAcrossArbitraryChunkSplits(t *testing.T) {
	full := `{"join_mode": "new_paragraph", "intent": "Describe the valley", "content": "Line 1 with \"quotes\" and \\slash\\.\nLine 2 with \u2014 dash and \uD83D\uDE80 rocket."}`

	// Feed 1 byte at a time to stress-test every escape & state boundary.
	ext := NewJSONStreamExtractor("content")
	var joinMode, intent string
	var content strings.Builder

	for i := 0; i < len(full); i++ {
		for _, ev := range ext.Feed(full[i : i+1]) {
			switch ev.Field {
			case "join_mode":
				if ev.Done {
					joinMode = ev.Value
				}
			case "intent":
				if ev.Done {
					intent = ev.Value
				}
			case "content":
				content.WriteString(ev.Delta)
			}
		}
	}

	if joinMode != "new_paragraph" {
		t.Fatalf("joinMode = %q, want new_paragraph", joinMode)
	}
	if intent != "Describe the valley" {
		t.Fatalf("intent = %q, want Describe the valley", intent)
	}
	wantContent := "Line 1 with \"quotes\" and \\slash\\.\nLine 2 with — dash and 🚀 rocket."
	if content.String() != wantContent {
		t.Fatalf("content = %q, want %q", content.String(), wantContent)
	}
}

func TestStreamStructuredGenerationExtractsFieldsAndStreamsLive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`{"join_mode":"inline_space",`,
			`"intent":"Finish thought",`,
			`"content":"And then `,
			`it worked.\nDone."}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "", time.Second)
	var joinMode, intent string
	var out strings.Builder
	err := client.StreamStructuredGeneration(
		context.Background(),
		[]Message{{Role: "user", Content: "go"}},
		"gen",
		map[string]any{"type": "object"},
		[]Message{{Role: "user", Content: "fallback"}},
		func(ev StreamEvent) error {
			if ev.JoinMode != "" {
				joinMode = ev.JoinMode
			}
			if ev.Intent != "" {
				intent = ev.Intent
			}
			out.WriteString(ev.Delta)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stream structured failed: %v", err)
	}
	if joinMode != "inline_space" || intent != "Finish thought" {
		t.Fatalf("unexpected metadata: join=%q intent=%q", joinMode, intent)
	}
	if out.String() != "And then it worked.\nDone." {
		t.Fatalf("unexpected content: %q", out.String())
	}
}

func TestStreamStructuredGenerationFallsBackOn400(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"response_format not supported with stream"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Plain fallback.\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "", time.Second)
	var out strings.Builder
	err := client.StreamStructuredGeneration(
		context.Background(),
		[]Message{{Role: "user", Content: "structured"}},
		"gen",
		map[string]any{"type": "object"},
		[]Message{{Role: "user", Content: "plain"}},
		func(ev StreamEvent) error {
			out.WriteString(ev.Delta)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if out.String() != "Plain fallback." || calls.Load() != 2 {
		t.Fatalf("unexpected fallback output=%q calls=%d", out.String(), calls.Load())
	}
}

func TestListModelsReturnsSortedIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"z-model"},{"id":"a-model"},{"id":"a-model"}]}`)
	}))
	defer server.Close()

	models, err := NewClient(server.URL, "", "", time.Second).ListModels(context.Background())
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(models) != 2 || models[0] != "a-model" || models[1] != "z-model" {
		t.Fatalf("unexpected models: %#v", models)
	}
}
