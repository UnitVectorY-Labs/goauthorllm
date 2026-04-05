package document

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitSections(t *testing.T) {
	body := "Preface text.\n\n# One\nAlpha\n\n## Two\nBeta"

	sections := SplitSections(body)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0].Heading != "" {
		t.Fatalf("expected preface section without heading, got %q", sections[0].Heading)
	}
	if sections[1].Heading != "# One" {
		t.Fatalf("unexpected first heading: %q", sections[1].Heading)
	}
	if sections[2].Heading != "## Two" {
		t.Fatalf("unexpected second heading: %q", sections[2].Heading)
	}
}

func TestLoadAndSavePreservesFrontMatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "draft.md")
	original := "---\ntitle: Draft\nsystem_message: Keep it lyrical.\n---\n\n# Intro\nHello\n"

	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if doc.SystemMessage != "Keep it lyrical." {
		t.Fatalf("unexpected system message: %q", doc.SystemMessage)
	}

	doc.SetBody("# Intro\nHello again")
	if err := doc.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}

	expected := "---\ntitle: Draft\nsystem_message: Keep it lyrical.\n---\n\n# Intro\nHello again\n"
	if string(saved) != expected {
		t.Fatalf("unexpected saved content:\n%s", string(saved))
	}
}

func TestSetFrontMatterUpdatesSystemMessage(t *testing.T) {
	doc := &Document{}
	doc.SetFrontMatter("title: Draft\nsystem_message: Stay tense.")

	if doc.SystemMessage != "Stay tense." {
		t.Fatalf("unexpected system message: %q", doc.SystemMessage)
	}
	if !doc.Dirty {
		t.Fatalf("expected dirty after front matter change")
	}
}

func TestLoadMissingFileCreatesEmptyDocument(t *testing.T) {
	doc, err := Load(filepath.Join(t.TempDir(), "nonexistent.md"))
	if err != nil {
		t.Fatalf("load missing file: %v", err)
	}
	if doc.Body != "" || doc.FrontMatter != "" {
		t.Fatalf("expected empty document, got body=%q front_matter=%q", doc.Body, doc.FrontMatter)
	}
}

func TestListMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.md", "a.markdown", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	files, err := ListMarkdownFiles(dir)
	if err != nil {
		t.Fatalf("list markdown files: %v", err)
	}

	if len(files) != 2 || files[0] != "a.markdown" || files[1] != "b.md" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestReplaceUnique(t *testing.T) {
	body := "Alpha beta.\nAlpha gamma."

	replaced, ok := ReplaceUnique(body, "beta", "delta")
	if !ok {
		t.Fatal("expected unique replacement to succeed")
	}
	if replaced != "Alpha delta.\nAlpha gamma." {
		t.Fatalf("unexpected replacement result: %q", replaced)
	}

	unchanged, ok := ReplaceUnique(body, "Alpha", "Omega")
	if ok {
		t.Fatal("expected duplicate replacement to fail")
	}
	if unchanged != body {
		t.Fatalf("expected unchanged body, got %q", unchanged)
	}
}

func TestNormalizeMarkdownFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"draft", "draft.md"},
		{"  draft.md  ", "draft.md"},
		{"notes.markdown", "notes.markdown"},
		{"", ""},
		{"path/to/file", "file.md"},
	}
	for _, tc := range tests {
		got := NormalizeMarkdownFilename(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeMarkdownFilename(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestDocumentWithoutFrontMatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.md")
	if err := os.WriteFile(path, []byte("# Hello\nWorld\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if doc.FrontMatter != "" {
		t.Fatalf("expected empty front matter, got %q", doc.FrontMatter)
	}
	if doc.Body != "# Hello\nWorld\n" {
		t.Fatalf("unexpected body: %q", doc.Body)
	}
}

func TestMatchCountEdgeCases(t *testing.T) {
	if got := MatchCount("hello", ""); got != 0 {
		t.Fatalf("expected 0 for empty needle, got %d", got)
	}
	if got := MatchCount("aaa", "a"); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}
