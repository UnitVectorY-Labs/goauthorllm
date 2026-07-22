---
layout: default
title: Configuration
nav_order: 4
---

# Configuration

goauthorllm reads an optional `.goauthorllm` file from the directory where the application is started. This YAML file can provide project-local defaults for connection and operation settings, and it can customize embedded prompts without modifying Go source or rebuilding the binary. It is never required, including for non-interactive use.

## Precedence

goauthorllm resolves configurable values in this order:

1. The command-line flag (`--base-url`, `--model`)
2. The environment variable (`GOAUTHORLLM_BASE_URL`, `GOAUTHORLLM_MODEL`, then the `OPENAI_*` aliases)
3. The `.goauthorllm` file (`base_url`, `model`)
4. The built-in default

The `.goauthorllm` file is optional. If it is absent, or if either key is omitted, goauthorllm falls back to the higher-priority sources and then to the built-in defaults.

Edit batch sizes follow the same precedence. Both must be positive integers. `max_edits` may be zero, meaning unlimited.

## When to Use It

The `.goauthorllm` file is useful when:

- You want a consistent tone or set of instructions for all documents in a folder
- You want a project-local default endpoint or model without exporting shell variables
- You need to replace a built-in prompt entirely for a specific project
- You want to append extra guidance to the default prompts without losing the baseline

This file is also the recommended way to provide a document-level system message when you do not want to use front matter in the document itself.

## Structure

The file supports these optional top-level keys:

| Key | Type | Purpose |
|-----|------|---------|
| `base_url` | string | Project-local default OpenAI-compatible endpoint URL |
| `model` | string | Project-local default model name |
| `generation_model` | string | Optional model for generation; falls back to `model` |
| `editing_model` | string | Optional model for copy and directed editing; falls back to `model` |
| `copy_edit_batch_size` | integer | Maximum copy-edit suggestions per batch; default `1` |
| `directed_edit_batch_size` | integer | Maximum directed-edit suggestions per batch; default `10` |
| `mode` | string | Non-interactive operation: `generate` or `edit` |
| `submode` | string | `continue` / `new-section` for generation, or `copy` / `directed` for editing |
| `approval` | string | Non-interactive edit approval: `approve-all` or `llm-approved` |
| `max_edits` | integer | Maximum edits applied in one non-interactive run; `0` is unlimited |
| `guidance` | string | Optional generation guidance |
| `guidance_file` | path | File containing generation guidance; takes precedence over `guidance` at this level |
| `edit_instructions` | string | Instructions for directed editing |
| `edit_instructions_file` | path | File containing directed edit instructions; takes precedence at this level |
| prompt name | map | Prompt override for one embedded prompt |

Prompt override keys support four optional fields:

```yaml
base_url: http://localhost:11434/v1
model: gemma3:4b
generation_model: optional-generation-model
editing_model: optional-editing-model
copy_edit_batch_size: 1
directed_edit_batch_size: 10
max_edits: 0
generate_prompt:
  append: |
    Keep the writing concise and avoid filler.
edit_prompt:
  replace: |
    You are a careful copy editor. Return one high-priority fix at a time.
continue_prompt:
  append: |
    Keep transitions tight.
```

### Fields

| Field | Effect |
|-------|--------|
| `append` | Adds text after the built-in prompt |
| `replace` | Substitutes the built-in prompt entirely |
| `append_file` | Reads text from a file and uses it as `append` |
| `replace_file` | Reads text from a file and uses it as `replace` |

If both `replace` and `append` are present for the same prompt, the application uses the `replace` text first and then appends the `append` text.

Paths are resolved from the directory where goauthorllm is started. A file-backed value replaces the corresponding inline field at the same configuration level.

## Flag and Environment Reference

Operation settings can be supplied without a config file:

| Config key | Flag | Environment variable |
|------------|------|----------------------|
| `mode` | `--mode` | `GOAUTHORLLM_MODE` |
| `submode` | `--submode` | `GOAUTHORLLM_SUBMODE` |
| `approval` | `--approval` | `GOAUTHORLLM_APPROVAL` |
| `max_edits` | `--max-edits` | `GOAUTHORLLM_MAX_EDITS` |
| `guidance` | `--guidance` | `GOAUTHORLLM_GUIDANCE` |
| `guidance_file` | `--guidance-file` | `GOAUTHORLLM_GUIDANCE_FILE` |
| `edit_instructions` | `--edit-instructions` | `GOAUTHORLLM_EDIT_INSTRUCTIONS` |
| `edit_instructions_file` | `--edit-instructions-file` | `GOAUTHORLLM_EDIT_INSTRUCTIONS_FILE` |
| `copy_edit_batch_size` | `--copy-edit-batch-size` | `GOAUTHORLLM_COPY_EDIT_BATCH_SIZE` |
| `directed_edit_batch_size` | `--directed-edit-batch-size` | `GOAUTHORLLM_DIRECTED_EDIT_BATCH_SIZE` |

`--non-interactive` itself must be given explicitly; placing other operation defaults in `.goauthorllm` does not disable the TUI.

## Prompt Names

The following prompt names are available for customization:

| Name | Purpose |
|------|---------|
| `generate_prompt` | Base system prompt for generate mode |
| `edit_prompt` | Base system prompt for edit mode |
| `directed_edit_prompt` | System prompt dedicated to the author's directed editing task |
| `continue_prompt` | Task instructions for continuing the current section |
| `new_section_prompt` | Task instructions for writing the next section |
| `section_context_prompt` | Context label for each document section |
| `user_guidance_prompt` | Wrapper for user-provided generation guidance |
| `edit_task_prompt` | Detailed requirements for edit suggestions |
| `directed_edit_task_prompt` | Requirements for batches of up to 10 directed replacements |
| `edit_history_prompt` | Template for prior edit decisions |
| `edit_feedback_prompt` | Template for correcting invalid suggestions |
| `edit_approval_prompt` | System prompt for the optional LLM approval pass |
| `edit_repair_prompt` | System prompt for repairing ambiguous replacements |

See the [Prompts](prompts) page for details on each prompt's purpose and default content.

## Example

A `.goauthorllm` file for a technical documentation project:

```yaml
base_url: https://openrouter.ai/api/v1
model: google/gemma-3-4b-it
generate_prompt:
  append: |
    Keep the document grounded and direct.
    Avoid speculative language and filler.
edit_prompt:
  replace: |
    You are an expert technical editor. Return exactly one suggestion
    with the exact text to replace and the corrected replacement.
edit_task_prompt:
  append: |
    Prioritize terminology consistency and clarity
    before smaller style tweaks.
```

If the `.goauthorllm` file is absent, the application uses the embedded prompt defaults and the built-in connection defaults unless a flag or environment variable overrides them.
