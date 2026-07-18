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

## Flags, Environment Variables, and Local Config

For `base_url` and `model`, precedence is:

1. Flag
2. Environment variable
3. `.goauthorllm`
4. Built-in default

The `.goauthorllm` file is optional and only provides connection-setting fallbacks for `base_url` and `model`.

| Setting | Environment Variable | Flag | `.goauthorllm` | Default | Description |
|---------|---------------------|------|----------------|---------|-------------|
| File path | `GOAUTHORLLM_FILE` | `--file` | not supported | chooser view | Markdown file to open. Also accepted as a positional argument. |
| Base URL | `GOAUTHORLLM_BASE_URL`, `OPENAI_BASE_URL` | `--base-url` | `base_url` | `http://localhost:11434/v1` | OpenAI-compatible endpoint URL. |
| Model | `GOAUTHORLLM_MODEL`, `OPENAI_MODEL` | `--model` | `model` | `gemma3:4b` | Model name sent to the endpoint. |
| API key | `GOAUTHORLLM_API_KEY`, `OPENAI_API_KEY` | `--api-key` | not supported | *(empty)* | Bearer token for the endpoint. |
| Timeout | `GOAUTHORLLM_TIMEOUT` | `--timeout` | not supported | `90s` | Request timeout as a Go duration string. |

## Screens

The application has five main screens:

1. **File Chooser** — lists markdown files in the current directory and accepts a new filename
2. **Mode Picker** — choose between Generate and Edit mode for the selected file
3. **Edit Options** — choose the built-in copy editor or enter custom directed-edit instructions
4. **Approval Mode** — choose manual review, automatic safety review, or approve all
5. **Workspace** — the document editor with mode-specific controls

## Generate Mode

Generate mode extends the document with model-generated markdown.

- Use the **editor** pane to write or review content
- Use the **guidance** pane to provide optional context for the next generation
- **Continue** streams additional text into the current section
- **New Section** asks the model to write the next heading and content

Generated content streams into the editor in real time and the document is saved automatically when generation completes.

## Edit Mode

Edit mode reviews the document and proposes one exact replacement at a time. Before opening the workspace, select an editor and an approval policy.

- **Copy Editor** uses the built-in copy-editing prompt
- **Custom Editor** accepts your directed-edit instructions, such as rewriting or removing a specified section
- The model sends a structured suggestion containing `old_text`, `new_text`, and an estimated `remaining_rounds`
- The application validates that `old_text` matches exactly one location
- If a suggestion is ambiguous or stale, a separate repair request asks the model to produce a uniquely matching replacement
- **Accept** applies the replacement, saves, and requests the next suggestion
- **Skip** records the suggestion as skipped and requests the next one
- **Refresh** requests a new suggestion without skipping the current one
- **Manual Review** shows every suggestion
- **Automatic Review** sends each suggestion through a second structured safety check. Copy edits are auto-applied only when they are unambiguously mechanical; directed edits are auto-applied only when they clearly satisfy your instructions. All other suggestions stay visible for review.
- **Approve All** applies every valid suggestion until the model reports that no useful rounds remain.
- The **History** button displays accepted, auto-accepted, approve-all, and skipped edits from the current session.

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
