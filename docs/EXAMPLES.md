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
