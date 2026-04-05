---
layout: default
title: Usage
nav_order: 3
---

# Usage

## Running the Application

Start goauthorllm with no arguments to open the file chooser:

```bash
goauthorllm
```

Open a specific file directly:

```bash
goauthorllm draft.md
```

Or use the `--file` flag:

```bash
goauthorllm --file draft.md
```

## Flags and Environment Variables

Flags take precedence over environment variables.

| Setting | Environment Variable | Flag | Default | Description |
|---------|---------------------|------|---------|-------------|
| File path | `GOAUTHORLLM_FILE` | `--file` | chooser view | Markdown file to open. Also accepted as a positional argument. |
| Base URL | `GOAUTHORLLM_BASE_URL`, `OPENAI_BASE_URL` | `--base-url` | `http://localhost:11434/v1` | OpenAI-compatible endpoint URL. |
| Model | `GOAUTHORLLM_MODEL`, `OPENAI_MODEL` | `--model` | `gemma3:4b` | Model name sent to the endpoint. |
| API key | `GOAUTHORLLM_API_KEY`, `OPENAI_API_KEY` | `--api-key` | *(empty)* | Bearer token for the endpoint. |
| Timeout | `GOAUTHORLLM_TIMEOUT` | `--timeout` | `90s` | Request timeout as a Go duration string. |

## Screens

The application has three main screens:

1. **File Chooser** — lists markdown files in the current directory and accepts a new filename
2. **Mode Picker** — choose between Generate and Edit mode for the selected file
3. **Workspace** — the document editor with mode-specific controls

## Generate Mode

Generate mode extends the document with model-generated markdown.

- Use the **editor** pane to write or review content
- Use the **guidance** pane to provide optional context for the next generation
- **Continue** streams additional text into the current section
- **New Section** asks the model to write the next heading and content

Generated content streams into the editor in real time and the document is saved automatically when generation completes.

## Edit Mode

Edit mode reviews the document and proposes one copy edit at a time.

- The model sends a structured suggestion containing `old_text` and `new_text`
- The application validates that `old_text` matches exactly one location
- **Accept** applies the replacement, saves, and requests the next suggestion
- **Skip** records the suggestion as skipped and requests the next one
- **Refresh** requests a new suggestion without skipping the current one

## Keyboard Controls

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Move focus between panes and buttons |
| `Ctrl+S` | Save the document |
| `Ctrl+O` | Return to file selection |
| `Ctrl+Q` | Quit the application |
| `Esc` | Back out of the current view or cancel an active request |
| `Enter` | Activate the focused button or add a newline in a text area |
| `PgUp` / `PgDn` | Page through the document editor |

### Generate Mode

| Key | Action |
|-----|--------|
| `Ctrl+G` | Continue the current section |
| `Ctrl+N` | Start a new section |

### Edit Mode

| Key | Action |
|-----|--------|
| `Ctrl+A` | Accept the current suggestion |
| `Ctrl+K` | Skip the current suggestion |
| `Ctrl+R` | Refresh the suggestion |

## Mouse Support

- **Scroll wheel** moves through the editor, prompt, or front matter pane
- **Click** buttons, file list entries, and mode cards
- **Click** panes to switch focus

## Autosave

The document is saved automatically every two seconds after a brief idle period. Manual save with `Ctrl+S` is always available.

## Front Matter

The **Message** button opens a modal editor for the document's YAML front matter. The `system_message` field in the front matter is sent to the LLM as document-specific instructions alongside the embedded prompts.

Front matter is optional. If you prefer not to use front matter, you can provide document-level guidance through the `.goauthorllm` configuration file instead.
