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

For configurable values, precedence is:

1. Flag
2. Environment variable
3. `.goauthorllm`
4. Built-in default

The `.goauthorllm` file is optional and provides project-local fallbacks for `base_url`, the default `model`, and optional generation- and editing-specific models.

| Setting | Environment Variable | Flag | `.goauthorllm` | Default | Description |
|---------|---------------------|------|----------------|---------|-------------|
| File path | `GOAUTHORLLM_FILE` | `--file` | not supported | chooser view | Markdown file to open. Also accepted as a positional argument. |
| Base URL | `GOAUTHORLLM_BASE_URL`, `OPENAI_BASE_URL` | `--base-url` | `base_url` | `http://localhost:11434/v1` | OpenAI-compatible endpoint URL. |
| Model | `GOAUTHORLLM_MODEL`, `OPENAI_MODEL` | `--model` | `model` | `gemma3:4b` | Model name sent to the endpoint. |
| Generation model | `GOAUTHORLLM_GENERATION_MODEL` | `--generation-model` | `generation_model` | value of `model` | Optional model used only for generation requests. |
| Editing model | `GOAUTHORLLM_EDITING_MODEL` | `--editing-model` | `editing_model` | value of `model` | Optional model used for copy and directed editing requests. |
| API key | `GOAUTHORLLM_API_KEY`, `OPENAI_API_KEY` | `--api-key` | not supported | *(empty)* | Bearer token for the endpoint. |
| Timeout | `GOAUTHORLLM_TIMEOUT` | `--timeout` | not supported | `90s` | Timeout for non-streaming LLM requests (such as edits), as a Go duration string. Streaming generation has no total timeout and can be stopped from the UI. |
| Copy-edit batch size | `GOAUTHORLLM_COPY_EDIT_BATCH_SIZE` | `--copy-edit-batch-size` | `copy_edit_batch_size` | `1` | Maximum suggestions requested in each copy-edit batch. |
| Directed-edit batch size | `GOAUTHORLLM_DIRECTED_EDIT_BATCH_SIZE` | `--directed-edit-batch-size` | `directed_edit_batch_size` | `10` | Maximum suggestions requested in each directed-edit batch. |
| Mode | `GOAUTHORLLM_MODE` | `--mode` | `mode` | none | Non-interactive operation: `generate` or `edit`. |
| Sub-mode | `GOAUTHORLLM_SUBMODE` | `--submode` | `submode` | none | Generation: `continue` or `new-section`; editing: `copy` or `directed`. |
| Approval | `GOAUTHORLLM_APPROVAL` | `--approval` | `approval` | none | Non-interactive editing: `approve-all` or `llm-approved`. |
| Maximum edits | `GOAUTHORLLM_MAX_EDITS` | `--max-edits` | `max_edits` | `0` | Maximum edits applied during one non-interactive run; `0` is unlimited. |
| Generation guidance | `GOAUTHORLLM_GUIDANCE`, `GOAUTHORLLM_GUIDANCE_FILE` | `--guidance`, `--guidance-file` | `guidance`, `guidance_file` | empty | Optional instructions for one generation. |
| Directed edit instructions | `GOAUTHORLLM_EDIT_INSTRUCTIONS`, `GOAUTHORLLM_EDIT_INSTRUCTIONS_FILE` | `--edit-instructions`, `--edit-instructions-file` | `edit_instructions`, `edit_instructions_file` | empty | Required for directed editing. |

## Non-Interactive Mode

Use `--non-interactive` for a single, fully scripted operation. This flag is deliberately required: without it, goauthorllm launches the TUI. The document must also be passed on the command line, either positionally or with `--file`.

Generate a continuation:

```bash
goauthorllm --non-interactive --mode generate --submode continue draft.md
```

Start a new section using guidance from a file:

```bash
goauthorllm --non-interactive --mode generate --submode new-section \
  --guidance-file next-section.txt draft.md
```

Apply every valid copy edit, stopping after five changes:

```bash
goauthorllm --non-interactive --mode edit --submode copy \
  --approval approve-all --max-edits 5 draft.md
```

Run a directed edit with a second LLM approval pass:

```bash
goauthorllm --non-interactive --mode edit --submode directed \
  --approval llm-approved --edit-instructions-file rewrite.txt draft.md
```

Generation writes the new content to the document and prints that generated addition to standard output. Editing saves each applied replacement and prints its old and new text in a simple `--- old` / `+++ new` format. Status and error messages go to standard error. There are no prompts or manual approval fallbacks in this mode: `llm-approved` skips suggestions rejected by the approval model, while `approve-all` applies every validated suggestion.

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Content was generated and saved, or at least one edit was applied and saved. |
| `1` | The operation failed, such as an LLM, file, prompt, or output error. |
| `2` | Command configuration or arguments were invalid. |
| `3` | The operation completed normally but made no change. This includes empty generation output and an edit pass with no applied suggestions. |

The `.goauthorllm` file remains optional in non-interactive mode. Endpoint, model, operation settings, guidance, instructions, batch sizes, edit limits, and prompt overrides can all come from flags or environment variables. See [Configuration](configuration) and [Prompts](prompts) for the complete mapping.

Prompt templates use the repeatable `--prompt-file NAME=PATH` and `--prompt-append-file NAME=PATH` flags, or prompt-specific environment variables documented on the [Prompts](prompts) page.

## Screens

The application has five main screens:

1. **File Chooser** — lists markdown files in the current directory and accepts a new filename
2. **Mode Picker** — choose View/Edit Document, Generate, or Edit with AI from a keyboard-and-mouse list
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

## View / Edit Document Mode

This mode opens the document without starting an LLM request. Use it to read or manually edit Markdown, document instructions, and front matter.

## Edit Mode

Edit mode reviews the document and proposes exact replacements. Before opening the workspace, select an editor and an approval policy.

The mode, editor, and approval screens use standard list navigation: move with `↑`/`↓`, choose with `Enter`, or click an item. Custom instructions can be entered directly. Press `Enter` to continue, or `Alt+Enter` to insert a newline. Use `Alt+↑` / `Alt+↓` to recall instructions previously used with this document.

- **Copy Editor** uses the built-in copy-editing prompt
- **Custom Editor** accepts your directed-edit instructions, such as rewriting or removing a specified section
- Copy editing returns a structured batch of `old_text`/`new_text` suggestions (one by default; configure `copy_edit_batch_size` to change it).
- Directed editing returns a structured batch of up to 10 `old_text`/`new_text` suggestions by default; configure `directed_edit_batch_size` to change it.
- The application validates that `old_text` matches exactly one location
- If a suggestion is ambiguous or stale, a separate repair request asks the model to produce a uniquely matching replacement
- **Accept** applies the replacement, saves, and requests the next suggestion
- **Skip** records the suggestion as skipped and requests the next one
- **Refresh** requests a new suggestion without skipping the current one
- **Manual Review** shows every suggestion
- **Automatic Review** sends each suggestion through a second structured safety check. Copy edits are auto-applied only when they are unambiguously mechanical; directed edits are auto-applied only when they clearly satisfy your instructions. All other suggestions stay visible for review.
- **Approve All** applies every valid suggestion until the model reports that no useful rounds remain.
- The edit workspace has **Suggestion**, **History**, and **Document** tabs. A suggestion remains visible while automatic review is running or when automatic review declines it.
- The **History** tab displays accepted, auto-accepted, approve-all, and skipped edits from the current session. Use `←`/`→` to page through full old/new diffs.

The Generate workspace has separate **Document** and **Guidance** tabs so each editor has the available screen height.

## Keyboard Controls

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Move forward/backward through workspace tabs; in custom setup, move between instructions and Next |
| `Ctrl+S` | Save the document |
| `Ctrl+O` | Return to file selection |
| `Ctrl+Q` | Quit the application |
| `Alt+E` | Open or close the last error in a wrapped, scrollable view; use ↑/↓ or PgUp/PgDn to scroll and Esc to close |
| `Esc` | Back out of the current view or cancel an active request |
| `Enter` | Submit generation guidance, continue from directed instructions, activate a button, or insert a newline in the document |
| `Alt+Enter` | Insert a newline in generation guidance or directed instructions |
| `Alt+↑` / `Alt+↓` | Recall older/newer instructions in a guidance field; moving past the newest entry restores your draft |
| `Ctrl+Z` | Undo a generation, bulk deletion, or recent manual typing in the document editor |
| `Alt+K` | Delete from the document cursor to the end; `Ctrl+Z` restores the passage |
| `PgUp` / `PgDn` | Page through the document editor or scroll an edit suggestion/history entry |
| `↑` / `↓` | Scroll the current edit suggestion/history entry |
| `←` / `→` | Change a focused workspace tab, or page through edit history |
| `Alt+M` | Open document instructions |
| `Alt+H` | Open edit history in Edit mode |

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

## Instruction History and Backtracking

Generation guidance and directed-edit instructions are saved in a SQLite sidecar beside the document: `draft.md.goauthorllm.sqlite` for `draft.md`. Each file has its own history, with separate entries for generation and directed editing. Submitted instructions and nonempty drafts left through Back or Quit are retained across launches. Consecutive identical entries are stored once. Move or rename the sidecar with the document to keep their history together.

While typing guidance or directed instructions, use `Alt+↑` and `Alt+↓` to browse saved entries. Returning past the newest entry restores the draft you were typing. Recalled instructions remain editable before submission.

In Generate mode, `Ctrl+Z` in the Document tab restores the state before the last generation, including a canceled partial generation. Recent manual typing is also undoable. To remove a larger passage, put the cursor at its beginning and press `Alt+K` to remove everything through the end of the document. You can undo this deletion with `Ctrl+Z`. The last 100 undo steps are available during the session; switching documents clears them. Document changes, including undo, follow the usual autosave behavior.

The interface switches to a compact layout on narrow or short terminals, including when a Termux keyboard reduces the visible terminal height. Suggestions and edit history support scrolling with `↑`/`↓`, `PgUp`/`PgDn`, or the mouse wheel.

## Optional Live Model Check

The normal test suite uses local fixtures. To check directed editing against a real endpoint with synthetic rename, paragraph deletion, and repeated-text examples:

```sh
GOAUTHORLLM_TEST_BASE_URL=https://your-endpoint/v1 \
GOAUTHORLLM_TEST_MODEL=your-model \
go test ./internal/app -run TestLiveDirectedEditing -v -count=1
```
