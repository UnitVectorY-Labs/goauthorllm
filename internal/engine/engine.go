package engine

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/prompts"
)

// Scope controls how much content a single generation round produces.
type Scope string

const (
	ScopeSentence  Scope = "sentence"
	ScopeParagraph Scope = "paragraph"
	ScopeSection   Scope = "section"
	ScopeFree      Scope = "free"
)

// NormalizeScope maps user/CLI scope names (including legacy continue / new-section)
// to a canonical Scope.
func NormalizeScope(raw string) Scope {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sentence":
		return ScopeSentence
	case "paragraph":
		return ScopeParagraph
	case "section", "new-section":
		return ScopeSection
	case "free", "continue":
		return ScopeFree
	default:
		return ScopeParagraph
	}
}

// Label returns a human-readable label for the scope.
func (s Scope) Label() string {
	switch s {
	case ScopeSentence:
		return "Sentence"
	case ScopeParagraph:
		return "Paragraph"
	case ScopeSection:
		return "Section"
	default:
		return "Free"
	}
}

// Instruction returns clear, model-friendly length/structure rules for the scope.
func (s Scope) Instruction() string {
	switch s {
	case ScopeSentence:
		return "Write 1 to 2 natural sentences that continue the current thought or complete the current sentence. Stop cleanly at sentence end. Do not start a new heading."
	case ScopeParagraph:
		return "Write exactly 1 focused new paragraph (about 3 to 5 sentences). Use join_mode=\"new_paragraph\" if the document ends with a completed sentence, or \"inline_space\" only if finishing an incomplete sentence. Do not start a new heading or write multiple paragraphs."
	case ScopeSection:
		return "Write exactly 1 complete markdown section starting with an appropriate heading (e.g. ## Heading) followed by well-developed content. Do not write more than one section."
	default:
		return "Continue the document naturally with appropriate markdown pacing until the current idea or section feels complete."
	}
}

// PlanStep describes one planned round in a multi-round generation.
type PlanStep struct {
	Title string `json:"title"`
	Goal  string `json:"goal"`
}

// GenerationRequest holds all inputs needed for one generation or planning call.
type GenerationRequest struct {
	Body                 string
	DocumentSystemMsg    string
	ProjectRules         string
	Guidance             string
	Scope                Scope
	RoundIndex           int // 1-based; 0 if single-round
	RoundTotal           int
	RoundStep            *PlanStep
	StructuredGeneration bool
	Overrides            prompts.Overrides
}

// PlanRounds asks the LLM for a concise structured plan of N distinct steps
// when generating multiple rounds (e.g., 5 new sections or 3 paragraphs).
func PlanRounds(ctx context.Context, client *llm.Client, req GenerationRequest, rounds int) ([]PlanStep, error) {
	if rounds <= 1 {
		return nil, nil
	}
	sysPrompt, err := prompts.Render(prompts.PlanPrompt, req.Overrides, nil)
	if err != nil {
		return nil, err
	}
	sysPrompt = combineSystemRules(sysPrompt, req.ProjectRules, req.DocumentSystemMsg)

	docView, _ := document.BuildGenerationContext(req.Body, 10000)
	var user strings.Builder
	fmt.Fprintf(&user, "Plan exactly %d sequential %s additions to extend this markdown document.\n", rounds, req.Scope.Label())
	if strings.TrimSpace(req.Guidance) != "" {
		fmt.Fprintf(&user, "\nAuthor guidance:\n%s\n", strings.TrimSpace(req.Guidance))
	}
	if docView != "" {
		fmt.Fprintf(&user, "\n<document>\n%s\n</document>\n", docView)
	} else {
		user.WriteString("\nThe document is currently empty. Plan from the beginning.\n")
	}
	fmt.Fprintf(&user, "\nReturn a JSON object with a `steps` array containing exactly %d distinct steps in logical order.", rounds)

	messages := []llm.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: user.String()},
	}

	raw, err := client.StructuredChat(ctx, messages, "generation_plan", planSchema(rounds))
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Steps []PlanStep `json:"steps"`
	}
	if err := json.Unmarshal([]byte(NormalizeJSON(raw)), &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Steps) == 0 {
		return nil, fmt.Errorf("plan contained no steps")
	}
	// Trim or pad gracefully to match requested rounds.
	if len(decoded.Steps) > rounds {
		decoded.Steps = decoded.Steps[:rounds]
	}
	return decoded.Steps, nil
}

// BuildGenerationMessages constructs a lean, KV-cache-friendly 2-message prompt
// for generation (both structured-JSON and plain-markdown variants).
func BuildGenerationMessages(req GenerationRequest, structured bool) ([]llm.Message, error) {
	baseSys, err := prompts.Render(prompts.GeneratePrompt, req.Overrides, nil)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", prompts.GeneratePrompt, err)
	}
	if structured {
		baseSys += "\n\nRespond with a JSON object matching the schema:\n" +
			"- join_mode: choose how your addition attaches to the end of the document:\n" +
			"  * \"inline_direct\": attach directly with no space (e.g., completing a partial word or starting with punctuation)\n" +
			"  * \"inline_space\": continue on the same paragraph separated by a single space\n" +
			"  * \"new_paragraph\": start a fresh paragraph separated by a blank line\n" +
			"  * \"new_section\": start a new markdown heading/section separated by a blank line\n" +
			"- intent: 1 concise phrase describing what this addition covers\n" +
			"- content: the exact markdown text to add (no leading separator whitespace needed; no outer code fences)"
	}
	sysMsg := combineSystemRules(baseSys, req.ProjectRules, req.DocumentSystemMsg)

	docView, tailAnchor := document.BuildGenerationContext(req.Body, 14000)

	var task string
	if req.Scope == ScopeSection {
		task, err = prompts.Render(prompts.NewSectionPrompt, req.Overrides, nil)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.NewSectionPrompt, err)
		}
	} else {
		roundLabel := ""
		if req.RoundTotal > 1 && req.RoundIndex > 0 {
			roundLabel = fmt.Sprintf("Round %d of %d", req.RoundIndex, req.RoundTotal)
			if req.RoundStep != nil && strings.TrimSpace(req.RoundStep.Title) != "" {
				roundLabel += " — " + strings.TrimSpace(req.RoundStep.Title)
			}
		}
		task, err = prompts.Render(prompts.ContinuePrompt, req.Overrides, struct {
			HasDocument      bool
			ScopeInstruction string
			RoundLabel       string
		}{
			HasDocument:      docView != "",
			ScopeInstruction: req.Scope.Instruction(),
			RoundLabel:       roundLabel,
		})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.ContinuePrompt, err)
		}
	}

	var user strings.Builder
	if docView != "" {
		user.WriteString("<document>\n")
		user.WriteString(docView)
		user.WriteString("\n</document>\n\n")
		user.WriteString("<document_tail>\n")
		user.WriteString(tailAnchor)
		user.WriteString("\n</document_tail>\n\n")
	}

	user.WriteString(task)

	if req.RoundStep != nil {
		fmt.Fprintf(&user, "\n\nPlanned focus for round %d of %d:\n- Title: %s\n- Goal: %s",
			req.RoundIndex, req.RoundTotal, strings.TrimSpace(req.RoundStep.Title), strings.TrimSpace(req.RoundStep.Goal))
	}

	if strings.TrimSpace(req.Guidance) != "" {
		guidanceText, err := prompts.Render(prompts.UserGuidancePrompt, req.Overrides, struct {
			Prompt string
		}{
			Prompt: strings.TrimSpace(req.Guidance),
		})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.UserGuidancePrompt, err)
		}
		user.WriteString("\n\n")
		user.WriteString(guidanceText)
	}

	return []llm.Message{
		{Role: "system", Content: sysMsg},
		{Role: "user", Content: user.String()},
	}, nil
}

// StreamRound executes one streaming generation round.
func StreamRound(ctx context.Context, client *llm.Client, req GenerationRequest, send func(llm.StreamEvent) error) error {
	plainMsgs, err := BuildGenerationMessages(req, false)
	if err != nil {
		return err
	}
	if !req.StructuredGeneration {
		return client.StreamChat(ctx, plainMsgs, send)
	}
	structuredMsgs, err := BuildGenerationMessages(req, true)
	if err != nil {
		return err
	}
	return client.StreamStructuredGeneration(ctx, structuredMsgs, "markdown_generation", generationSchema, plainMsgs, send)
}

// EditHistoryEntry records a prior edit decision in the current session.
type EditHistoryEntry struct {
	Action  string
	Line    int
	OldText string
	NewText string
	Reason  string
}

// EditRequest holds all inputs for fetching a batch of structured edit suggestions.
type EditRequest struct {
	Body              string
	DocumentSystemMsg string
	ProjectRules      string
	CopyEditRules     string
	Instructions      string // empty => copy edit; non-empty => directed/custom edit
	BatchSize         int
	History           []EditHistoryEntry
	Overrides         prompts.Overrides
}

// EditBatchResult is the validated outcome of FetchEditBatch.
type EditBatchResult struct {
	Assessment       string
	Edits            []document.ResolvedEdit
	MoreRoundsNeeded bool
	Note             string
	Err              error
}

// BuildEditMessages builds a lean 2-message prompt using a line-numbered document.
func BuildEditMessages(req EditRequest, feedback string) ([]llm.Message, error) {
	if req.BatchSize <= 0 {
		return nil, fmt.Errorf("edit batch size must be greater than zero")
	}

	isDirected := strings.TrimSpace(req.Instructions) != ""
	sysPromptName := prompts.EditPrompt
	taskPromptName := prompts.EditTaskPrompt
	if isDirected {
		sysPromptName = prompts.DirectedEditPrompt
		taskPromptName = prompts.DirectedEditTaskPrompt
	}

	sysBase, err := prompts.Render(sysPromptName, req.Overrides, nil)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", sysPromptName, err)
	}
	extraRules := req.ProjectRules
	if !isDirected && strings.TrimSpace(req.CopyEditRules) != "" {
		if extraRules != "" {
			extraRules += "\n\n"
		}
		extraRules += "Copy-editing rules:\n" + strings.TrimSpace(req.CopyEditRules)
	}
	sysMsg := combineSystemRules(sysBase, extraRules, req.DocumentSystemMsg)

	taskText, err := prompts.Render(taskPromptName, req.Overrides, struct {
		BatchSize int
	}{
		BatchSize: req.BatchSize,
	})
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", taskPromptName, err)
	}

	var user strings.Builder
	user.WriteString(taskText)
	if isDirected {
		user.WriteString("\n\nAuthor editing instructions:\n")
		user.WriteString(strings.TrimSpace(req.Instructions))
	}

	if len(req.History) > 0 {
		// Limit history sent to the last 25 entries to keep prompts lean.
		hist := req.History
		if len(hist) > 25 {
			hist = hist[len(hist)-25:]
		}
		historyText, err := prompts.Render(prompts.EditHistoryPrompt, req.Overrides, struct {
			History []EditHistoryEntry
		}{History: hist})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.EditHistoryPrompt, err)
		}
		user.WriteString("\n\n")
		user.WriteString(historyText)
	}

	if strings.TrimSpace(feedback) != "" {
		fbText, err := prompts.Render(prompts.EditFeedbackPrompt, req.Overrides, struct {
			Feedback string
		}{Feedback: strings.TrimSpace(feedback)})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.EditFeedbackPrompt, err)
		}
		user.WriteString("\n\n")
		user.WriteString(fbText)
	}

	user.WriteString("\n\n<document_lines>\n")
	user.WriteString(document.FormatWithLineNumbers(req.Body))
	user.WriteString("\n</document_lines>")

	return []llm.Message{
		{Role: "system", Content: sysMsg},
		{Role: "user", Content: user.String()},
	}, nil
}

// FetchEditBatch requests structured edits from the LLM and deterministically
// resolves & validates every edit against the document with smart whitespace
// integration.
func FetchEditBatch(ctx context.Context, client *llm.Client, req EditRequest) EditBatchResult {
	if strings.TrimSpace(req.Body) == "" {
		return EditBatchResult{Note: "Document is empty. Add content before editing."}
	}
	if req.BatchSize <= 0 {
		return EditBatchResult{Err: fmt.Errorf("edit batch size must be greater than zero")}
	}

	isDirected := strings.TrimSpace(req.Instructions) != ""
	schemaName := "copy_edit_batch"
	if isDirected {
		schemaName = "directed_edit_batch"
	}

	feedback := ""
	for attempt := 0; attempt < 3; attempt++ {
		messages, err := BuildEditMessages(req, feedback)
		if err != nil {
			return EditBatchResult{Err: err}
		}

		raw, err := client.StructuredChat(ctx, messages, schemaName, editBatchSchema(req.BatchSize))
		if err != nil {
			return EditBatchResult{Err: err}
		}

		wire, err := decodeEditWire(raw, req.BatchSize)
		if err != nil {
			feedback = "Invalid JSON response: " + err.Error() + ". Return only the structured object matching the schema."
			continue
		}
		if len(wire.Edits) == 0 {
			note := "No further copy edits suggested."
			if isDirected {
				note = "Editing task complete."
			}
			if strings.TrimSpace(wire.Assessment) != "" {
				note = strings.TrimSpace(wire.Assessment)
			}
			return EditBatchResult{
				Assessment:       strings.TrimSpace(wire.Assessment),
				MoreRoundsNeeded: false,
				Note:             note,
			}
		}

		resolved, failures := resolveNonOverlapping(req.Body, wire.Edits)
		if len(resolved) > 0 {
			note := fmt.Sprintf("Found %d edit(s)", len(resolved))
			if strings.TrimSpace(wire.Assessment) != "" {
				note = strings.TrimSpace(wire.Assessment)
			}
			return EditBatchResult{
				Assessment:       strings.TrimSpace(wire.Assessment),
				Edits:            resolved,
				MoreRoundsNeeded: wire.MoreRoundsNeeded || len(failures) > 0,
				Note:             note,
			}
		}

		// All edits failed to resolve — retry once/twice with exact diagnostics.
		feedback = "None of the proposed edits matched the document:\n" + strings.Join(failures, "\n") +
			"\nCopy old_text verbatim from the document without line-number prefixes."
	}

	return EditBatchResult{Err: fmt.Errorf("could not resolve valid edits: %s", feedback)}
}

type wireEdit struct {
	Reason     string `json:"reason"`
	TargetLine int    `json:"target_line"`
	Action     string `json:"action"`
	OldText    string `json:"old_text"`
	NewText    string `json:"new_text"`
}

type wireEditBatch struct {
	Assessment       string     `json:"assessment"`
	Edits            []wireEdit `json:"edits"`
	MoreRoundsNeeded bool       `json:"more_rounds_needed"`
}

func decodeEditWire(raw string, limit int) (wireEditBatch, error) {
	var parsed struct {
		Assessment       *string     `json:"assessment"`
		Edits            *[]wireEdit `json:"edits"`
		MoreRoundsNeeded *bool       `json:"more_rounds_needed"`
	}
	if err := json.Unmarshal([]byte(NormalizeJSON(raw)), &parsed); err != nil {
		return wireEditBatch{}, err
	}
	if parsed.Edits == nil {
		return wireEditBatch{}, fmt.Errorf("missing edits array")
	}
	edits := *parsed.Edits
	if len(edits) > limit {
		edits = edits[:limit]
	}
	out := wireEditBatch{Edits: edits}
	if parsed.Assessment != nil {
		out.Assessment = *parsed.Assessment
	}
	if parsed.MoreRoundsNeeded != nil {
		out.MoreRoundsNeeded = *parsed.MoreRoundsNeeded
	}
	return out, nil
}

func resolveNonOverlapping(body string, specs []wireEdit) ([]document.ResolvedEdit, []string) {
	var (
		resolved []document.ResolvedEdit
		failures []string
	)
	for i, w := range specs {
		edit, err := document.ResolveEdit(body, document.EditSpec{
			TargetLine: w.TargetLine,
			Action:     document.EditAction(strings.ToLower(strings.TrimSpace(w.Action))),
			OldText:    w.OldText,
			NewText:    w.NewText,
			Reason:     w.Reason,
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("- Edit %d (line %d): %v", i+1, w.TargetLine, err))
			continue
		}
		overlaps := false
		for _, prev := range resolved {
			if edit.Start < prev.End && prev.Start < edit.End {
				overlaps = true
				break
			}
		}
		if overlaps {
			failures = append(failures, fmt.Sprintf("- Edit %d overlaps another edit near line %d", i+1, edit.Line))
			continue
		}
		resolved = append(resolved, edit)
	}
	return resolved, failures
}

func combineSystemRules(base, projectRules, docSystemMsg string) string {
	var parts []string
	if s := strings.TrimSpace(base); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(projectRules); s != "" {
		parts = append(parts, "Project rules (.goauthorllm):\n"+s)
	}
	if s := strings.TrimSpace(docSystemMsg); s != "" {
		parts = append(parts, "Document instructions (front matter):\n"+s)
	}
	return strings.Join(parts, "\n\n")
}

// NormalizeJSON strips optional markdown code fences around JSON output.
func NormalizeJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	return strings.TrimSpace(trimmed)
}

// generationSchema puts join_mode and intent BEFORE content so streaming JSON
// emits boundary metadata first, then streams content live token-by-token.
var generationSchema = jsontext.Value(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "join_mode": {
      "type": "string",
      "enum": ["inline_direct", "inline_space", "new_paragraph", "new_section"]
    },
    "intent": {
      "type": "string"
    },
    "content": {
      "type": "string"
    }
  },
  "required": ["join_mode", "intent", "content"]
}`)

func planSchema(rounds int) jsontext.Value {
	return jsontext.Value(fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "steps": {
      "type": "array",
      "minItems": %d,
      "maxItems": %d,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "title": {"type": "string"},
          "goal": {"type": "string"}
        },
        "required": ["title", "goal"]
      }
    }
  },
  "required": ["steps"]
}`, rounds, rounds))
}

func editBatchSchema(maxItems int) jsontext.Value {
	return jsontext.Value(fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "assessment": {"type": "string"},
    "edits": {
      "type": "array",
      "maxItems": %d,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "reason": {"type": "string"},
          "target_line": {"type": "integer", "minimum": 1},
          "action": {"type": "string", "enum": ["replace", "delete", "insert_after"]},
          "old_text": {"type": "string"},
          "new_text": {"type": "string"}
        },
        "required": ["reason", "target_line", "action", "old_text", "new_text"]
      }
    },
    "more_rounds_needed": {"type": "boolean"}
  },
  "required": ["assessment", "edits", "more_rounds_needed"]
}`, maxItems))
}
