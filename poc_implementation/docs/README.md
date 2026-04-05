# goauthorllm

goauthorllm is a TUI for working on documents from the terminal. The product is framed around two core workflows:

- `Generate` mode for drafting or extending a document with LLM assistance
- `Edit` mode for copy-editing a selected document with one suggestion at a time

## What It Is For

- Drafting technical notes, articles, reports, and long-form documents
- Extending an existing markdown document with model assistance
- Copy-editing documents one fix at a time with structured model output
- Keeping the workflow inside a single terminal app instead of bouncing between tools

## Current Shape

- Bubble Tea TUI with file selection, mode selection, and a shared document workspace
- `Generate` mode with streaming markdown continuation and next-section actions
- `Edit` mode with structured-output copy-edit suggestions and exact-match validation
- Front matter support for document metadata and prompt context
- OpenAI-compatible LLM client
- Autosave, manual save, and progressive `Esc` back-navigation
- Embedded default prompts with local override support from `.goauthorllm`

For setup and command-line details, see [Usage](USAGE.md) and [Configuration](CONFIG.md).
