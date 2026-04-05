package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStructuredChatSendsJsonSchemaAndReturnsContent(t *testing.T) {
	var received struct {
		Model          string    `json:"model"`
		Messages       []Message `json:"messages"`
		ResponseFormat struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Name   string         `json:"name"`
				Schema map[string]any `json:"schema"`
				Strict bool           `json:"strict"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"match\":\"foo\",\"replace\":\"bar\"}"}}]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "test-key", time.Second)
	content, err := client.StructuredChat(context.Background(), []Message{{Role: "user", Name: "instructions", Content: "fix this"}}, "edit_suggestion", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"match":   map[string]any{"type": "string"},
			"replace": map[string]any{"type": "string"},
		},
		"required":             []string{"match", "replace"},
		"additionalProperties": false,
	})
	if err != nil {
		t.Fatalf("structured chat failed: %v", err)
	}

	if content != `{"match":"foo","replace":"bar"}` {
		t.Fatalf("unexpected structured content: %q", content)
	}
	if received.ResponseFormat.Type != "json_schema" {
		t.Fatalf("unexpected response format type: %q", received.ResponseFormat.Type)
	}
	if received.ResponseFormat.JSONSchema.Name != "edit_suggestion" {
		t.Fatalf("unexpected schema name: %q", received.ResponseFormat.JSONSchema.Name)
	}
	if !received.ResponseFormat.JSONSchema.Strict {
		t.Fatalf("expected strict schema mode")
	}
	if got := received.Model; got != "test-model" {
		t.Fatalf("unexpected model: %q", got)
	}
}

func TestStructuredChatRejectsEmptySchemaName(t *testing.T) {
	client := NewClient("http://example.com", "test-model", "", time.Second)
	_, err := client.StructuredChat(context.Background(), nil, "   ", map[string]any{"type": "object"})
	if err == nil {
		t.Fatalf("expected error for empty schema name")
	}
	if !strings.Contains(err.Error(), "schema name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStreamChatHandlesSSEStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "", time.Second)
	var collected []string
	err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) error {
		if event.Delta != "" {
			collected = append(collected, event.Delta)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream chat failed: %v", err)
	}
	if got := strings.Join(collected, ""); got != "Hello world" {
		t.Fatalf("unexpected collected stream: %q", got)
	}
}

func TestStreamChatHandlesNonStreamingFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Complete response"}}]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "", time.Second)
	var gotDelta string
	var gotDone bool
	err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) error {
		if event.Delta != "" {
			gotDelta = event.Delta
		}
		if event.Done {
			gotDone = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream chat failed: %v", err)
	}
	if gotDelta != "Complete response" {
		t.Fatalf("unexpected delta: %q", gotDelta)
	}
	if !gotDone {
		t.Fatal("expected done event")
	}
}

func TestStreamChatHandlesErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "", time.Second)
	err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStreamChatOmitsAuthHeaderWhenKeyEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected no auth header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "", time.Second)
	err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
