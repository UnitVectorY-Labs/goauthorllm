package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

// speedPresets lets users click or tap a key to cycle reading pace quickly.
var speedPresets = []int{0, 35, 65, 110, 180}

type planDoneMsg struct {
	runID int
	steps []engine.PlanStep
	err   error
}

type streamChunkMsg struct {
	runID int
	round int
	event llm.StreamEvent
}

type pacerTickMsg struct {
	runID int
	round int
	at    time.Time
}

func (m *Model) cycleScope(delta int) {
	scopes := []engine.Scope{engine.ScopeSentence, engine.ScopeParagraph, engine.ScopeSection, engine.ScopeFree}
	idx := 0
	for i, s := range scopes {
		if s == m.gen.scope {
			idx = i
			break
		}
	}
	m.gen.scope = scopes[(idx+delta+len(scopes))%len(scopes)]
	m.setStatus("Generation scope: "+m.gen.scope.Label(), "accent")
}

func (m *Model) adjustRounds(delta int) {
	m.gen.rounds = clampInt(m.gen.rounds+delta, 1, 20)
	m.setStatus(fmt.Sprintf("Generation rounds: ×%d", m.gen.rounds), "accent")
}

func (m *Model) cycleStreamSpeed(delta int) {
	idx := 0
	found := false
	for i, v := range speedPresets {
		if v == m.cfg.StreamSpeed {
			idx = i
			found = true
			break
		}
	}
	if !found {
		if delta > 0 {
			m.cfg.StreamSpeed = speedPresets[0]
		} else {
			m.cfg.StreamSpeed = speedPresets[len(speedPresets)-1]
		}
	} else {
		m.cfg.StreamSpeed = speedPresets[(idx+delta+len(speedPresets))%len(speedPresets)]
	}
	m.setStatus("Stream speed: "+formatSpeedLabel(m.cfg.StreamSpeed), "accent")
}

// startGeneration begins a 1-round or N-round generation run.
func (m *Model) startGeneration(roundsOverride int) tea.Cmd {
	if m.doc == nil || m.generationClient == nil || m.busy() {
		return nil
	}

	m.doc.SetFrontMatter(m.frontMatter.Value())
	m.doc.SetBody(m.editor.Value())
	m.pushUndo(m.doc.Body)

	guidance := strings.TrimSpace(m.prompt.Value())
	if guidance != "" {
		m.persistInstruction("generation", guidance)
		m.gen.lastGuidance = guidance
	} else if roundsOverride > 0 {
		// "+1 Round" can reuse last guidance or stay empty cleanly.
		guidance = m.gen.lastGuidance
	} else {
		m.gen.lastGuidance = ""
	}

	rounds := m.gen.rounds
	if roundsOverride > 0 {
		rounds = roundsOverride
	}
	rounds = clampInt(rounds, 1, 20)

	m.gen.runID++
	m.gen.active = true
	m.gen.totalRounds = rounds
	m.gen.currentRound = 0
	m.gen.plan = nil
	m.gen.currentIntent = ""
	m.gen.currentJoinMode = document.JoinAuto
	m.gen.pacer.Reset()

	// Switch focus to the document so the user watches the live stream clearly.
	m.writeFocus = writeFocusEditor
	m.syncFocus()

	if rounds > 1 {
		return m.startMultiRoundPlan(guidance, rounds)
	}
	return m.startNextGenerationRound()
}

func (m *Model) startMultiRoundPlan(guidance string, rounds int) tea.Cmd {
	m.gen.planning = true
	runID := m.gen.runID
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Timeout)
	m.gen.cancel = cancel

	req := engine.GenerationRequest{
		Body:              m.doc.Body,
		DocumentSystemMsg: m.doc.SystemMessage,
		ProjectRules:      m.cfg.Rules,
		Guidance:          guidance,
		Scope:             m.gen.scope,
		Overrides:         m.cfg.MessageOverrides,
	}
	client := m.generationClient
	m.setStatus(fmt.Sprintf("Planning %d %s rounds...", rounds, strings.ToLower(m.gen.scope.Label())), "accent")

	return tea.Batch(m.spin.Tick, func() tea.Msg {
		steps, err := engine.PlanRounds(ctx, client, req, rounds)
		return planDoneMsg{runID: runID, steps: steps, err: err}
	})
}

func (m *Model) handlePlanDone(msg planDoneMsg) (tea.Model, tea.Cmd) {
	if msg.runID != m.gen.runID || !m.gen.active {
		return m, nil
	}
	m.gen.planning = false
	m.gen.cancel = nil

	if msg.err != nil {
		if msg.err == context.Canceled {
			m.gen.active = false
			m.setStatus("Generation stopped", "info")
			return m, nil
		}
		// Graceful degradation: if planning fails on a very small model, proceed round-by-round without a pre-plan.
		m.gen.plan = nil
		m.setStatus("Proceeding without outline plan: "+msg.err.Error(), "warn")
	} else {
		m.gen.plan = msg.steps
	}
	return m, m.startNextGenerationRound()
}

func (m *Model) startNextGenerationRound() tea.Cmd {
	m.gen.currentRound++
	if m.gen.currentRound > m.gen.totalRounds {
		return m.finishGenerationRun(false)
	}

	m.gen.roundStarted = false
	m.gen.roundPrefixAdded = false
	m.gen.roundAddedChars = 0
	m.gen.currentJoinMode = document.JoinAuto
	m.gen.currentIntent = ""
	m.gen.pacer.Reset()

	var step *engine.PlanStep
	if m.gen.currentRound-1 < len(m.gen.plan) {
		s := m.gen.plan[m.gen.currentRound-1]
		step = &s
		m.gen.currentIntent = s.Title
	}

	req := engine.GenerationRequest{
		Body:                 m.editor.Value(),
		DocumentSystemMsg:    m.doc.SystemMessage,
		ProjectRules:         m.cfg.Rules,
		Guidance:             m.gen.lastGuidance,
		Scope:                m.gen.scope,
		RoundIndex:           m.gen.currentRound,
		RoundTotal:           m.gen.totalRounds,
		RoundStep:            step,
		StructuredGeneration: m.cfg.StructuredGeneration,
		Overrides:            m.cfg.MessageOverrides,
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.gen.cancel = cancel
	ch := make(chan llm.StreamEvent, 128)
	m.gen.ch = ch

	runID := m.gen.runID
	round := m.gen.currentRound
	client := m.generationClient

	go func() {
		defer close(ch)
		err := engine.StreamRound(ctx, client, req, func(ev llm.StreamEvent) error {
			select {
			case ch <- ev:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			select {
			case ch <- llm.StreamEvent{Err: err}:
			default:
			}
		}
	}()

	label := fmt.Sprintf("Generating %s", strings.ToLower(m.gen.scope.Label()))
	if m.gen.totalRounds > 1 {
		label = fmt.Sprintf("Round %d/%d", m.gen.currentRound, m.gen.totalRounds)
		if step != nil && step.Title != "" {
			label += " — " + step.Title
		}
	}
	m.setStatus(label+" · Esc stops", "accent")

	cmds := []tea.Cmd{m.spin.Tick, waitStreamChunk(ch, runID, round)}
	if m.cfg.StreamSpeed > 0 {
		cmds = append(cmds, schedulePacerTick(runID, round))
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleStreamChunk(msg streamChunkMsg) (tea.Model, tea.Cmd) {
	if msg.runID != m.gen.runID || msg.round != m.gen.currentRound || !m.gen.active {
		return m, nil
	}

	ev := msg.event
	if ev.Err != nil {
		m.gen.ch = nil
		m.gen.cancel = nil
		if ev.Err == context.Canceled {
			return m, m.finishGenerationRun(true)
		}
		m.gen.active = false
		m.gen.pacer.Clear()
		m.setStatus("Generation error (partial text kept): "+ev.Err.Error(), "error")
		return m, nil
	}

	if ev.JoinMode != "" {
		m.gen.currentJoinMode = document.JoinMode(ev.JoinMode)
	}
	if ev.Intent != "" {
		m.gen.currentIntent = ev.Intent
		if m.gen.totalRounds > 1 {
			m.setStatus(fmt.Sprintf("Round %d/%d — %s · Esc stops", m.gen.currentRound, m.gen.totalRounds, ev.Intent), "accent")
		} else {
			m.setStatus("Writing: "+ev.Intent+" · Esc stops", "accent")
		}
	}

	if ev.Delta != "" {
		m.gen.roundStarted = true
		if m.cfg.StreamSpeed <= 0 {
			m.appendRevealedText(ev.Delta)
		} else {
			wasEmpty := !m.gen.pacer.HasPending()
			m.gen.pacer.Push(ev.Delta)
			if wasEmpty {
				// Reveal immediately on first chunk and keep ticking smoothly.
				if revealed := m.gen.pacer.Drain(m.cfg.StreamSpeed, time.Now()); revealed != "" {
					m.appendRevealedText(revealed)
				}
			}
		}
	}

	if ev.Done {
		m.gen.ch = nil
		m.gen.cancel = nil
		m.gen.pacer.MarkUpstreamDone()
		if m.cfg.StreamSpeed <= 0 || m.gen.pacer.Finished() {
			return m, m.advanceAfterRound()
		}
		// Keep draining buffered text at the user's chosen reading speed!
		return m, schedulePacerTick(msg.runID, msg.round)
	}

	return m, waitStreamChunk(m.gen.ch, msg.runID, msg.round)
}

func (m *Model) handlePacerTick(msg pacerTickMsg) (tea.Model, tea.Cmd) {
	if msg.runID != m.gen.runID || msg.round != m.gen.currentRound || !m.gen.active {
		return m, nil
	}

	if revealed := m.gen.pacer.Drain(m.cfg.StreamSpeed, msg.at); revealed != "" {
		m.appendRevealedText(revealed)
	}

	if m.gen.pacer.Finished() {
		return m, m.advanceAfterRound()
	}
	if m.gen.ch != nil || m.gen.pacer.HasPending() {
		return m, schedulePacerTick(msg.runID, msg.round)
	}
	return m, nil
}

func (m *Model) appendRevealedText(delta string) {
	if delta == "" || m.doc == nil {
		return
	}
	current := m.editor.Value()
	chunk := delta
	if !m.gen.roundPrefixAdded {
		chunk = document.NormalizeJoinStart(current, delta, m.gen.currentJoinMode, string(m.gen.scope))
		if chunk != "" {
			m.gen.roundPrefixAdded = true
		}
	}
	if chunk == "" {
		return
	}
	updated := current + chunk
	m.gen.roundAddedChars += len([]rune(chunk))
	m.editor.SetValue(updated)
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.doc.SetBody(updated)
	m.lastEditAt = time.Now()
}

func (m *Model) advanceAfterRound() tea.Cmd {
	if !m.gen.active {
		return nil
	}
	if m.gen.currentRound < m.gen.totalRounds {
		return m.startNextGenerationRound()
	}
	return m.finishGenerationRun(false)
}

func (m *Model) finishGenerationRun(stopped bool) tea.Cmd {
	m.gen.active = false
	m.gen.planning = false
	m.gen.ch = nil
	if m.gen.cancel != nil {
		m.gen.cancel()
		m.gen.cancel = nil
	}
	m.gen.pacer.Clear()

	if err := m.saveCurrentDocument(); err != nil {
		m.setStatus("Generated text kept, but save failed: "+err.Error(), "error")
		return nil
	}
	if stopped {
		m.setStatus("Stopped right here & saved · Ctrl+Z undoes · Ctrl+R adds another round", "info")
		return nil
	}
	m.prompt.SetValue("")
	if m.gen.totalRounds > 1 {
		m.setStatus(fmt.Sprintf("Completed %d %s rounds & saved · Ctrl+R continues another round", m.gen.totalRounds, strings.ToLower(m.gen.scope.Label())), "success")
	} else {
		m.setStatus(m.gen.scope.Label()+" complete & saved · Ctrl+R adds another round · Ctrl+Z undoes", "success")
	}
	return nil
}

// stopGeneration immediately cancels the HTTP stream and discards unrevealed
// paced text so the document ends at the exact character visible on screen.
func (m *Model) stopGeneration() {
	if !m.gen.active && !m.gen.planning {
		return
	}
	m.gen.runID++ // invalidate any in-flight messages
	if m.gen.cancel != nil {
		m.gen.cancel()
		m.gen.cancel = nil
	}
	m.gen.pacer.Clear()
	m.gen.active = false
	m.gen.planning = false
	m.gen.ch = nil
	if err := m.saveCurrentDocument(); err != nil {
		m.setStatus("Stopped, save failed: "+err.Error(), "error")
		return
	}
	m.setStatus("Stopped at visible text & saved · Ctrl+Z undoes", "info")
}

func waitStreamChunk(ch <-chan llm.StreamEvent, runID, round int) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		ev, ok := <-ch
		if !ok {
			return streamChunkMsg{runID: runID, round: round, event: llm.StreamEvent{Done: true}}
		}
		return streamChunkMsg{runID: runID, round: round, event: ev}
	}
}

func schedulePacerTick(runID, round int) tea.Cmd {
	return tea.Tick(pacerTickInterval, func(t time.Time) tea.Msg {
		return pacerTickMsg{runID: runID, round: round, at: t}
	})
}
