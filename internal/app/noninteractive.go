package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

// NonInteractiveResult describes a completed non-interactive operation.
type NonInteractiveResult struct {
	Changed int
}

// RunNonInteractive performs one configured operation without starting Bubble Tea.
func RunNonInteractive(cfg config.Config, output io.Writer) (NonInteractiveResult, error) {
	clientModel := cfg.GenerationModel
	if strings.EqualFold(cfg.Mode, "edit") {
		clientModel = cfg.EditingModel
	}
	if strings.TrimSpace(clientModel) == "" {
		clientModel = cfg.Model
	}
	client := llm.NewClient(cfg.BaseURL, clientModel, cfg.APIKey, cfg.Timeout)

	doc, err := document.Load(cfg.FilePath)
	if err != nil {
		return NonInteractiveResult{}, fmt.Errorf("load document: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "generate":
		return runNonInteractiveGeneration(cfg, client, doc, output)
	case "edit":
		return runNonInteractiveEditing(cfg, client, doc, output)
	default:
		return NonInteractiveResult{}, fmt.Errorf("unsupported non-interactive mode %q", cfg.Mode)
	}
}

func runNonInteractiveGeneration(cfg config.Config, client *llm.Client, doc *document.Document, output io.Writer) (NonInteractiveResult, error) {
	mode := modeContinue
	if strings.EqualFold(cfg.Submode, "new-section") {
		mode = modeNewSection
	}
	messages, err := buildGenerationMessages(doc.Body, doc.SystemMessage, cfg.Guidance, mode, cfg.MessageOverrides)
	if err != nil {
		return NonInteractiveResult{}, err
	}

	var generated strings.Builder
	err = client.StreamChat(context.Background(), messages, func(event llm.StreamEvent) error {
		if event.Err != nil {
			return event.Err
		}
		if event.Delta != "" {
			_, _ = generated.WriteString(event.Delta)
		}
		return nil
	})
	if err != nil {
		return NonInteractiveResult{}, fmt.Errorf("generate content: %w", err)
	}
	if strings.TrimSpace(generated.String()) == "" {
		return NonInteractiveResult{}, nil
	}

	addition := normalizeGenerationStart(doc.Body, generated.String(), mode)
	doc.SetBody(doc.Body + addition)
	if err := doc.Save(); err != nil {
		return NonInteractiveResult{}, fmt.Errorf("save generated content: %w", err)
	}
	if _, err := fmt.Fprintln(output, addition); err != nil {
		return NonInteractiveResult{}, fmt.Errorf("log generated content: %w", err)
	}
	return NonInteractiveResult{Changed: 1}, nil
}

func runNonInteractiveEditing(cfg config.Config, client *llm.Client, doc *document.Document, output io.Writer) (NonInteractiveResult, error) {
	kind := editKindCopy
	batchSize := cfg.CopyEditBatchSize
	if strings.EqualFold(cfg.Submode, "directed") {
		kind = editKindDirected
		batchSize = cfg.DirectedEditBatchSize
	}
	approveWithLLM := strings.EqualFold(cfg.Approval, "llm-approved")
	history := make([]editHistoryEntry, 0)
	applied := 0

	for {
		if cfg.MaxEdits > 0 && applied >= cfg.MaxEdits {
			return NonInteractiveResult{Changed: applied}, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		result := fetchEditSuggestion(ctx, client, doc.Body, doc.SystemMessage, history, cfg.MessageOverrides, kind, cfg.EditInstructions, batchSize)
		cancel()
		if result.Err != nil {
			return NonInteractiveResult{Changed: applied}, fmt.Errorf("request edit suggestions: %w", result.Err)
		}
		if len(result.Suggestions) == 0 {
			return NonInteractiveResult{Changed: applied}, nil
		}

		for _, suggestion := range result.Suggestions {
			if cfg.MaxEdits > 0 && applied >= cfg.MaxEdits {
				return NonInteractiveResult{Changed: applied}, nil
			}
			if approveWithLLM {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
				approved, err := approveEditSuggestion(ctx, client, doc.Body, suggestion, kind, cfg.EditInstructions, cfg.MessageOverrides)
				cancel()
				if err != nil {
					return NonInteractiveResult{Changed: applied}, fmt.Errorf("review edit suggestion: %w", err)
				}
				if !approved {
					history = append(history, editHistoryEntry{Action: "llm-rejected", OldText: suggestion.OldText, NewText: suggestion.NewText})
					continue
				}
			}

			updated, ok := document.ReplaceUnique(doc.Body, suggestion.OldText, suggestion.NewText)
			if !ok {
				return NonInteractiveResult{Changed: applied}, fmt.Errorf("edit became stale: old text no longer matches exactly once")
			}
			doc.SetBody(updated)
			if err := doc.Save(); err != nil {
				return NonInteractiveResult{Changed: applied}, fmt.Errorf("save edit: %w", err)
			}
			if err := logEdit(output, suggestion); err != nil {
				return NonInteractiveResult{Changed: applied}, err
			}
			history = append(history, editHistoryEntry{Action: "non-interactive-accepted", OldText: suggestion.OldText, NewText: suggestion.NewText})
			applied++
		}

		if result.RemainingRounds <= 0 {
			return NonInteractiveResult{Changed: applied}, nil
		}
	}
}

func logEdit(output io.Writer, suggestion editSuggestion) error {
	if _, err := fmt.Fprintln(output, "--- old"); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	if _, err := fmt.Fprintln(output, suggestion.OldText); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	if _, err := fmt.Fprintln(output, "+++ new"); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	if _, err := fmt.Fprintln(output, suggestion.NewText); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	return nil
}
