---
layout: default
title: Prompts
nav_order: 5
---

# Prompts

goauthorllm ships with embedded prompt templates that control how the application communicates with the LLM. These prompts are compiled into the binary and can be customized through [configuration](configuration), environment variables, or file flags.

## Supplying Prompt Files

In `.goauthorllm`, each prompt accepts inline `replace` / `append` text or file-backed `replace_file` / `append_file` values:

```yaml
generate_prompt:
  replace_file: prompts/generate.txt
continue_prompt:
  append_file: prompts/continue-rules.txt
```

The equivalent repeatable flags use `NAME=PATH`:

```bash
goauthorllm --prompt-file generate_prompt=prompts/generate.txt \
  --prompt-append-file continue_prompt=prompts/continue-rules.txt
```

For environment-only configuration, use `GOAUTHORLLM_<PROMPT_NAME>_FILE` to replace a prompt or `GOAUTHORLLM_<PROMPT_NAME>_APPEND_FILE` to append to it. For example:

```bash
export GOAUTHORLLM_GENERATE_PROMPT_FILE="$PWD/prompts/generate.txt"
export GOAUTHORLLM_CONTINUE_PROMPT_APPEND_FILE="$PWD/prompts/continue-rules.txt"
```

Flags override environment-backed files, which override `.goauthorllm`. Template replacements must retain the template variables documented below when the application needs them. Generation guidance (`--guidance-file`) and directed-edit instructions (`--edit-instructions-file`) are per-operation inputs, not replacements for these application prompts.

## How Prompts Work

Each interaction with the LLM is built from a sequence of messages. The application selects prompts based on the active mode and renders them with context-specific data before sending them to the model.

In **generate mode** the message sequence typically includes:
1. A system prompt setting the assistant's role
2. Document-level instructions from front matter or configuration
3. Section context for each existing section
4. Task instructions specific to continue or new section
5. Optional user guidance from the prompt pane

In **edit mode** the suggestion message sequence includes:
1. A system prompt specific to either copy editing or the author's directed editing task
2. Document-level instructions
3. Task requirements for structured output
4. The full document body
5. Edit history and optional feedback for retries

Automatic Review additionally sends the proposed edit to a separate structured approval request. If a replacement does not occur exactly once, a separate structured repair request is used before it can be shown or applied.

## Prompt Reference

Each prompt listed below links to its source file in the repository.

### generate_prompt

Sets the assistant's role and behavior for generate mode. This is the system message sent at the start of every generation request.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/generate_prompt.txt)

### edit_prompt

Sets the assistant's role for edit mode. Instructs the model to review the document and propose one fix at a time using structured output.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_prompt.txt)

### directed_edit_prompt

Sets the assistant role for custom directed edits. It makes the author's task the sole priority, explicitly excludes unrelated copy editing, and stops once the task is satisfied.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/directed_edit_prompt.txt)

### continue_prompt

Task instructions for continuing the current section. This is a Go template that receives the section label and whether there is an excerpt to continue from.

Template variables: `SectionLabel`, `HasExcerpt`

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/continue_prompt.txt)

### new_section_prompt

Task instructions for writing the next document section. Asks the model to start with an appropriate heading.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/new_section_prompt.txt)

### section_context_prompt

Provides context for each existing section in the document. Used to give the model awareness of the document structure.

Template variables: `Index`, `Total`

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/section_context_prompt.txt)

### user_guidance_prompt

Wraps user-provided guidance from the prompt pane into a structured message for the model.

Template variable: `Prompt`

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/user_guidance_prompt.txt)

### edit_task_prompt

Detailed requirements for edit suggestions. Instructs the model on how to format the `old_text` and `new_text` fields and what constitutes a valid suggestion.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_task_prompt.txt)

### directed_edit_task_prompt

Defines the structured batch used by directed editing: up to 10 validated, non-overlapping `old_text`/`new_text` replacements spanning different parts of the document.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/directed_edit_task_prompt.txt)

### edit_history_prompt

Template that summarizes prior edit decisions (accepted or skipped) so the model avoids repeating suggestions.

Template variable: `History` (list of entries with `Action`, `OldText`, `NewText`)

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_history_prompt.txt)

### edit_feedback_prompt

Template for correcting an invalid suggestion on retry. Sent when a previous suggestion failed validation.

Template variable: `Feedback`

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_feedback_prompt.txt)

### edit_approval_prompt

Sets the system role for automatic review. It allows copy edits to be accepted only when they are unambiguously mechanical, and directed edits only when they clearly follow the author's instructions.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_approval_prompt.txt)

### edit_repair_prompt

Sets the system role for repairing an invalid or ambiguous replacement so `old_text` can match exactly once.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_repair_prompt.txt)
