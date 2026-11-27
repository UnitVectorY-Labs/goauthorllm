package document

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// JoinMode controls how generated text attaches to the end of an existing document.
type JoinMode string

const (
	JoinAuto         JoinMode = "auto"
	JoinInlineDirect JoinMode = "inline_direct"
	JoinInlineSpace  JoinMode = "inline_space"
	JoinNewParagraph JoinMode = "new_paragraph"
	JoinNewSection   JoinMode = "new_section"
)

// EditAction describes the kind of edit operation.
type EditAction string

const (
	ActionReplace     EditAction = "replace"
	ActionDelete      EditAction = "delete"
	ActionInsertAfter EditAction = "insert_after"
)

// EditSpec is a proposed edit returned by the LLM.
type EditSpec struct {
	TargetLine int
	Action     EditAction
	OldText    string
	NewText    string
	Reason     string
}

// ResolvedEdit represents an edit mapped to an exact byte span in the document
// with normalized whitespace integration.
type ResolvedEdit struct {
	Start      int
	End        int
	Line       int // 1-based starting line number
	OldText    string
	NewText    string
	Reason     string
	Action     EditAction
	ContextPre string
	ContextPost string
}

// NormalizeJoinStart computes the exact separator prefix and cleaned first chunk
// when attaching freshly generated content to body, honoring both joinMode and
// scope while preventing double-spaces, missing spaces, or crushed headings.
func NormalizeJoinStart(body, firstChunk string, joinMode JoinMode, scope string) string {
	if firstChunk == "" {
		return ""
	}

	// Empty document: strip leading blank lines/spaces only when starting fresh.
	if strings.TrimSpace(body) == "" {
		return strings.TrimLeft(firstChunk, " \t\r\n")
	}

	clean := firstChunk
	trimmedHead := strings.TrimLeft(clean, " \t\r\n")
	if trimmedHead == "" {
		return ""
	}

	// Determine effective join mode.
	effective := joinMode
	if scope == "section" || looksLikeHeadingStart(trimmedHead) {
		effective = JoinNewSection
	} else if scope == "paragraph" && effective != JoinInlineDirect && endsWithCompletedSentence(body) {
		effective = JoinNewParagraph
	} else if effective == "" || effective == JoinAuto {
		if scope == "paragraph" {
			effective = JoinNewParagraph
		} else {
			effective = inferJoinMode(body, clean)
		}
	}

	switch effective {
	case JoinNewSection, JoinNewParagraph:
		prefix := blankLineSeparator(body)
		return prefix + strings.TrimLeft(clean, " \t\r\n")

	case JoinInlineDirect:
		// Keep direct attachment (e.g., punctuation or finishing a word).
		if strings.HasSuffix(body, "\n") {
			return strings.TrimLeft(clean, "\r\n")
		}
		return clean

	case JoinInlineSpace:
		// If body ends with a heading line, never append inline to the heading.
		if looksLikeHeadingStart(lastNonEmptyLine(body)) && !strings.HasSuffix(body, "\n\n") {
			return blankLineSeparator(body) + strings.TrimLeft(clean, " \t\r\n")
		}
		// If body already ends in whitespace or newline, strip duplicate leading spaces.
		if strings.HasSuffix(body, " ") || strings.HasSuffix(body, "\t") || strings.HasSuffix(body, "\n") {
			return strings.TrimLeft(clean, " \t\r\n")
		}
		firstRune, ok := firstNonSpaceRune(clean)
		if ok && isNoLeadingSpace(firstRune) {
			return strings.TrimLeft(clean, " \t\r\n")
		}
		return " " + strings.TrimLeft(clean, " \t\r\n")

	default:
		return clean
	}
}

func inferJoinMode(body, chunk string) JoinMode {
	if strings.HasSuffix(body, "\n\n") {
		return JoinNewParagraph
	}
	lastLn := lastLineText(body)
	if looksLikeHeadingStart(lastLn) {
		return JoinNewParagraph
	}
	if strings.HasPrefix(chunk, "\n\n") {
		return JoinNewParagraph
	}
	firstRune, ok := firstNonSpaceRune(chunk)
	if !ok {
		return JoinInlineDirect
	}
	if isNoLeadingSpace(firstRune) {
		return JoinInlineDirect
	}
	if strings.HasSuffix(body, " ") || strings.HasSuffix(body, "\t") || strings.HasSuffix(body, "\n") {
		return JoinInlineDirect
	}
	return JoinInlineSpace
}

func blankLineSeparator(body string) string {
	trimmedRightSpaces := strings.TrimRight(body, " \t")
	trailingNL := 0
	for i := len(trimmedRightSpaces) - 1; i >= 0 && trimmedRightSpaces[i] == '\n'; i-- {
		trailingNL++
	}
	switch {
	case trailingNL >= 2:
		return ""
	case trailingNL == 1:
		return "\n"
	default:
		return "\n\n"
	}
}

func looksLikeHeadingStart(s string) bool {
	s = strings.TrimLeft(s, " ")
	if len(s) < 2 || s[0] != '#' {
		return false
	}
	n := 0
	for n < len(s) && n < 6 && s[n] == '#' {
		n++
	}
	return n > 0 && n < len(s) && s[n] == ' '
}

func lastLineText(s string) string {
	trimmed := strings.TrimRight(s, "\n")
	if idx := strings.LastIndex(trimmed, "\n"); idx >= 0 {
		return strings.TrimSpace(trimmed[idx+1:])
	}
	return strings.TrimSpace(trimmed)
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, " \t\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return ""
}

func endsWithCompletedSentence(body string) bool {
	trimmed := strings.TrimRight(body, " \t\r\n")
	if trimmed == "" {
		return false
	}
	runes := []rune(trimmed)
	last := runes[len(runes)-1]
	if (last == '"' || last == '\'' || last == ')' || last == ']') && len(runes) > 1 {
		last = runes[len(runes)-2]
	}
	return last == '.' || last == '!' || last == '?'
}

func firstNonSpaceRune(s string) (rune, bool) {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return r, true
		}
	}
	return 0, false
}

func isNoLeadingSpace(r rune) bool {
	switch r {
	case '.', ',', '!', '?', ';', ':', ')', ']', '}', '%', '\'', '"':
		return true
	default:
		return false
	}
}

// FormatWithLineNumbers renders body with 1-based line numbers (`1| text`) so
// LLMs can anchor edits reliably even when phrases repeat.
func FormatWithLineNumbers(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d| %s", i+1, line)
	}
	return b.String()
}

// BuildGenerationContext returns a token-efficient representation of body for
// generation prompts. Short/medium documents are included verbatim; very long
// documents include an outline of earlier sections plus the full recent tail.
func BuildGenerationContext(body string, maxChars int) (docView string, tailAnchor string) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if strings.TrimSpace(body) == "" {
		return "", ""
	}
	if maxChars <= 0 {
		maxChars = 14000
	}

	// Build exact tail anchor (last 3 non-empty lines or up to 320 runes).
	lines := strings.Split(body, "\n")
	tailStart := max(0, len(lines)-4)
	tailAnchor = strings.Join(lines[tailStart:], "\n")
	if runes := []rune(tailAnchor); len(runes) > 360 {
		tailAnchor = string(runes[len(runes)-360:])
	}

	if len([]rune(body)) <= maxChars {
		return body, tailAnchor
	}

	// Keep an outline of earlier headings + the last maxChars*3/4 runes verbatim.
	sections := SplitSections(body)
	var outline strings.Builder
	outline.WriteString("[Earlier document outline]\n")
	for _, sec := range sections {
		if sec.Heading != "" {
			outline.WriteString("- ")
			outline.WriteString(sec.Heading)
			outline.WriteByte('\n')
		}
	}
	outline.WriteString("\n[Recent document content]\n")
	keep := maxChars * 3 / 4
	runes := []rune(body)
	recent := string(runes[len(runes)-keep:])
	// Align recent start to a line boundary if possible.
	if idx := strings.IndexByte(recent, '\n'); idx >= 0 && idx < 200 {
		recent = recent[idx+1:]
	}
	return outline.String() + recent, tailAnchor
}

// ResolveEdit locates an EditSpec in body using exact matching, line-anchored
// disambiguation for repeated text, and whitespace-tolerant fallback, and
// normalizes replacement spacing so edits integrate cleanly into surrounding
// newlines and spaces.
func ResolveEdit(body string, spec EditSpec) (ResolvedEdit, error) {
	if strings.TrimSpace(body) == "" {
		return ResolvedEdit{}, fmt.Errorf("document is empty")
	}

	action := spec.Action
	if action == "" {
		if spec.NewText == "" {
			action = ActionDelete
		} else {
			action = ActionReplace
		}
	}

	oldText := strings.ReplaceAll(spec.OldText, "\r\n", "\n")
	newText := strings.ReplaceAll(spec.NewText, "\r\n", "\n")

	// Strip accidental line-number prefix (e.g., "12| ") if a smaller LLM echoed it.
	oldText = stripLineNumberPrefixes(oldText)
	newText = stripLineNumberPrefixes(newText)

	if oldText == "" {
		return ResolvedEdit{}, fmt.Errorf("old_text is empty")
	}
	if action == ActionReplace && oldText == newText {
		return ResolvedEdit{}, fmt.Errorf("old_text and new_text are identical")
	}
	if action == ActionDelete {
		newText = ""
	}
	if action == ActionInsertAfter {
		if strings.TrimSpace(newText) == "" {
			return ResolvedEdit{}, fmt.Errorf("insert_after requires non-empty new_text")
		}
	}

	start, end, ok := findBestSpan(body, oldText, spec.TargetLine)
	if !ok {
		return ResolvedEdit{}, fmt.Errorf("could not locate old_text near line %d", spec.TargetLine)
	}

	matchedOld := body[start:end]
	finalStart, finalEnd, finalNew := integrateEditSpan(body, start, end, matchedOld, newText, action)
	if body[finalStart:finalEnd] == finalNew {
		return ResolvedEdit{}, fmt.Errorf("edit produces no change")
	}

	line := lineNumberAtOffset(body, finalStart)
	pre, post := surroundingContext(body, finalStart, finalEnd)

	return ResolvedEdit{
		Start:       finalStart,
		End:         finalEnd,
		Line:        line,
		OldText:     body[finalStart:finalEnd],
		NewText:     finalNew,
		Reason:      strings.TrimSpace(spec.Reason),
		Action:      action,
		ContextPre:  pre,
		ContextPost: post,
	}, nil
}

// ApplyResolvedEdit applies a previously resolved edit if the underlying slice
// still matches, or re-resolves it cleanly if prior edits shifted offsets.
func ApplyResolvedEdit(body string, edit ResolvedEdit) (string, bool) {
	if edit.Start >= 0 && edit.End <= len(body) && edit.Start <= edit.End && body[edit.Start:edit.End] == edit.OldText {
		return body[:edit.Start] + edit.NewText + body[edit.End:], true
	}
	// Re-locate near original line if earlier edits shifted byte offsets.
	start, end, ok := findBestSpan(body, edit.OldText, edit.Line)
	if !ok {
		return body, false
	}
	return body[:start] + edit.NewText + body[end:], true
}

func findBestSpan(body, oldText string, targetLine int) (int, int, bool) {
	// 1. Exact matches (disambiguated by targetLine if multiple).
	if matches := allMatchOffsets(body, oldText); len(matches) > 0 {
		best := pickClosestToLine(body, matches, targetLine)
		return best, best + len(oldText), true
	}

	// 2. Trimmed leading/trailing whitespace match (common when LLM adds/drops \n or space).
	trimmed := strings.TrimSpace(oldText)
	if trimmed != "" && trimmed != oldText {
		if matches := allMatchOffsets(body, trimmed); len(matches) > 0 {
			best := pickClosestToLine(body, matches, targetLine)
			return best, best + len(trimmed), true
		}
	}

	// 3. Multi-line trimmed-line match anchored near targetLine.
	if start, end, ok := findLineWindowMatch(body, oldText, targetLine); ok {
		return start, end, true
	}

	return 0, 0, false
}

func allMatchOffsets(body, needle string) []int {
	if needle == "" {
		return nil
	}
	var offsets []int
	searchFrom := 0
	for {
		idx := strings.Index(body[searchFrom:], needle)
		if idx < 0 {
			break
		}
		abs := searchFrom + idx
		offsets = append(offsets, abs)
		searchFrom = abs + 1
	}
	return offsets
}

func pickClosestToLine(body string, offsets []int, targetLine int) int {
	if len(offsets) == 1 || targetLine <= 0 {
		return offsets[0]
	}
	bestOffset := offsets[0]
	bestDist := math.MaxInt
	for _, off := range offsets {
		line := lineNumberAtOffset(body, off)
		dist := line - targetLine
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestOffset = off
		}
	}
	return bestOffset
}

func findLineWindowMatch(body, needle string, targetLine int) (int, int, bool) {
	needleLines := strings.Split(strings.Trim(needle, "\n"), "\n")
	if len(needleLines) == 0 {
		return 0, 0, false
	}
	bodyLines := strings.Split(body, "\n")
	lineStarts := make([]int, len(bodyLines))
	pos := 0
	for i, ln := range bodyLines {
		lineStarts[i] = pos
		pos += len(ln) + 1
	}

	var candidates []int
	for i := 0; i <= len(bodyLines)-len(needleLines); i++ {
		matched := true
		for j, nl := range needleLines {
			if strings.TrimSpace(bodyLines[i+j]) != strings.TrimSpace(nl) {
				matched = false
				break
			}
		}
		if matched {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return 0, 0, false
	}

	bestIdx := candidates[0]
	if targetLine > 0 {
		bestDist := math.MaxInt
		for _, idx := range candidates {
			dist := (idx + 1) - targetLine
			if dist < 0 {
				dist = -dist
			}
			if dist < bestDist {
				bestDist = dist
				bestIdx = idx
			}
		}
	}

	start := lineStarts[bestIdx]
	lastLineIdx := bestIdx + len(needleLines) - 1
	end := lineStarts[lastLineIdx] + len(bodyLines[lastLineIdx])
	return start, end, true
}

// integrateEditSpan ensures replacements, deletions, and insertions respect
// surrounding spaces, line breaks, and markdown blank lines cleanly.
func integrateEditSpan(body string, start, end int, matchedOld, newText string, action EditAction) (int, int, string) {
	switch action {
	case ActionInsertAfter:
		// Keep matchedOld intact and attach newText seamlessly after it.
		sep := ""
		if strings.HasPrefix(strings.TrimLeft(newText, " \t\r\n"), "#") || strings.Contains(newText, "\n") {
			// Block insertion after a full line/paragraph.
			sep = "\n\n"
			newText = strings.Trim(newText, "\n")
		} else if end == len(body) || body[end] == '\n' {
			// End of a line/sentence.
			if strings.HasSuffix(matchedOld, " ") || strings.HasPrefix(newText, " ") || isPunctuationStart(newText) {
				sep = ""
			} else {
				sep = " "
			}
		} else if !strings.HasSuffix(matchedOld, " ") && !strings.HasPrefix(newText, " ") && !isPunctuationStart(newText) {
			sep = " "
		}
		return start, end, matchedOld + sep + newText

	case ActionDelete:
		// Check if deleting an entire line or multi-line paragraph block.
		atLineStart := start == 0 || body[start-1] == '\n'
		atLineEnd := end == len(body) || body[end] == '\n'
		if atLineStart && atLineEnd {
			s, e := start, end
			// Consume trailing newline(s) so deleting a whole paragraph doesn't leave 3+ blank lines.
			if e < len(body) && body[e] == '\n' {
				e++
				if e < len(body) && body[e] == '\n' {
					// If preceded by a blank line or at start of document, also drop the extra separator blank line.
					if s == 0 || (s >= 2 && body[s-1] == '\n' && body[s-2] == '\n') {
						e++
					}
				}
			} else if s > 0 && body[s-1] == '\n' {
				// Deleting the final line/paragraph at EOF: consume preceding blank lines cleanly.
				s--
				if s > 0 && body[s-1] == '\n' {
					s--
				}
			}
			return s, e, ""
		}
		// Inline deletion inside a line: avoid leaving double spaces.
		s, e := start, end
		if s > 0 && body[s-1] == ' ' && e < len(body) && body[e] == ' ' {
			e++
		} else if s > 0 && body[s-1] == ' ' && e < len(body) && isNoLeadingSpace(rune(body[e])) {
			s--
		}
		return s, e, ""

	default: // ActionReplace
		// Preserve boundary newlines if matchedOld started/ended with them and LLM omitted them.
		replacement := newText
		if strings.HasPrefix(matchedOld, "\n") && !strings.HasPrefix(replacement, "\n") {
			leading := countLeading(matchedOld, '\n')
			replacement = strings.Repeat("\n", leading) + replacement
		}
		if strings.HasSuffix(matchedOld, "\n") && !strings.HasSuffix(replacement, "\n") {
			trailing := countTrailing(matchedOld, '\n')
			replacement = replacement + strings.Repeat("\n", trailing)
		}
		// Preserve leading/trailing single space if matchedOld was space-bounded.
		if strings.HasPrefix(matchedOld, " ") && !strings.HasPrefix(replacement, " ") && !strings.HasPrefix(replacement, "\n") {
			replacement = " " + replacement
		}
		if strings.HasSuffix(matchedOld, " ") && !strings.HasSuffix(replacement, " ") && !strings.HasSuffix(replacement, "\n") {
			replacement = replacement + " "
		}
		return start, end, replacement
	}
}

func isPunctuationStart(s string) bool {
	r, ok := firstNonSpaceRune(s)
	return ok && isNoLeadingSpace(r)
}

func countLeading(s string, ch byte) int {
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return n
}

func countTrailing(s string, ch byte) int {
	n := 0
	for n < len(s) && s[len(s)-1-n] == ch {
		n++
	}
	return n
}

func stripLineNumberPrefixes(s string) string {
	lines := strings.Split(s, "\n")
	changed := false
	for i, line := range lines {
		bar := strings.Index(line, "| ")
		if bar > 0 && bar <= 6 {
			allDigits := true
			for j := 0; j < bar; j++ {
				if line[j] < '0' || line[j] > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				lines[i] = line[bar+2:]
				changed = true
			}
		}
	}
	if !changed {
		return s
	}
	return strings.Join(lines, "\n")
}

func lineNumberAtOffset(body string, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(body) {
		offset = len(body)
	}
	return 1 + strings.Count(body[:offset], "\n")
}

func surroundingContext(body string, start, end int) (pre, post string) {
	// Up to 1 line before start and 1 line after end for visual context in diff view.
	lineStart := strings.LastIndex(body[:start], "\n")
	if lineStart > 0 {
		prevStart := strings.LastIndex(body[:lineStart], "\n")
		pre = strings.TrimSpace(body[prevStart+1 : lineStart])
	}
	if end < len(body) {
		after := body[end:]
		if nl := strings.IndexByte(after, '\n'); nl >= 0 {
			rest := after[nl+1:]
			if nl2 := strings.IndexByte(rest, '\n'); nl2 >= 0 {
				post = strings.TrimSpace(rest[:nl2])
			} else {
				post = strings.TrimSpace(rest)
			}
		}
	}
	return pre, post
}
