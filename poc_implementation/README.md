# goauthorllm

goauthorllm is a terminal document authoring tool built with Bubble Tea. It focuses on two workflows for the same markdown file:

- `Generate` mode continues the current section or drafts the next one
- `Edit` mode proposes one exact-match copy edit at a time

The app keeps the document, prompt context, and model interaction inside a single TUI. It supports autosave, front matter metadata, local prompt overrides from `.goauthorllm`, and OpenAI-compatible chat endpoints.

## Quick Start

From a local checkout:

```bash
go build .
./goauthorllm --file draft.md
```

If you do not pass a file, the app starts in a chooser view for markdown files in the current directory.

Runtime settings can come from `.env`, environment variables, or flags. The included `.env.example` matches the built-in local defaults.

## Documentation

- [Overview](docs/README.md)
- [Install](docs/INSTALL.md)
- [Usage](docs/USAGE.md)
- [Configuration](docs/CONFIG.md)
- [Examples](docs/EXAMPLES.md)
