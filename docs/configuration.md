---
layout: default
title: Configuration
nav_order: 4
---

# Configuration

goauthorllm reads an optional `.goauthorllm` file from the directory where the application is started. This YAML file can provide project-local defaults for `base_url` and `model`, and it can customize the embedded prompt messages without modifying Go source or rebuilding the binary.

## Precedence

For `base_url` and `model`, goauthorllm resolves values in this order:

1. The command-line flag (`--base-url`, `--model`)
2. The environment variable (`GOAUTHORLLM_BASE_URL`, `GOAUTHORLLM_MODEL`, then the `OPENAI_*` aliases)
3. The `.goauthorllm` file (`base_url`, `model`)
4. The built-in default

The `.goauthorllm` file is optional. If it is absent, or if either key is omitted, goauthorllm falls back to the higher-priority sources and then to the built-in defaults.

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
| prompt name | map | Prompt override for one embedded prompt |

Prompt override keys support two optional fields:

```yaml
base_url: http://localhost:11434/v1
model: gemma3:4b
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

If both `replace` and `append` are present for the same prompt, the application uses the `replace` text first and then appends the `append` text.

## Prompt Names

The following prompt names are available for customization:

| Name | Purpose |
|------|---------|
| `generate_prompt` | Base system prompt for generate mode |
| `edit_prompt` | Base system prompt for edit mode |
| `continue_prompt` | Task instructions for continuing the current section |
| `new_section_prompt` | Task instructions for writing the next section |
| `section_context_prompt` | Context label for each document section |
| `user_guidance_prompt` | Wrapper for user-provided generation guidance |
| `edit_task_prompt` | Detailed requirements for edit suggestions |
| `edit_history_prompt` | Template for prior edit decisions |
| `edit_feedback_prompt` | Template for correcting invalid suggestions |

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
