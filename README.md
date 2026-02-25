# grove

[![CI](https://github.com/yash-srivastava19/grove/actions/workflows/ci.yml/badge.svg)](https://github.com/yash-srivastava19/grove/actions/workflows/ci.yml)

Your knowledge garden in the terminal.

```
grove — 3 notes
──────────────────────────────────────────────────────
▸ Daily 2026-02-25                             just now
  Ideas Backlog                                    1d
  Meeting Notes                                    1w
──────────────────────────────────────────────────────
j/k  Enter  n new  t daily  / search  ? help  q quit
```

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/yash-srivastava19/grove/main/install.sh | bash
```

Or manually:

```sh
git clone https://github.com/yash-srivastava19/grove
cd grove && go build -o grove . && sudo mv grove /usr/local/bin/
```

## Use it

```sh
grove              # open your vault (TUI)
grove today        # open today's daily note in $EDITOR
grove add "idea"   # append a quick thought to today's note (no TUI needed)
grove new "title"  # create a note and open it
grove list         # list all notes
```

## Keys (inside TUI)

| Key | Action |
|-----|--------|
| `j` / `k` | navigate |
| `Enter` | open note |
| `n` | new note |
| `t` | today's daily note |
| `/` | fuzzy search (title + tags + body) |
| `e` | edit in `$EDITOR` (nvim, vim…) |
| `A` | ask AI about this note |
| `d` | delete |
| `gg` / `G` | top / bottom |
| `?` | help |
| `q` | quit / back |

## Suggested workflows

**Daily driver** — open grove each morning, hit `t` to start your daily note. Use `grove add "quick thought"` from anywhere in your shell to capture without opening the TUI.

**Project notes** — `grove new "project-x kickoff"`, add tags `[work, project-x]`. Search with `/project-x` to find everything.

**AI-powered review** — open any note, hit `A`, ask Gemini to summarize, critique, or ask probing questions. Good for thinking out loud.

## AI setup

If you use [pairy](https://github.com/yash-srivastava19/pairy), you're already set — grove reads `~/.config/pairy/config.json`.

Otherwise, set `GEMINI_API_KEY` or create `~/.config/grove/config.json`:

```json
{ "api_key": "YOUR_GEMINI_API_KEY" }
```

Get a free key at [aistudio.google.com](https://aistudio.google.com).

## Notes format

Plain markdown with frontmatter — your files, forever:

```markdown
---
title: My Note
tags: [work, ideas]
created: 2026-02-25T10:00:00Z
updated: 2026-02-25T10:30:00Z
---

Your content here.
```

Default location: `~/.local/share/grove/notes/`
