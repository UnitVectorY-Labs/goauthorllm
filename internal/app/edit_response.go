package app

import (
	"encoding/json"
	"fmt"
)

// Unmarshal alone treats missing fields and null as zero values, which can
// silently turn an invalid response into a successful task-complete result.
func decodeEditBatch(raw string, limit int) (directedEditBatch, error) {
	var wire struct {
		Suggestions *[]struct {
			OldText *string `json:"old_text"`
			NewText *string `json:"new_text"`
		} `json:"suggestions"`
		RemainingRounds *int `json:"remaining_rounds"`
	}
	if err := json.Unmarshal([]byte(normalizeStructuredJSON(raw)), &wire); err != nil {
		return directedEditBatch{}, err
	}
	if wire.Suggestions == nil || wire.RemainingRounds == nil {
		return directedEditBatch{}, fmt.Errorf("suggestions array and remaining_rounds are required")
	}
	if *wire.RemainingRounds < 0 || len(*wire.Suggestions) > limit {
		return directedEditBatch{}, fmt.Errorf("invalid remaining_rounds or batch size")
	}
	batch := directedEditBatch{RemainingRounds: *wire.RemainingRounds}
	for _, s := range *wire.Suggestions {
		if s.OldText == nil || s.NewText == nil {
			return directedEditBatch{}, fmt.Errorf("each suggestion needs old_text and new_text")
		}
		batch.Suggestions = append(batch.Suggestions, editSuggestion{OldText: *s.OldText, NewText: *s.NewText})
	}
	return batch, nil
}
