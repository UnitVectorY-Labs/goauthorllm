package document

import (
	"strings"
	"testing"
)

func TestNormalizeJoinStartHandlesAllModes(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		chunk    string
		joinMode JoinMode
		scope    string
		want     string
	}{
		{"empty doc", "", "\n\n# Intro\n", JoinNewSection, "section", "# Intro\n"},
		{"inline space after sentence", "Hello world.", "Next sentence.", JoinInlineSpace, "sentence", " Next sentence."},
		{"inline space skips when body ends with space", "Hello world. ", " Next sentence.", JoinInlineSpace, "sentence", "Next sentence."},
		{"inline space after heading forces blank line", "# Title", "Opening line.", JoinInlineSpace, "sentence", "\n\nOpening line."},
		{"inline direct punctuation", "Hello", ", world!", JoinInlineDirect, "sentence", ", world!"},
		{"new paragraph after single newline", "Para one.\n", "Para two.", JoinNewParagraph, "paragraph", "\nPara two."},
		{"new paragraph after no newline", "Para one.", "\nPara two.", JoinNewParagraph, "paragraph", "\n\nPara two."},
		{"new paragraph after blank line", "Para one.\n\n", "\n\nPara two.", JoinNewParagraph, "paragraph", "Para two."},
		{"section forces blank line", "End of section.", "## Next\n\nBody.", JoinAuto, "section", "\n\n## Next\n\nBody."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeJoinStart(tc.body, tc.chunk, tc.joinMode, tc.scope)
			if got != tc.want {
				t.Fatalf("NormalizeJoinStart(%q, %q, %q, %q) = %q, want %q", tc.body, tc.chunk, tc.joinMode, tc.scope, got, tc.want)
			}
		})
	}
}

func TestResolveEditDisambiguatesRepeatedTextByTargetLine(t *testing.T) {
	body := "# Draft\n\nAlice says hello.\n\nBob waves.\n\nAlice says hello.\n"
	edit, err := ResolveEdit(body, EditSpec{
		TargetLine: 7,
		Action:     ActionReplace,
		OldText:    "Alice says hello.",
		NewText:    "Maya says hello.",
		Reason:     "Rename second occurrence",
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if edit.Line != 7 {
		t.Fatalf("expected line 7, got %d", edit.Line)
	}
	updated, ok := ApplyResolvedEdit(body, edit)
	if !ok {
		t.Fatal("apply failed")
	}
	want := "# Draft\n\nAlice says hello.\n\nBob waves.\n\nMaya says hello.\n"
	if updated != want {
		t.Fatalf("updated = %q, want %q", updated, want)
	}
}

func TestResolveEditDeletesWholeParagraphWithoutLeavingTripleNewlines(t *testing.T) {
	body := "# Draft\n\nKeep first.\n\nRemove this paragraph.\n\nKeep last.\n"
	edit, err := ResolveEdit(body, EditSpec{
		TargetLine: 5,
		Action:     ActionDelete,
		OldText:    "Remove this paragraph.",
		NewText:    "",
		Reason:     "Drop obsolete paragraph",
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	updated, ok := ApplyResolvedEdit(body, edit)
	if !ok {
		t.Fatal("apply failed")
	}
	want := "# Draft\n\nKeep first.\n\nKeep last.\n"
	if updated != want {
		t.Fatalf("updated = %q, want %q", updated, want)
	}
}

func TestResolveEditStripsAccidentalLineNumberPrefixes(t *testing.T) {
	body := "# Title\n\nFix teh typo.\n"
	edit, err := ResolveEdit(body, EditSpec{
		TargetLine: 3,
		Action:     ActionReplace,
		OldText:    "3| Fix teh typo.",
		NewText:    "3| Fix the typo.",
		Reason:     "Typo",
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	updated, ok := ApplyResolvedEdit(body, edit)
	if !ok || updated != "# Title\n\nFix the typo.\n" {
		t.Fatalf("unexpected updated: %q (ok=%v)", updated, ok)
	}
}

func TestResolveEditInsertAfterAddsCleanSpacing(t *testing.T) {
	body := "# Title\n\nFirst sentence.\n"
	edit, err := ResolveEdit(body, EditSpec{
		TargetLine: 3,
		Action:     ActionInsertAfter,
		OldText:    "First sentence.",
		NewText:    "Second sentence.",
		Reason:     "Add follow-up",
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	updated, ok := ApplyResolvedEdit(body, edit)
	if !ok || updated != "# Title\n\nFirst sentence. Second sentence.\n" {
		t.Fatalf("unexpected updated: %q (ok=%v)", updated, ok)
	}
}

func TestBuildGenerationContextCompactsLongDocuments(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString("## Section Heading\n\n")
		b.WriteString(strings.Repeat("Long narrative prose goes here. ", 40))
		b.WriteString("\n\n")
	}
	docView, tail := BuildGenerationContext(b.String(), 2000)
	if !strings.Contains(docView, "[Earlier document outline]") {
		t.Fatalf("expected compacted outline for long doc")
	}
	if len([]rune(docView)) > 3500 {
		t.Fatalf("expected compacted docView, got len %d", len([]rune(docView)))
	}
	if tail == "" {
		t.Fatal("expected non-empty tail anchor")
	}
}
