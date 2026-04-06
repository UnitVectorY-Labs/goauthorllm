---
layout: default
title: Prompts
nav_order: 5
---

# Prompts

goauthorllm ships with embedded prompt templates that control how the application communicates with the LLM. These prompts are compiled into the binary and can be customized through the [`.goauthorllm` configuration file](configuration).

## How Prompts Work

Each interaction with the LLM is built from a sequence of messages. The application selects prompts based on the active mode and renders them with context-specific data before sending them to the model.

In **generate mode** the message sequence typically includes:
1. A system prompt setting the assistant's role
2. Document-level instructions from front matter or configuration
3. Section context for each existing section
4. Task instructions specific to continue or new section
5. Optional user guidance from the prompt pane

In **edit mode** the message sequence includes:
1. A system prompt for copy editing
2. Document-level instructions
3. Task requirements for structured output
4. The full document body
5. Edit history and optional feedback for retries

## Prompt Reference

Each prompt listed below links to its source file in the repository.

### generate_prompt

Sets the assistant's role and behavior for generate mode. This is the system message sent at the start of every generation request.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/generate_prompt.txt)

### edit_prompt

Sets the assistant's role for edit mode. Instructs the model to review the document and propose one fix at a time using structured output.

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_prompt.txt)

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

### edit_history_prompt

Template that summarizes prior edit decisions (accepted or skipped) so the model avoids repeating suggestions.

Template variable: `History` (list of entries with `Action`, `OldText`, `NewText`)

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_history_prompt.txt)

### edit_feedback_prompt

Template for correcting an invalid suggestion on retry. Sent when a previous suggestion failed validation.

Template variable: `Feedback`

[View source](https://github.com/UnitVectorY-Labs/goauthorllm/blob/main/internal/prompts/assets/edit_feedback_prompt.txt)
