---
layout: default
title: Configuration
nav_order: 4
---

# Configuration

`goauthorllm` reads an optional `.goauthorllm` YAML file from the working directory — and you can edit, validate, save, and hot-reload `.goauthorllm` directly inside the application from the **Config** tab (`^T` or `^P` → `3`).

## Precedence

Configurable values resolve in this order:

1. Command-line flag (`--base-url`, `--model`, `--stream-speed`, etc.)
2. Environment variable (`GOAUTHORLLM_*`, plus `OPENAI_BASE_URL`, `OPENAI_MODEL`, `OPENAI_API_KEY`)
3. `.goauthorllm` in the current directory
4. Built-in default

## In-App Config Editor

Switch to **Config** (`^T` to cycle views, or `^P` → `3`) at any time to edit `.goauthorllm`:

- **Form view** — edit endpoint URL, default/generation/editing models, universal stream speed (`stream_chars_per_sec`), default generation length, structured streaming toggle, edit batch sizes, project writing `rules`, and default `copy_edit_rules`.
- **Fetch Models (`^F` / `Pick…`)** — queries `<base_url>/models` live and lets you filter and pick any available model.
- **Raw YAML view (`^Y`)** — inspect or edit `.goauthorllm` directly, including advanced prompt overrides.
- **Save & Apply (`^S`)** — writes `.goauthorllm` and reloads the active LLM clients immediately without restarting.

## Supported Keys

| Key | Type | Default | Purpose |
|-----|------|---------|---------|
| `base_url` | string | `http://localhost:11434/v1` | OpenAI-compatible endpoint URL |
| `model` | string | `gemma3:4b` | Default model ID |
| `generation_model` | string | *(value of `model`)* | Optional model override for generation |
| `editing_model` | string | *(value of `model`)* | Optional model override for editing |
| `stream_chars_per_sec` | integer | `0` (`Max`) | Streaming reveal speed in characters/sec (`0` = unthrottled) |
| `default_scope` | string | `paragraph` | Default generation scope: `sentence`, `paragraph`, `section`, or `free` |
| `structured_generation` | boolean | `true` | Stream structured JSON (`join_mode`, `intent`, `content`) with automatic plain fallback |
| `rules` | string | *(empty)* | Project-wide writing rules applied to every generation |
| `rules_file` | path | *(empty)* | File containing project-wide writing rules |
| `copy_edit_rules` | string | *(empty)* | Custom default copy-editing instructions |
| `copy_edit_rules_file` | path | *(empty)* | File containing custom copy-editing rules |
| `copy_edit_batch_size` | integer | `5` | Maximum copy edits requested per pass |
| `directed_edit_batch_size` | integer | `10` | Maximum directed edits requested per pass |
| `rounds` | integer | `1` | Default number of generation rounds |
| `max_edits` | integer | `0` | Maximum edits applied in non-interactive mode (`0` = unlimited) |
| prompt override key | map | *(none)* | `replace`, `append`, `replace_file`, or `append_file` for any embedded prompt |

## Example `.goauthorllm`

```yaml
base_url: http://localhost:11434/v1
model: your-model-name
stream_chars_per_sec: 65
default_scope: paragraph
structured_generation: true
copy_edit_batch_size: 5
directed_edit_batch_size: 10
rules: |
  Write clear, direct technical prose in active voice.
  Keep headings concise and avoid filler introductions.
copy_edit_rules: |
  Fix grammar, spelling, and awkward phrasing.
  Prefer concise sentences and consistent Markdown formatting.
```
