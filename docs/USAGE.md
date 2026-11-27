---
layout: default
title: Usage
nav_order: 3
---

# Usage

## Running the Application

Start with no arguments to open the file browser (or press `^T` / `^P` to open Config or the Command Menu):

```bash
goauthorllm
```

Open a markdown document directly into the Write studio:

```bash
goauthorllm draft.md
```

## Unified Studio Interface

`goauthorllm` uses an orange-accented studio workspace designed for **macOS, Windows, Linux, and Termux**, with universal `Ctrl` (`^`) shortcuts visible directly in the interface plus mouse/tap support:

1. **Write** — Markdown document editor paired with live generation controls:
   - **Length pills (`^L`)**: `Sentence`, `Paragraph`, `Section`, `Free`
   - **Rounds selector (`^N`)**: `×1` to `×20`. Multi-round runs automatically create a structured outline plan first so each round produces distinct, non-repetitive content.
   - **Controls Bar focus (`Tab`)**: Cycles `Document → Guidance Prompt → Controls Bar`. When the Controls Bar is focused, tap `1`–`4` to pick Length directly, `-`/`+` to fine-tune Rounds, `Enter` to Generate, or `e`/`c`/`f`/`m`/`?` to jump to Edit, Config, Files, Meta, or Help.
   - **Exact Mid-Stream Stop (`Esc`)**: Cancels streaming immediately and keeps the exact text visible on screen (buffered, unrevealed text is discarded). Stream pacing (`stream_chars_per_sec`) is configured universally in `.goauthorllm` via the **Config** tab.
   - **Guidance Prompt (`Tab`)**: Optional guidance input (auto-expands on small/Termux screens), with per-file prompt history recall.
   - **+1 Round (`^R`)**: Immediately run one more round with the current length.
2. **Edit** — Copy editing and custom directed editing:
   - Leave **Edit Instructions** blank to run copy editing using your `.goauthorllm` `copy_edit_rules`, or type custom instructions inline for targeted edits.
   - Press **`^G`** to review line-numbered, context-aware word-level diffs one by one (`y` accept, `n` skip, `a` approve all remaining, `u` undo), or **`^X`** to approve all validated edits automatically.
   - Switch between **Diff**, **Log**, and **Document** sub-tabs with `←`/`→` or mouse click.
3. **Config** — Edit, validate, save, and hot-reload `.goauthorllm` right inside the app, including **`^F` Fetch Models** from `<base_url>/models`, universal `stream_chars_per_sec` pacing, and **`^Y` Raw YAML** toggle.
4. **Files (`^O`)** — Browse markdown files in the current directory or create a new document by name.

## Universal Keyboard Reference

Every action works across **macOS, Windows, Linux, and Termux** without relying on `Alt`/`Option` or function keys:

| Key | Action |
|-----|--------|
| `^P` (`Ctrl+P`) | Open **Command Menu** (jump to any view, length, rounds, Meta, Error details, or Help) |
| `^T` (`Ctrl+T`) | Cycle views: `Write → Edit → Config → Files` |
| `^O` (`Ctrl+O`) | Jump directly to Files |
| `^S` (`Ctrl+S`) | Save document (or save & apply `.goauthorllm` in Config) |
| `^Q` (`Ctrl+Q`) | Save & quit |
| `Esc` | Stop active stream/request immediately, close menu/overlay, or return to editor |
| `Tab` | Cycle `Document → Guidance Prompt → Controls Bar` (Write), or `Instructions ↔ Diff` (Edit), or fields (Config) |
| `^G` (`Ctrl+G`) | Generate content (Write) or start Edit review (Edit) |
| `^L` (`Ctrl+L`) | Cycle generation length: `Sentence → Paragraph → Section → Free` (Write) |
| `^N` (`Ctrl+N`) | Cycle generation rounds: `×1 → ×2 → ×3 → ×5` (Write) |
| `^R` (`Ctrl+R`) | Run `+1 Round` (Write), Re-scan (Edit), or Refresh (Files) |
| `^X` (`Ctrl+X`) | Approve All edits (Edit) |
| `y` / `n` / `a` / `u` | Accept / Skip / Approve Remaining / Undo (when in Diff view) |
| `^F` / `^Y` | Fetch Models / Toggle Form vs Raw YAML (Config) |
| `^Z` / `^K` | Undo last change / Delete from cursor to end of document (Write Editor) |
| `^J` | Insert newline inside Guidance Prompt or Edit Instructions |

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
