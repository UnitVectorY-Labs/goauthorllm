---
layout: default
title: Usage
nav_order: 3
---

# Usage

## Running the Application

Start with no arguments to open the file browser (or jump straight to Config with `Alt+3`):

```bash
goauthorllm
```

Open a markdown document directly into the Write studio:

```bash
goauthorllm draft.md
```

## Unified Studio Interface

`goauthorllm` uses a single orange-accented studio workspace with four top-level tabs — all navigable by keyboard (`Alt+1`..`Alt+4`) or mouse click:

1. **Write (`Alt+1`)** — Markdown document editor paired with inline generation controls:
   - **Scope pills**: `Sentence`, `Paragraph`, `Section`, `Free` (`Alt+S` or click)
   - **Rounds selector**: `×1` to `×20` (`Alt+[` / `Alt+]` or click `−` / `+`). Multi-round runs automatically generate a structured outline plan first so each round produces distinct, non-repetitive content.
   - **Stream Speed**: `Max`, `35 c/s`, `65 c/s`, `110 c/s`, `180 c/s`, or any custom value in `.goauthorllm` (`Alt+-` / `Alt+=` or click). Adjust speed live while streaming!
   - **Exact Mid-Stream Stop**: Press **`Esc`** (or click **Stop**) at any point during paced streaming to cancel immediately and keep the exact text visible on screen (buffered, unrevealed text is discarded).
   - **Guidance Prompt (`Tab`)**: Always-accessible optional guidance input (automatically expands on small terminals), with per-file prompt history recall (`Alt+↑`/`Alt+↓`).
   - **+1 Round (`Ctrl+R`)**: Immediately run one more round with the current scope.
2. **Edit (`Alt+2`)** — Fast copy editing and custom directed editing:
   - Leave **Edit Instructions** blank to run copy editing using your `.goauthorllm` `copy_edit_rules`, or type custom instructions inline for targeted edits.
   - Choose **Review Edits (`Ctrl+G`)** to inspect line-numbered, context-aware word-level diffs one by one (`y` accept, `n` skip, `a` approve all remaining, `u` undo), or **Approve All (`Alt+A`)** to apply every validated edit automatically.
   - Switch between **Diff**, **Log**, and **Document** sub-tabs at any time.
3. **Config (`Alt+3`)** — Edit, validate, save, and hot-reload `.goauthorllm` right inside the app, including live **Fetch Models (`Alt+F`)** from `<base_url>/models` and a **Raw YAML (`Alt+Y`)** toggle.
4. **Files (`Alt+4` / `Ctrl+O`)** — Browse markdown files in the current directory or create a new document by name.

## Keyboard Reference

| Key | Action |
|-----|--------|
| `Alt+1`..`Alt+4` | Switch Write / Edit / Config / Files |
| `Tab` | Toggle focus between Document & Guidance Prompt (Write) or Instructions & Diff (Edit) |
| `Ctrl+G` | Generate content (Write) or start Edit review (Edit) |
| `Alt+S` | Cycle generation scope (`Sentence` / `Paragraph` / `Section` / `Free`) |
| `Alt+[` / `Alt+]` | Decrease / Increase generation rounds (`×1`..`×20`) |
| `Alt+-` / `Alt+=` | Adjust live streaming reading speed |
| `Ctrl+R` | Run `+1 Round` (Write) or Re-scan (Edit) |
| `Alt+A` | Approve All edits (Edit) |
| `y` / `n` / `a` / `u` | Accept / Skip / Approve Remaining / Undo (when focused on Diff) |
| `Ctrl+S` | Save document (or save & apply `.goauthorllm` in Config) |
| `Ctrl+Z` / `Alt+K` | Undo last change / Delete from cursor to end of document |
| `Alt+↑` / `Alt+↓` | Recall saved prompt/instruction history for the current document |
| `Alt+M` | Open Document Instructions (YAML Front Matter) |
| `F1` or `Alt+/` | Open interactive Help overlay |
| `Alt+E` | Open full Error Details overlay |
| `Esc` | Stop active stream/request immediately or close overlay |
| `Ctrl+Q` | Save & quit |

## Non-Interactive Mode

Use `--non-interactive` for scripted runs:

```bash
# Generate 3 new sections
goauthorllm --non-interactive --mode generate --submode section --rounds 3 draft.md

# Generate 1 paragraph with guidance
goauthorllm --non-interactive --mode generate --submode paragraph \
  --guidance "Summarize key tradeoffs" draft.md

# Run copy edits with approve-all
goauthorllm --non-interactive --mode edit --submode copy \
  --approval approve-all --max-edits 10 draft.md

# Run directed edits with custom instructions
goauthorllm --non-interactive --mode edit --submode directed \
  --approval approve-all --edit-instructions "Replace passive voice" draft.md
```

Exit codes: `0` (changed and saved), `1` (error), `2` (invalid usage/config), `3` (completed with no changes).
