---
layout: default
title: Configuration
nav_order: 4
---

# Configuration

goauthorllm reads an optional `.goauthorllm` file from the directory where the application is started. This YAML file customizes the embedded prompt messages without modifying Go source or rebuilding the binary.

## When to Use It

The `.goauthorllm` file is useful when:

- You want a consistent tone or set of instructions for all documents in a folder
- You need to replace a built-in prompt entirely for a specific project
- You want to append extra guidance to the default prompts without losing the baseline

This file is also the recommended way to provide a document-level system message when you do not want to use front matter in the document itself.

## Structure

The root keys are prompt names. Each key supports two optional fields:

```yaml
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

If the `.goauthorllm` file is absent, the application uses the embedded defaults without changes.
