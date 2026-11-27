package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

func TestBuildGenerationMessagesIsLeanTwoMessages(t *testing.T) {
	msgs, err := BuildGenerationMessages(GenerationRequest{
		Body:              "# Title\n\nFirst para.\n\n## Section 2\n\nSecond para.",
		ProjectRules:      "Be concise.",
		DocumentSystemMsg: "Audience is Go developers.",
		Guidance:          "Cover benchmarks.",
		Scope:             ScopeParagraph,
	}, true)
	if err != nil {
		t.Fatalf("build messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected lean 2-message prompt, got %d messages", len(msgs))
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "Be concise.") || !strings.Contains(msgs[0].Content, "Audience is Go developers.") {
		t.Fatalf("system prompt missing rules: %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[1].Content, "<document>") || !strings.Contains(msgs[1].Content, "Cover benchmarks.") {
		t.Fatalf("user prompt missing document or guidance: %q", msgs[1].Content)
	}
}

func TestPlanRoundsReturnsRequestedSteps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"steps\":[{\"title\":\"Setup\",\"goal\":\"Explain setup\"},{\"title\":\"Benchmarks\",\"goal\":\"Add numbers\"},{\"title\":\"Wrap-up\",\"goal\":\"Summarize\"}]}"}}]}`)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test", "", time.Second)
	steps, err := PlanRounds(context.Background(), client, GenerationRequest{
		Body:  "# Guide\n\nIntro.",
		Scope: ScopeSection,
	}, 3)
	if err != nil {
		t.Fatalf("plan rounds: %v", err)
	}
	if len(steps) != 3 || steps[1].Title != "Benchmarks" {
		t.Fatalf("unexpected steps: %#v", steps)
	}
}

func TestFetchEditBatchResolvesLineAnchoredEditsAndFiltersInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"assessment\":\"Fixed typo\",\"edits\":[{\"reason\":\"Fix typo\",\"target_line\":3,\"action\":\"replace\",\"old_text\":\"teh\",\"new_text\":\"the\"},{\"reason\":\"Bad\",\"target_line\":99,\"action\":\"replace\",\"old_text\":\"nonexistent\",\"new_text\":\"x\"}],\"more_rounds_needed\":false}"}}]}`)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test", "", time.Second)
	res := FetchEditBatch(context.Background(), client, EditRequest{
		Body:      "# Draft\n\nFix teh typo.\n",
		BatchSize: 5,
	})
	if res.Err != nil {
		t.Fatalf("fetch edit batch: %v", res.Err)
	}
	if len(res.Edits) != 1 || res.Edits[0].NewText != "the" || res.Edits[0].Line != 3 {
		t.Fatalf("expected 1 resolved edit on line 3, got %#v", res.Edits)
	}
}
