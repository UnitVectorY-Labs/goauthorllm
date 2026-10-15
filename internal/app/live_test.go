package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

// Opt in explicitly; live tests only send these synthetic documents.
func TestLiveDirectedEditing(t *testing.T) {
	base := os.Getenv("GOAUTHORLLM_TEST_BASE_URL")
	if base == "" {
		t.Skip("set GOAUTHORLLM_TEST_BASE_URL and GOAUTHORLLM_TEST_MODEL")
	}
	client := llm.NewClient(base, os.Getenv("GOAUTHORLLM_TEST_MODEL"), "", 90*time.Second)
	for _, tc := range []struct {
		name, body, instructions string
		check                    func(string) bool
	}{
		{"rename", "# Draft\n\nAlice walks home.\n\nBob greets Alice.\n", "Rename Alice to Maya everywhere. Preserve everything else.", func(s string) bool { return !strings.Contains(s, "Alice") && strings.Count(s, "Maya") == 2 }},
		{"delete", "# Draft\n\nKeep this paragraph.\n\nRemove this obsolete paragraph.\n\nKeep this conclusion.\n", "Delete the paragraph 'Remove this obsolete paragraph.' Preserve all other text.", func(s string) bool {
			return !strings.Contains(s, "obsolete") && strings.Contains(s, "Keep this paragraph.") && strings.Contains(s, "Keep this conclusion.")
		}},
		{"repeated", "# Draft\n\nAlice says hello.\n\nAlice says hello.\n", "Rename Alice to Maya everywhere. Preserve everything else.", func(s string) bool { return !strings.Contains(s, "Alice") && strings.Count(s, "Maya") == 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			result := fetchEditSuggestion(ctx, client, tc.body, "", nil, nil, editKindDirected, tc.instructions, 5)
			if result.Err != nil {
				t.Fatal(result.Err)
			}
			body := tc.body
			for _, s := range result.Suggestions {
				var ok bool
				body, ok = document.ReplaceUnique(body, s.OldText, s.NewText)
				if !ok {
					t.Fatalf("stale suggestion: %+v", s)
				}
			}
			if !tc.check(body) {
				t.Fatalf("task incomplete: %q; result=%+v", body, result)
			}
			t.Logf("validated and applied %d suggestions", len(result.Suggestions))
		})
	}
}
