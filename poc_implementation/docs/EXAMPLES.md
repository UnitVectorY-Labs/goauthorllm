# Examples

## Start A New Draft

Create a new markdown file in the current folder and start generating from it:

```bash
go run . --file draft.md
```

Use this when you want the model to extend a memo, article, proposal, or similar document.

## Continue A Technical Note

Open an existing markdown file and ask the model to keep going from the current section:

```bash
go run . docs/design-notes.md
```

This is useful for architecture notes, RFCs, and long-form documentation where you want to preserve the existing structure.

## Draft With Local Prompt Guidance

If you want a consistent tone for a folder, add a `.goauthorllm` file alongside the document:

```yaml
generate_prompt:
  append: |
    Keep the writing concise and avoid speculative language.
continue_prompt:
  append: |
    Favor concrete transitions over broad summary.
```

Then launch the app in that directory and work as usual.

## Copy-Edit A Document

Open the file, choose `Edit`, and let the app propose one exact replacement at a time. A good use case is tightening prose in a report or fixing terminology in a technical guide.

## Keep The Scope General

The repository is not meant to be story-specific. The same flow should work for:

- Technical documentation
- Articles and essays
- Policy or process docs
- Narrative drafts
