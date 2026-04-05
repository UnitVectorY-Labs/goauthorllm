# Configuration

goauthorllm reads a local `.goauthorllm` file from the directory you start the app in. The file is YAML and is intended to customize the baked-in prompt messages without editing Go source.

## Goals

- Replace a built-in prompt entirely when you need a different tone or policy
- Append extra instructions to the default prompt when you only need a small local override
- Keep document-specific prompt tweaks close to the working folder

## Structure

The configuration format is intentionally simple. The root keys are the prompt message names:

```yaml
generate_prompt:
  append: |
    Prefer concise paragraphs and avoid filler.
edit_prompt:
  replace: |
    You are a careful copy editor. Return one high-priority fix at a time.
continue_prompt:
  append: |
    Keep transitions tight.
```

### Fields

- Each root key is a named embedded prompt message
- `generate_prompt` controls the base system prompt used by generate mode
- `edit_prompt` controls the base system prompt used by edit mode
- Additional prompt names cover task instructions, section-context instructions, edit history feedback, and other app-generated instruction messages
- `append` adds text after the baked-in system prompt
- `replace` swaps out the baked-in system prompt entirely

If both `replace` and `append` are present, the app uses `replace` first and then appends `append`.

## Prompt Names

The prompt text lives in embedded assets under `internal/prompts/assets/` and is compiled into the binary. `.goauthorllm` customizes those embedded defaults without changing Go source.

Current built-in prompt names include:

- `generate_prompt`
- `edit_prompt`
- `section_context_prompt`
- `continue_prompt`
- `new_section_prompt`
- `user_guidance_prompt`
- `edit_task_prompt`
- `edit_history_prompt`
- `edit_feedback_prompt`

## Example

```yaml
generate_prompt:
  append: |
    Keep the document grounded and direct.
edit_prompt:
  replace: |
    You are an expert copy editor. Return exactly one suggestion with a match string and replacement string.
edit_task_prompt:
  append: |
    Prioritize terminology and clarity before smaller style tweaks.
```

If `.goauthorllm` is absent, the app should use the embedded defaults.
