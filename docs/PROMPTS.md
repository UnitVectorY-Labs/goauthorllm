---
layout: default
title: Prompts
nav_order: 5
---

# Prompts & Structured Outputs

`goauthorllm` uses lean, KV-cache-friendly two-message prompts (`system` + `user`) combined with structured JSON schemas so smaller local models stay focused, accurate, and cleanly integrated with existing markdown spacing and newlines.

## Generation Architecture

- **Lean Context**: Sends the document inside `<document>` with a `<document_tail>` anchor (automatically compacting very long documents into an earlier-headings outline plus the recent tail) instead of repeating fake multi-turn history.
- **Structured Streaming (`structured_generation: true`)**: Streams a JSON schema response (`join_mode`, `intent`, `content`) token-by-token. The model explicitly chooses how its addition connects (`inline_direct`, `inline_space`, `new_paragraph`, or `new_section`), states a short `intent`, and streams `content` live into the editor as characters are unescaped. If an endpoint rejects `response_format` with `stream: true`, `goauthorllm` automatically falls back to plain streaming with deterministic boundary joining.
- **Multi-Round Planning (`rounds > 1`)**: Runs a quick structured outline pass (`plan_prompt`) to plan `N` distinct steps before streaming each round sequentially so models do not stall early or repeat themselves.

## Editing Architecture

- **Line-Numbered Context**: Sends the document formatted with 1-based line numbers (`1| ...`) so the model anchors edits cleanly even when lines or phrases repeat.
- **Structured Edits**: Returns `{assessment, edits: [{reason, target_line, action, old_text, new_text}], more_rounds_needed}` where `action` is `replace`, `delete`, or `insert_after`.
- **Deterministic Whitespace Integration**: Each edit is resolved near `target_line` and applied with smart newline/space normalization — deleting a paragraph never leaves triple blank lines, and inline replacements preserve surrounding spacing.

## Customizable Prompt Names

All embedded prompts can be overridden in `.goauthorllm`, via environment variables (`GOAUTHORLLM_<NAME>_FILE` / `_APPEND_FILE`), or via `--prompt-file NAME=PATH` / `--prompt-append-file NAME=PATH`:

| Name | Purpose |
|------|---------|
| `generate_prompt` | System prompt for content generation |
| `continue_prompt` | Task instructions for `sentence`, `paragraph`, and `free` scopes (`HasDocument`, `ScopeInstruction`, `RoundLabel`) |
| `new_section_prompt` | Task instructions for `section` scope |
| `plan_prompt` | System prompt for multi-round outline planning |
| `user_guidance_prompt` | Wrapper for optional author guidance (`Prompt`) |
| `edit_prompt` | System prompt for copy editing |
| `directed_edit_prompt` | System prompt for custom/directed editing instructions |
| `edit_task_prompt` | Task instructions for copy editing (`BatchSize`) |
| `directed_edit_task_prompt` | Task instructions for custom/directed editing (`BatchSize`) |
| `edit_history_prompt` | Session history summary (`History`) |
| `edit_feedback_prompt` | Validation feedback sent on retry (`Feedback`) |
