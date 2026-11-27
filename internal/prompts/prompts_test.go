package prompts

import (
	"strings"
	"testing"
)

func TestBasePromptsAreNonEmpty(t *testing.T) {
	generate, err := Base(GeneratePrompt)
	if err != nil {
		t.Fatalf("base generate: %v", err)
	}
	if generate == "" {
		t.Fatal("expected generate prompt to be non-empty")
	}

	edit, err := Base(EditPrompt)
	if err != nil {
		t.Fatalf("base edit: %v", err)
	}
	if edit == "" {
		t.Fatal("expected edit prompt to be non-empty")
	}
}

func TestApplyPromptOverrideAppendAndReplace(t *testing.T) {
	base := "Base prompt"

	if got := Apply(base, Override{Append: "Extra guidance"}); got != "Base prompt\n\nExtra guidance" {
		t.Fatalf("unexpected appended prompt: %q", got)
	}

	if got := Apply(base, Override{Replace: "Replacement"}); got != "Replacement" {
		t.Fatalf("unexpected replaced prompt: %q", got)
	}

	if got := Apply(base, Override{Replace: "Replacement", Append: "Extra guidance"}); got != "Replacement\n\nExtra guidance" {
		t.Fatalf("unexpected combined prompt: %q", got)
	}
}

func TestBuildRejectsUnknownPromptName(t *testing.T) {
	if _, err := Build(Name("unknown_prompt"), nil); err == nil {
		t.Fatal("expected unknown prompt name to fail")
	}
}

func TestRenderUsesTemplatesAndOverrides(t *testing.T) {
	overrides := Overrides{
		ContinuePrompt: {
			Append: "\nStay consistent with prior terminology.",
		},
	}

	got, err := Render(ContinuePrompt, overrides, struct {
		HasDocument      bool
		ScopeInstruction string
		RoundLabel       string
	}{
		HasDocument:      true,
		ScopeInstruction: "Write 1 paragraph.",
		RoundLabel:       "Round 1 of 3",
	})
	if err != nil {
		t.Fatalf("render continue prompt: %v", err)
	}
	if !strings.Contains(got, "Write 1 paragraph.") || !strings.Contains(got, "Round 1 of 3") {
		t.Fatalf("expected rendered scope and round label, got %q", got)
	}
	if !strings.Contains(got, "Stay consistent with prior terminology.") {
		t.Fatalf("expected appended override text, got %q", got)
	}
}
