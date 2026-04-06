---
layout: default
title: Overview
nav_order: 1
---

# goauthorllm

goauthorllm is a terminal user interface (TUI) application for working with documents using LLM assistance. It is built around two core workflows:

- **Generate** mode for drafting and extending documents with streaming LLM output
- **Edit** mode for copy-editing a document one structured suggestion at a time

## What It Does

goauthorllm opens a markdown file in a full-screen terminal workspace and connects to an OpenAI-compatible LLM endpoint. The application handles the interaction with the model so you can focus on writing.

In **Generate** mode the model continues the active section or writes the next one. The output streams directly into the document editor in real time.

In **Edit** mode the model reviews the entire document and proposes one exact-match replacement at a time. You accept, skip, or refresh each suggestion before moving on.

## Design Approach

The interface is designed around a single-document workflow. You choose a file, pick a mode, and work in a full-screen workspace with keyboard shortcuts and mouse support. The key decisions behind the design:

- **Streaming output** keeps the experience interactive instead of waiting for a complete response
- **Structured suggestions** in edit mode ensure that every proposed change can be validated before applying it
- **Embedded prompts** ship with the binary so the application works without extra configuration files
- **Local overrides** through the `.goauthorllm` file let you customize prompts per folder without modifying source
- **Autosave** and manual save keep the document on disk throughout the session
- **Progressive Esc navigation** lets you back out of modals, workspaces, and eventually exit the application

## Supported Endpoints

goauthorllm works with any endpoint that implements the OpenAI chat completions API. This includes:

- [Ollama](https://ollama.com) running locally
- [OpenAI API](https://platform.openai.com)
- [OpenRouter](https://openrouter.ai)
- Any other OpenAI-compatible service

## Next Steps

- [Installation](installation) for building and installing the binary
- [Usage](usage) for flags, environment variables, and workflow details
- [Configuration](configuration) for the `.goauthorllm` override file
- [Prompts](prompts) for details on each embedded prompt
- [Examples](examples) for common workflows
