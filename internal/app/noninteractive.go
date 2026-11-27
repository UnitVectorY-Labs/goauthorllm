package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
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
	client := llm.NewClient(cfg.BaseURL, clientModel, cfg.APIKey, cfg.Timeout, cfg.UserAgent)

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
	scope := engine.NormalizeScope(cfg.Submode)
	rounds := max(1, cfg.Rounds)

	baseReq := engine.GenerationRequest{
		Body:                 doc.Body,
		DocumentSystemMsg:    doc.SystemMessage,
		ProjectRules:         cfg.Rules,
		Guidance:             cfg.Guidance,
		Scope:                scope,
		StructuredGeneration: cfg.StructuredGeneration,
		Overrides:            cfg.MessageOverrides,
	}

	var plan []engine.PlanStep
	if rounds > 1 {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		steps, err := engine.PlanRounds(ctx, client, baseReq, rounds)
		cancel()
		if err == nil {
			plan = steps
		}
	}

	changedRounds := 0
	for round := 1; round <= rounds; round++ {
		req := baseReq
		req.Body = doc.Body
		req.RoundIndex = round
		req.RoundTotal = rounds
		if round-1 < len(plan) {
			step := plan[round-1]
			req.RoundStep = &step
		}

		joinMode := document.JoinAuto
		var generated strings.Builder
		err := engine.StreamRound(context.Background(), client, req, func(ev llm.StreamEvent) error {
			if ev.Err != nil {
				return ev.Err
			}
			if ev.JoinMode != "" {
				joinMode = document.JoinMode(ev.JoinMode)
			}
			if ev.Delta != "" {
				generated.WriteString(ev.Delta)
			}
			return nil
		})
		if err != nil {
			return NonInteractiveResult{Changed: changedRounds}, fmt.Errorf("generate content (round %d/%d): %w", round, rounds, err)
		}
		if strings.TrimSpace(generated.String()) == "" {
			continue
		}

		addition := document.NormalizeJoinStart(doc.Body, generated.String(), joinMode, string(scope))
		doc.SetBody(doc.Body + addition)
		if err := doc.Save(); err != nil {
			return NonInteractiveResult{Changed: changedRounds}, fmt.Errorf("save generated content: %w", err)
		}
		if _, err := fmt.Fprintln(output, addition); err != nil {
			return NonInteractiveResult{Changed: changedRounds}, fmt.Errorf("log generated content: %w", err)
		}
		changedRounds++
	}

	return NonInteractiveResult{Changed: changedRounds}, nil
}

func runNonInteractiveEditing(cfg config.Config, client *llm.Client, doc *document.Document, output io.Writer) (NonInteractiveResult, error) {
	instructions := ""
	batchSize := cfg.CopyEditBatchSize
	if strings.EqualFold(cfg.Submode, "directed") {
		instructions = cfg.EditInstructions
		batchSize = cfg.DirectedEditBatchSize
	}

	history := make([]engine.EditHistoryEntry, 0)
	applied := 0

	for {
		if cfg.MaxEdits > 0 && applied >= cfg.MaxEdits {
			return NonInteractiveResult{Changed: applied}, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		result := engine.FetchEditBatch(ctx, client, engine.EditRequest{
			Body:              doc.Body,
			DocumentSystemMsg: doc.SystemMessage,
			ProjectRules:      cfg.Rules,
			CopyEditRules:     cfg.CopyEditRules,
			Instructions:      instructions,
			BatchSize:         batchSize,
			History:           history,
			Overrides:         cfg.MessageOverrides,
		})
		cancel()

		if result.Err != nil {
			return NonInteractiveResult{Changed: applied}, fmt.Errorf("request edit suggestions: %w", result.Err)
		}
		if len(result.Edits) == 0 {
			return NonInteractiveResult{Changed: applied}, nil
		}

		roundApplied := 0
		for _, edit := range result.Edits {
			if cfg.MaxEdits > 0 && applied >= cfg.MaxEdits {
				return NonInteractiveResult{Changed: applied}, nil
			}
			updated, ok := document.ApplyResolvedEdit(doc.Body, edit)
			if !ok {
				continue
			}
			doc.SetBody(updated)
			if err := doc.Save(); err != nil {
				return NonInteractiveResult{Changed: applied}, fmt.Errorf("save edit: %w", err)
			}
			if err := logEdit(output, edit); err != nil {
				return NonInteractiveResult{Changed: applied}, err
			}
			history = append(history, engine.EditHistoryEntry{
				Action:  "approved-all",
				Line:    edit.Line,
				OldText: edit.OldText,
				NewText: edit.NewText,
				Reason:  edit.Reason,
			})
			applied++
			roundApplied++
		}

		if !result.MoreRoundsNeeded || roundApplied == 0 {
			return NonInteractiveResult{Changed: applied}, nil
		}
	}
}

func logEdit(output io.Writer, edit document.ResolvedEdit) error {
	if _, err := fmt.Fprintf(output, "@@ line %d (%s) @@\n", edit.Line, edit.Reason); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	if _, err := fmt.Fprintln(output, "--- old"); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	if _, err := fmt.Fprintln(output, edit.OldText); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	if _, err := fmt.Fprintln(output, "+++ new"); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	if _, err := fmt.Fprintln(output, edit.NewText); err != nil {
		return fmt.Errorf("log edit: %w", err)
	}
	return nil
}
