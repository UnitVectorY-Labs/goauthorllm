package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

// Opt in explicitly via env vars:
//
//	GOAUTHORLLM_TEST_BASE_URL=... GOAUTHORLLM_TEST_MODEL=... go test ./internal/app -run TestLive -v -count=1
func liveTestClient(t *testing.T) *llm.Client {
	t.Helper()
	base := os.Getenv("GOAUTHORLLM_TEST_BASE_URL")
	model := os.Getenv("GOAUTHORLLM_TEST_MODEL")
	if base == "" || model == "" {
		t.Skip("set GOAUTHORLLM_TEST_BASE_URL and GOAUTHORLLM_TEST_MODEL")
	}
	return llm.NewClient(base, model, os.Getenv("GOAUTHORLLM_TEST_API_KEY"), 90*time.Second)
}

func TestLiveListModels(t *testing.T) {
	client := liveTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}
	t.Logf("endpoint returned %d models", len(models))
}

func TestLiveStructuredStreamingGenerationScopes(t *testing.T) {
	client := liveTestClient(t)

	for _, tc := range []struct {
		name  string
		scope engine.Scope
		body  string
	}{
		{"sentence", engine.ScopeSentence, "# Mountain Trail\n\nThe morning fog lifted slowly from the pine ridge."},
		{"paragraph", engine.ScopeParagraph, "# Mountain Trail\n\nThe morning fog lifted slowly from the pine ridge. We tightened our packs and stepped onto the narrow switchback."},
		{"section", engine.ScopeSection, "# Mountain Trail\n\n## Departure\n\nWe left the trailhead at dawn with clear skies ahead."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()

			var joinMode document.JoinMode
			var intent string
			var streamed strings.Builder
			chunks := 0

			err := engine.StreamRound(ctx, client, engine.GenerationRequest{
				Body:                 tc.body,
				Scope:                tc.scope,
				StructuredGeneration: true,
			}, func(ev llm.StreamEvent) error {
				if ev.JoinMode != "" {
					joinMode = document.JoinMode(ev.JoinMode)
				}
				if ev.Intent != "" {
					intent = ev.Intent
				}
				if ev.Delta != "" {
					chunks++
					streamed.WriteString(ev.Delta)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("StreamRound(%s): %v", tc.name, err)
			}
			text := strings.TrimSpace(streamed.String())
			if text == "" {
				t.Fatalf("expected non-empty generated text")
			}
			joined := tc.body + document.NormalizeJoinStart(tc.body, streamed.String(), joinMode, string(tc.scope))
			if tc.scope == engine.ScopeSection && !strings.Contains(text, "#") {
				t.Fatalf("expected heading in section generation, got %q", text)
			}
			t.Logf("scope=%s join=%q intent=%q chunks=%d len=%d\n--- joined ---\n%s", tc.name, joinMode, intent, chunks, len(text), joined)
		})
	}
}

func TestLiveMultiRoundSections(t *testing.T) {
	client := liveTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	body := "# Caching Primer\n\nCaching reduces latency by keeping frequently accessed data close to callers."
	req := engine.GenerationRequest{
		Body:                 body,
		Guidance:             "Keep each section brief (1 short paragraph per section).",
		Scope:                engine.ScopeSection,
		StructuredGeneration: true,
	}

	steps, err := engine.PlanRounds(ctx, client, req, 3)
	if err != nil {
		t.Fatalf("PlanRounds: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 planned steps, got %d: %#v", len(steps), steps)
	}

	for i, step := range steps {
		r := req
		r.Body = body
		r.RoundIndex = i + 1
		r.RoundTotal = 3
		s := step
		r.RoundStep = &s

		joinMode := document.JoinAuto
		var out strings.Builder
		if err := engine.StreamRound(ctx, client, r, func(ev llm.StreamEvent) error {
			if ev.JoinMode != "" {
				joinMode = document.JoinMode(ev.JoinMode)
			}
			out.WriteString(ev.Delta)
			return nil
		}); err != nil {
			t.Fatalf("round %d failed: %v", i+1, err)
		}
		body += document.NormalizeJoinStart(body, out.String(), joinMode, string(engine.ScopeSection))
	}

	sections := document.SplitSections(body)
	if len(sections) < 4 {
		t.Fatalf("expected at least 4 sections (1 intro + 3 generated), got %d:\n%s", len(sections), body)
	}
	t.Logf("multi-round generated %d total sections:\n%s", len(sections), body)
}

func TestLiveDirectedEditing(t *testing.T) {
	client := liveTestClient(t)
	for _, tc := range []struct {
		name, body, instructions string
		check                    func(string) bool
	}{
		{
			"rename",
			"# Draft\n\nAlice walks home.\n\nBob greets Alice.\n",
			"Rename Alice to Maya everywhere. Preserve everything else.",
			func(s string) bool { return !strings.Contains(s, "Alice") && strings.Count(s, "Maya") == 2 },
		},
		{
			"delete",
			"# Draft\n\nKeep this paragraph.\n\nRemove this obsolete paragraph.\n\nKeep this conclusion.\n",
			"Delete the paragraph 'Remove this obsolete paragraph.' Preserve all other text cleanly with no triple blank lines.",
			func(s string) bool {
				return !strings.Contains(s, "obsolete") &&
					strings.Contains(s, "Keep this paragraph.") &&
					strings.Contains(s, "Keep this conclusion.") &&
					!strings.Contains(s, "\n\n\n")
			},
		},
		{
			"repeated",
			"# Draft\n\nAlice says hello.\n\nAlice says hello.\n",
			"Rename Alice to Maya everywhere. Preserve everything else.",
			func(s string) bool { return !strings.Contains(s, "Alice") && strings.Count(s, "Maya") == 2 },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()

			body := tc.body
			var hist []engine.EditHistoryEntry
			for round := 0; round < 3; round++ {
				res := engine.FetchEditBatch(ctx, client, engine.EditRequest{
					Body:         body,
					Instructions: tc.instructions,
					BatchSize:    5,
					History:      hist,
				})
				if res.Err != nil {
					t.Fatal(res.Err)
				}
				if len(res.Edits) == 0 {
					break
				}
				for _, e := range res.Edits {
					var ok bool
					body, ok = document.ApplyResolvedEdit(body, e)
					if !ok {
						t.Fatalf("failed to apply edit: %+v", e)
					}
					hist = append(hist, engine.EditHistoryEntry{
						Action: "approved-all", Line: e.Line, OldText: e.OldText, NewText: e.NewText, Reason: e.Reason,
					})
				}
				if !res.MoreRoundsNeeded || tc.check(body) {
					break
				}
			}
			if !tc.check(body) {
				t.Fatalf("task incomplete: %q", body)
			}
			t.Logf("result=%q", body)
		})
	}
}

func TestLiveCopyEditingWithCustomRules(t *testing.T) {
	client := liveTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	body := "# Notes\n\nThe servr recieve three packets every seconds.\n"
	res := engine.FetchEditBatch(ctx, client, engine.EditRequest{
		Body:          body,
		CopyEditRules: "Fix spelling and subject-verb agreement while keeping technical terms intact.",
		BatchSize:     5,
	})
	if res.Err != nil {
		t.Fatalf("FetchEditBatch: %v", res.Err)
	}
	if len(res.Edits) == 0 {
		t.Fatalf("expected at least 1 copy edit for typos, got none (assessment=%q)", res.Assessment)
	}
	updated := body
	for _, e := range res.Edits {
		var ok bool
		updated, ok = document.ApplyResolvedEdit(updated, e)
		if !ok {
			t.Fatalf("failed to apply edit: %+v", e)
		}
	}
	if strings.Contains(updated, "servr") || strings.Contains(updated, "recieve") {
		t.Fatalf("expected typos fixed, got %q", updated)
	}
	t.Logf("copy edited (%d edits): %q", len(res.Edits), updated)
}
