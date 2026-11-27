package app

import (
	"time"
)

const pacerTickInterval = 33 * time.Millisecond // ~30 fps smooth reveal

// streamPacer buffers incoming LLM deltas and releases them at a configurable
// characters-per-second rate so the user can comfortably read as content
// streams in and stop at the exact visible point.
type streamPacer struct {
	queue        []rune
	carry        float64
	upstreamDone bool
	lastTick     time.Time
}

func (p *streamPacer) Reset() {
	p.queue = p.queue[:0]
	p.carry = 0
	p.upstreamDone = false
	p.lastTick = time.Now()
}

func (p *streamPacer) Push(text string) {
	if text == "" {
		return
	}
	p.queue = append(p.queue, []rune(text)...)
}

func (p *streamPacer) MarkUpstreamDone() {
	p.upstreamDone = true
}

func (p *streamPacer) HasPending() bool {
	return len(p.queue) > 0
}

func (p *streamPacer) Finished() bool {
	return p.upstreamDone && len(p.queue) == 0
}

// Clear immediately discards all buffered, unrevealed text (used when the user
// stops mid-stream so the document ends right where they read up to).
func (p *streamPacer) Clear() {
	p.queue = p.queue[:0]
	p.carry = 0
	p.upstreamDone = true
}

// Drain returns the next slice of text to reveal based on charsPerSec and elapsed time.
// If charsPerSec <= 0, all pending text is returned immediately.
func (p *streamPacer) Drain(charsPerSec int, now time.Time) string {
	if len(p.queue) == 0 {
		p.lastTick = now
		return ""
	}
	if charsPerSec <= 0 {
		out := string(p.queue)
		p.queue = p.queue[:0]
		p.carry = 0
		p.lastTick = now
		return out
	}

	if p.lastTick.IsZero() {
		p.lastTick = now
	}
	elapsed := now.Sub(p.lastTick).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > 0.25 {
		elapsed = 0.25 // cap burst after pause
	}
	p.lastTick = now

	p.carry += elapsed * float64(charsPerSec)
	take := int(p.carry)
	if take <= 0 {
		return ""
	}
	p.carry -= float64(take)
	if take > len(p.queue) {
		take = len(p.queue)
	}
	out := string(p.queue[:take])
	p.queue = p.queue[take:]
	return out
}
