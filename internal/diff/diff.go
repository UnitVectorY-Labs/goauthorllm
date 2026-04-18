// Package diff provides word-level diff highlighting for edit suggestions.
// It uses the Myers diff algorithm to compute minimal edits between two
// strings and renders the result with ANSI color codes: red for removed
// text and green for added text.
package diff

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// OpKind describes the kind of edit operation.
type OpKind int

const (
	// OpEqual means the token is unchanged.
	OpEqual OpKind = iota
	// OpDelete means the token was removed from the old text.
	OpDelete
	// OpInsert means the token was added in the new text.
	OpInsert
)

// Op represents a single diff operation on a token.
type Op struct {
	Kind OpKind
	Text string
}

// Tokenize splits text into alternating tokens of words and whitespace/punctuation,
// preserving all original characters so the tokens can be concatenated back to
// reproduce the input exactly.
func Tokenize(s string) []string {
	if s == "" {
		return nil
	}

	var tokens []string
	var current strings.Builder
	inWord := false

	for _, r := range s {
		isWordChar := isWord(r)
		if isWordChar != inWord && current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
		current.WriteRune(r)
		inWord = isWordChar
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// Diff computes a word-level diff between old and new text using the
// Myers diff algorithm.
func Diff(oldText, newText string) []Op {
	oldTokens := Tokenize(oldText)
	newTokens := Tokenize(newText)
	return myersDiff(oldTokens, newTokens)
}

// FormatOld renders the old text with deleted portions highlighted in red.
// Unchanged portions are rendered with the given default style.
func FormatOld(ops []Op, defaultStyle, deleteStyle lipgloss.Style) string {
	var b strings.Builder
	for _, op := range ops {
		switch op.Kind {
		case OpEqual:
			b.WriteString(defaultStyle.Render(op.Text))
		case OpDelete:
			b.WriteString(deleteStyle.Render(op.Text))
		case OpInsert:
			// Skip inserts when rendering old text
		}
	}
	return b.String()
}

// FormatNew renders the new text with inserted portions highlighted in green.
// Unchanged portions are rendered with the given default style.
func FormatNew(ops []Op, defaultStyle, insertStyle lipgloss.Style) string {
	var b strings.Builder
	for _, op := range ops {
		switch op.Kind {
		case OpEqual:
			b.WriteString(defaultStyle.Render(op.Text))
		case OpInsert:
			b.WriteString(insertStyle.Render(op.Text))
		case OpDelete:
			// Skip deletes when rendering new text
		}
	}
	return b.String()
}

// isWord returns true for characters that should be grouped into word tokens.
func isWord(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_' || r == '-' || r == '\''
}

// myersDiff implements the Myers diff algorithm on token slices.
// It returns a sequence of Op values representing the minimal edit script.
func myersDiff(a, b []string) []Op {
	n := len(a)
	m := len(b)

	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		ops := make([]Op, m)
		for i, t := range b {
			ops[i] = Op{Kind: OpInsert, Text: t}
		}
		return ops
	}
	if m == 0 {
		ops := make([]Op, n)
		for i, t := range a {
			ops[i] = Op{Kind: OpDelete, Text: t}
		}
		return ops
	}

	// Myers algorithm
	max := n + m
	// v stores the furthest reaching endpoint for each diagonal k.
	// We index with k + max to avoid negative indices.
	size := 2*max + 1
	v := make([]int, size)

	// trace stores a copy of v at each step d, used to reconstruct the path.
	type traceEntry = []int
	var trace []traceEntry

	var found bool
	for d := 0; d <= max; d++ {
		// Save current state for backtracking
		vc := make([]int, size)
		copy(vc, v)
		trace = append(trace, vc)

		for k := -d; k <= d; k += 2 {
			var x int
			idx := k + max
			if k == -d || (k != d && v[idx-1] < v[idx+1]) {
				x = v[idx+1] // move down (insert)
			} else {
				x = v[idx-1] + 1 // move right (delete)
			}
			y := x - k

			// Follow diagonal (equal tokens)
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}

			v[idx] = x

			if x >= n && y >= m {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	// Backtrack to reconstruct the edit script
	return backtrack(trace, a, b, max)
}

// backtrack reconstructs the edit operations from the Myers trace.
func backtrack(trace [][]int, a, b []string, max int) []Op {
	x := len(a)
	y := len(b)

	var ops []Op

	for d := len(trace) - 1; d >= 0; d-- {
		v := trace[d]
		k := x - y
		idx := k + max

		var prevK int
		if k == -d || (k != d && v[idx-1] < v[idx+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[prevK+max]
		prevY := prevX - prevK

		// Follow diagonal back (equal tokens)
		for x > prevX && y > prevY {
			x--
			y--
			ops = append(ops, Op{Kind: OpEqual, Text: a[x]})
		}

		if d > 0 {
			if x == prevX {
				// Insert
				y--
				ops = append(ops, Op{Kind: OpInsert, Text: b[y]})
			} else {
				// Delete
				x--
				ops = append(ops, Op{Kind: OpDelete, Text: a[x]})
			}
		}
	}

	// Reverse
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}

	return ops
}
