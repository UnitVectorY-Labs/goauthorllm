package document

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitSections(t *testing.T) {
	body := `Preface text.

# One
Alpha

## Two
Beta`

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
	original := `---
title: Draft
system_message: Keep it lyrical.
---

# Intro
Hello
`

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

	if string(saved) != `---
title: Draft
system_message: Keep it lyrical.
---

# Intro
Hello again
` {
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
