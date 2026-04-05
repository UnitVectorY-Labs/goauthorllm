package prompts

import "testing"

func TestBasePromptsUseDocumentLanguage(t *testing.T) {
	generate, err := Base(GeneratePrompt)
	if err != nil {
		t.Fatalf("base generate: %v", err)
	}
	if generate == "" {
		t.Fatal("expected generate prompt to be non-empty")
	}
	if want := "documents"; want != "" && !containsFold(generate, want) {
		t.Fatalf("expected generate prompt to mention %q, got %q", want, generate)
	}

	edit, err := Base(EditPrompt)
	if err != nil {
		t.Fatalf("base edit: %v", err)
	}
	if edit == "" {
		t.Fatal("expected edit prompt to be non-empty")
	}
	if want := "copy editing documents"; !containsFold(edit, want) {
		t.Fatalf("expected edit prompt to mention %q, got %q", want, edit)
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
		SectionLabel string
		HasExcerpt   bool
	}{
		SectionLabel: "# Intro",
		HasExcerpt:   true,
	})
	if err != nil {
		t.Fatalf("render continue prompt: %v", err)
	}
	if !containsFold(got, "inside # Intro") {
		t.Fatalf("expected rendered section label, got %q", got)
	}
	if !containsFold(got, "Stay consistent with prior terminology.") {
		t.Fatalf("expected appended override text, got %q", got)
	}
}

func containsFold(value, substr string) bool {
	value = lower(value)
	substr = lower(substr)
	return len(substr) == 0 || indexOf(value, substr) >= 0
}

func lower(value string) string {
	out := []rune(value)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + ('a' - 'A')
		}
	}
	return string(out)
}

func indexOf(value, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i+len(substr) <= len(value); i++ {
		if value[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
