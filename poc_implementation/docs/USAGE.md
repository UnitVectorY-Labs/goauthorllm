# Usage

## Running The App

Load configuration from `.env`, environment variables, or command-line flags, then start the TUI:

```bash
go run . --file draft.md
```

If no file is provided, the app starts in a chooser view that lists markdown files in the current working directory.
After you choose a file, the TUI asks whether you want to work in `Generate` mode or `Edit` mode.

You can also pass the file as a positional argument:

```bash
go run . draft.md
```

Build and run a binary:

```bash
go build .
./goauthorllm --file draft.md
```

## Flags And Environment Variables

Flags override environment values.

| Setting | Environment variable(s) | Flag | Default | Description |
| --- | --- | --- | --- | --- |
| File path | `GOAUTHORLLM_FILE` | `--file` | chooser view | Markdown file to open. Can also be passed as a positional argument. |
| Base URL | `GOAUTHORLLM_BASE_URL`, `OPENAI_BASE_URL` | `--base-url` | `http://localhost:11434/v1` | OpenAI-compatible endpoint URL. |
| Model | `GOAUTHORLLM_MODEL`, `OPENAI_MODEL` | `--model` | `gemma4:e4b` | Model name sent to the endpoint. |
| API key | `GOAUTHORLLM_API_KEY`, `OPENAI_API_KEY` | `--api-key` | empty | Optional bearer token for the endpoint. |
| Timeout | `GOAUTHORLLM_TIMEOUT` | `--timeout` | `90s` | Request timeout as a Go duration string. |

## Workflow

The application has two operating modes for the same selected document.

### Generate Mode

- Open or create a markdown file
- Choose `Generate`
- Use the editor and prompt box to shape the next generation request
- Continue the current section or start a new section
- Save manually or let autosave keep the file current

### Edit Mode

- Open or create a markdown file
- Choose `Edit`
- The app sends the full document body to the model with the copy-editing system prompt
- The model returns structured output containing `old_text` and `new_text`
- The UI verifies that `old_text` matches exactly one location before applying it
- `Accept` applies the replacement and asks for the next suggestion
- `Skip` leaves the document unchanged and asks for the next suggestion

## Controls

- `Tab` and `Shift+Tab` move focus
- `Ctrl+S` saves immediately
- `Ctrl+O` returns to file selection
- `Ctrl+Q` quits
- `Esc` backs out of the current view, and repeated `Esc` presses eventually exit the app
- `Esc` also cancels an in-flight LLM request

### Generate Mode Shortcuts

- `Ctrl+G` continues the current section
- `Ctrl+N` starts a new section

### Edit Mode Shortcuts

- `Ctrl+A` accepts the current suggestion
- `Ctrl+K` skips the current suggestion
- `Ctrl+R` requests a fresh suggestion
