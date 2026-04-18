package diff

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTokenizeEmpty(t *testing.T) {
	tokens := Tokenize("")
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens, got %v", tokens)
	}
}

func TestTokenizeSingleWord(t *testing.T) {
	tokens := Tokenize("hello")
	if len(tokens) != 1 || tokens[0] != "hello" {
		t.Fatalf("expected [hello], got %v", tokens)
	}
}

func TestTokenizeWordsAndSpaces(t *testing.T) {
	tokens := Tokenize("hello world")
	expected := []string{"hello", " ", "world"}
	assertTokens(t, tokens, expected)
}

func TestTokenizePreservesMultipleSpaces(t *testing.T) {
	tokens := Tokenize("hello  world")
	expected := []string{"hello", "  ", "world"}
	assertTokens(t, tokens, expected)
}

func TestTokenizePunctuation(t *testing.T) {
	tokens := Tokenize("hello, world!")
	expected := []string{"hello", ", ", "world", "!"}
	assertTokens(t, tokens, expected)
}

func TestTokenizeNewlines(t *testing.T) {
	tokens := Tokenize("line one\nline two")
	expected := []string{"line", " ", "one", "\n", "line", " ", "two"}
	assertTokens(t, tokens, expected)
}

func TestTokenizeHyphens(t *testing.T) {
	tokens := Tokenize("well-known fact")
	expected := []string{"well-known", " ", "fact"}
	assertTokens(t, tokens, expected)
}

func TestTokenizeApostrophes(t *testing.T) {
	tokens := Tokenize("it's fine")
	expected := []string{"it's", " ", "fine"}
	assertTokens(t, tokens, expected)
}

func TestTokenizeRoundTrip(t *testing.T) {
	inputs := []string{
		"hello world",
		"The quick brown fox jumps over the lazy dog.",
		"line one\nline two\n",
		"  leading spaces",
		"trailing spaces  ",
		"mixed\ttabs\tand spaces",
		"punctuation: yes! no? maybe...",
		"",
	}
	for _, input := range inputs {
		tokens := Tokenize(input)
		got := strings.Join(tokens, "")
		if got != input {
			t.Errorf("round-trip failed for %q: got %q", input, got)
		}
	}
}

func TestDiffIdentical(t *testing.T) {
	ops := Diff("hello world", "hello world")
	for _, op := range ops {
		if op.Kind != OpEqual {
			t.Fatalf("expected all OpEqual for identical strings, got %v", ops)
		}
	}
	assertReconstructsOld(t, ops, "hello world")
	assertReconstructsNew(t, ops, "hello world")
}

func TestDiffCompletelyDifferent(t *testing.T) {
	ops := Diff("alpha", "beta")
	assertReconstructsOld(t, ops, "alpha")
	assertReconstructsNew(t, ops, "beta")

	hasDelete := false
	hasInsert := false
	for _, op := range ops {
		if op.Kind == OpDelete {
			hasDelete = true
		}
		if op.Kind == OpInsert {
			hasInsert = true
		}
	}
	if !hasDelete || !hasInsert {
		t.Fatalf("expected both delete and insert ops, got %v", ops)
	}
}

func TestDiffSingleWordChange(t *testing.T) {
	ops := Diff("the quick brown fox", "the slow brown fox")
	assertReconstructsOld(t, ops, "the quick brown fox")
	assertReconstructsNew(t, ops, "the slow brown fox")

	assertHasOp(t, ops, OpDelete, "quick")
	assertHasOp(t, ops, OpInsert, "slow")
	assertHasOp(t, ops, OpEqual, "the")
	assertHasOp(t, ops, OpEqual, "brown")
	assertHasOp(t, ops, OpEqual, "fox")
}

func TestDiffInsertionOnly(t *testing.T) {
	ops := Diff("hello world", "hello beautiful world")
	assertReconstructsOld(t, ops, "hello world")
	assertReconstructsNew(t, ops, "hello beautiful world")

	assertHasOp(t, ops, OpInsert, "beautiful")
}

func TestDiffDeletionOnly(t *testing.T) {
	ops := Diff("hello beautiful world", "hello world")
	assertReconstructsOld(t, ops, "hello beautiful world")
	assertReconstructsNew(t, ops, "hello world")

	assertHasOp(t, ops, OpDelete, "beautiful")
}

func TestDiffFromEmpty(t *testing.T) {
	ops := Diff("", "hello")
	assertReconstructsOld(t, ops, "")
	assertReconstructsNew(t, ops, "hello")

	if len(ops) != 1 || ops[0].Kind != OpInsert || ops[0].Text != "hello" {
		t.Fatalf("expected single insert, got %v", ops)
	}
}

func TestDiffToEmpty(t *testing.T) {
	ops := Diff("hello", "")
	assertReconstructsOld(t, ops, "hello")
	assertReconstructsNew(t, ops, "")

	if len(ops) != 1 || ops[0].Kind != OpDelete || ops[0].Text != "hello" {
		t.Fatalf("expected single delete, got %v", ops)
	}
}

func TestDiffBothEmpty(t *testing.T) {
	ops := Diff("", "")
	if len(ops) != 0 {
		t.Fatalf("expected no ops for empty strings, got %v", ops)
	}
}

func TestDiffMultipleChanges(t *testing.T) {
	ops := Diff(
		"The cat sat on the mat.",
		"The dog sat on the rug.",
	)
	assertReconstructsOld(t, ops, "The cat sat on the mat.")
	assertReconstructsNew(t, ops, "The dog sat on the rug.")

	assertHasOp(t, ops, OpDelete, "cat")
	assertHasOp(t, ops, OpInsert, "dog")
	assertHasOp(t, ops, OpDelete, "mat")
	assertHasOp(t, ops, OpInsert, "rug")
}

func TestDiffPunctuationChange(t *testing.T) {
	ops := Diff("Hello, world!", "Hello; world.")
	assertReconstructsOld(t, ops, "Hello, world!")
	assertReconstructsNew(t, ops, "Hello; world.")
}

func TestDiffMultiline(t *testing.T) {
	old := "First line\nSecond line\nThird line"
	new := "First line\nModified line\nThird line"
	ops := Diff(old, new)
	assertReconstructsOld(t, ops, old)
	assertReconstructsNew(t, ops, new)

	assertHasOp(t, ops, OpDelete, "Second")
	assertHasOp(t, ops, OpInsert, "Modified")
}

func TestDiffWhitespaceOnly(t *testing.T) {
	ops := Diff("hello  world", "hello world")
	assertReconstructsOld(t, ops, "hello  world")
	assertReconstructsNew(t, ops, "hello world")
}

func TestDiffRepeatedWords(t *testing.T) {
	ops := Diff("the the the", "the a the")
	assertReconstructsOld(t, ops, "the the the")
	assertReconstructsNew(t, ops, "the a the")
}

func TestDiffLongSentence(t *testing.T) {
	old := "In the beginning, God created the heavens and the earth."
	new := "In the beginning, the Creator formed the heavens and the world."
	ops := Diff(old, new)
	assertReconstructsOld(t, ops, old)
	assertReconstructsNew(t, ops, new)
}

func TestDiffCaseSensitive(t *testing.T) {
	ops := Diff("Hello World", "hello world")
	assertReconstructsOld(t, ops, "Hello World")
	assertReconstructsNew(t, ops, "hello world")

	assertHasOp(t, ops, OpDelete, "Hello")
	assertHasOp(t, ops, OpInsert, "hello")
}

func TestFormatOldHighlightsDeletes(t *testing.T) {
	ops := []Op{
		{Kind: OpEqual, Text: "hello "},
		{Kind: OpDelete, Text: "old"},
		{Kind: OpEqual, Text: " world"},
	}
	noStyle := lipgloss.NewStyle()
	deleteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	result := FormatOld(ops, noStyle, deleteStyle)
	// Should contain "old" rendered with delete style, but not any insert text
	if !strings.Contains(result, "hello ") {
		t.Fatal("expected unchanged text in output")
	}
	if !strings.Contains(result, "world") {
		t.Fatal("expected unchanged text in output")
	}
}

func TestFormatNewHighlightsInserts(t *testing.T) {
	ops := []Op{
		{Kind: OpEqual, Text: "hello "},
		{Kind: OpInsert, Text: "new"},
		{Kind: OpEqual, Text: " world"},
	}
	noStyle := lipgloss.NewStyle()
	insertStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	result := FormatNew(ops, noStyle, insertStyle)
	if !strings.Contains(result, "hello ") {
		t.Fatal("expected unchanged text in output")
	}
	if !strings.Contains(result, "world") {
		t.Fatal("expected unchanged text in output")
	}
}

func TestFormatOldExcludesInserts(t *testing.T) {
	ops := []Op{
		{Kind: OpEqual, Text: "a"},
		{Kind: OpInsert, Text: "INSERTED"},
		{Kind: OpEqual, Text: "b"},
	}
	noStyle := lipgloss.NewStyle()
	deleteStyle := lipgloss.NewStyle()

	result := FormatOld(ops, noStyle, deleteStyle)
	if strings.Contains(result, "INSERTED") {
		t.Fatal("FormatOld should not include insert text")
	}
}

func TestFormatNewExcludesDeletes(t *testing.T) {
	ops := []Op{
		{Kind: OpEqual, Text: "a"},
		{Kind: OpDelete, Text: "DELETED"},
		{Kind: OpEqual, Text: "b"},
	}
	noStyle := lipgloss.NewStyle()
	insertStyle := lipgloss.NewStyle()

	result := FormatNew(ops, noStyle, insertStyle)
	if strings.Contains(result, "DELETED") {
		t.Fatal("FormatNew should not include delete text")
	}
}

func TestFormatEmptyOps(t *testing.T) {
	noStyle := lipgloss.NewStyle()
	style := lipgloss.NewStyle()

	resultOld := FormatOld(nil, noStyle, style)
	resultNew := FormatNew(nil, noStyle, style)

	if resultOld != "" {
		t.Fatalf("expected empty string for nil ops, got %q", resultOld)
	}
	if resultNew != "" {
		t.Fatalf("expected empty string for nil ops, got %q", resultNew)
	}
}

func TestDiffAdjacentChanges(t *testing.T) {
	ops := Diff("abc def", "xyz ghi")
	assertReconstructsOld(t, ops, "abc def")
	assertReconstructsNew(t, ops, "xyz ghi")
}

func TestDiffTrailingPunctuation(t *testing.T) {
	ops := Diff("end.", "end!")
	assertReconstructsOld(t, ops, "end.")
	assertReconstructsNew(t, ops, "end!")
}

func TestDiffLeadingWhitespace(t *testing.T) {
	ops := Diff("  hello", "    hello")
	assertReconstructsOld(t, ops, "  hello")
	assertReconstructsNew(t, ops, "    hello")
}

// assertTokens checks that token slices match.
func assertTokens(t *testing.T, got, expected []string) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("expected %d tokens %v, got %d tokens %v", len(expected), expected, len(got), got)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("token[%d]: expected %q, got %q", i, expected[i], got[i])
		}
	}
}

// assertReconstructsOld verifies that filtering ops for old text reconstructs the original.
func assertReconstructsOld(t *testing.T, ops []Op, expected string) {
	t.Helper()
	var b strings.Builder
	for _, op := range ops {
		if op.Kind == OpEqual || op.Kind == OpDelete {
			b.WriteString(op.Text)
		}
	}
	if got := b.String(); got != expected {
		t.Fatalf("old reconstruction failed: expected %q, got %q", expected, got)
	}
}

// assertReconstructsNew verifies that filtering ops for new text reconstructs the new string.
func assertReconstructsNew(t *testing.T, ops []Op, expected string) {
	t.Helper()
	var b strings.Builder
	for _, op := range ops {
		if op.Kind == OpEqual || op.Kind == OpInsert {
			b.WriteString(op.Text)
		}
	}
	if got := b.String(); got != expected {
		t.Fatalf("new reconstruction failed: expected %q, got %q", expected, got)
	}
}

// assertHasOp verifies that at least one op with the given kind and text exists.
func assertHasOp(t *testing.T, ops []Op, kind OpKind, text string) {
	t.Helper()
	for _, op := range ops {
		if op.Kind == kind && op.Text == text {
			return
		}
	}
	t.Fatalf("expected op {Kind: %d, Text: %q} not found in %v", kind, text, ops)
}
