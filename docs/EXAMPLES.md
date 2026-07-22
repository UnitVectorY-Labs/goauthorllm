---
layout: default
title: Examples
nav_order: 6
---

# Examples

## Start a New Draft

Create a new markdown file and begin generating content:

```bash
goauthorllm --file draft.md
```

Choose **Generate** mode, then use **Continue** to start writing or type guidance in the prompt pane first.

## Continue an Existing Document

Open a document that already has content:

```bash
goauthorllm docs/design-notes.md
```

The model reads the existing sections and continues from where the document ends. This works well for architecture notes, RFCs, and long-form documentation.

## Use a Local System Message

If you want a consistent tone for all documents in a folder, create a `.goauthorllm` file:

```yaml
generate_prompt:
  append: |
    Keep the writing concise and avoid speculative language.
continue_prompt:
  append: |
    Favor concrete transitions over broad summary.
```

Then run goauthorllm in that directory. Every document opened in that folder uses the same prompt adjustments.

## Copy-Edit a Document

Open a file and choose **Edit** mode. The application sends the document to the model and displays one suggested change at a time. Accept the ones you agree with and skip the rest.

This works well for:

- Tightening prose in a report
- Fixing terminology in a technical guide
- Catching grammar and style issues

## Script Copy Editing Until Complete

Exit code `3` means the editor completed a pass without applying an edit. Limiting each invocation to one edit makes that status useful as a loop condition:

```bash
#!/usr/bin/env bash

while true; do
  if goauthorllm --non-interactive --mode edit --submode copy \
      --approval approve-all --max-edits 1 draft.md; then
    continue
  else
    status=$?
    if [ "$status" -eq 3 ]; then
      echo "Copy editing complete" >&2
      break
    fi
    exit "$status"
  fi
done
```

Each successful iteration prints and saves one old/new replacement. Configuration or runtime failures retain their distinct exit codes instead of being mistaken for completion.

## Run Without a Config File

Every connection and operation value can be supplied through flags or environment variables. Only `--non-interactive` and the document path must be explicit command-line arguments:

```bash
export GOAUTHORLLM_BASE_URL="https://api.openai.com/v1"
export GOAUTHORLLM_MODEL="gpt-4o"
export GOAUTHORLLM_API_KEY="sk-..."
export GOAUTHORLLM_MODE="generate"
export GOAUTHORLLM_SUBMODE="new-section"
export GOAUTHORLLM_GUIDANCE_FILE="$PWD/prompts/next-section.txt"

goauthorllm --non-interactive draft.md
```

For a directed edit, set `GOAUTHORLLM_MODE=edit`, `GOAUTHORLLM_SUBMODE=directed`, `GOAUTHORLLM_APPROVAL=llm-approved`, and provide `GOAUTHORLLM_EDIT_INSTRUCTIONS` or `GOAUTHORLLM_EDIT_INSTRUCTIONS_FILE`.

## Use with OpenAI

Set your API key and point to the OpenAI endpoint:

```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_BASE_URL="https://api.openai.com/v1"
export OPENAI_MODEL="gpt-4o"
goauthorllm
```

## Use with Ollama

Ollama runs locally and is the default endpoint:

```bash
ollama pull gemma3:4b
goauthorllm
```

No API key is needed for local Ollama.

## Use with OpenRouter

```bash
export GOAUTHORLLM_API_KEY="sk-or-..."
export GOAUTHORLLM_BASE_URL="https://openrouter.ai/api/v1"
export GOAUTHORLLM_MODEL="google/gemma-3-4b-it"
goauthorllm
```
