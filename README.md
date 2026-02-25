# grove

[![CI](https://github.com/yash-srivastava19/grove/actions/workflows/ci.yml/badge.svg)](https://github.com/yash-srivastava19/grove/actions/workflows/ci.yml)

Your knowledge garden in the terminal. Obsidian-style note-taking, fully in the CLI.

```
grove — 3 notes
──────────────────────────────────────────────────────
▸ Daily 2026-02-25                             just now
  Ideas Backlog                                    1d
  Meeting Notes                                    1w
──────────────────────────────────────────────────────
j/k  Enter  n new  t daily  / search  d delete  ? help  q quit
```

## Features

- **Vim keybindings** throughout — `j/k`, `gg`/`G`, `/` search, `d` delete
- **Markdown rendering** in the viewer (via Glamour)
- **Fuzzy search** across titles, tags, and body
- **Daily notes** — `t` or `grove today` to open today's note
- **AI assistant** — `A` in any note to ask Gemini about it (reuses pairy config)
- **$EDITOR integration** — `e` opens the note in your editor (nvim, vim, etc.)
- **YAML frontmatter** — title, tags, created/updated timestamps

## Install

```sh
git clone https://github.com/yash-srivastava19/grove
cd grove
go build -o grove .
sudo mv grove /usr/local/bin/
```

## Usage

```sh
grove                   # open TUI
grove new "my note"     # create and open note in $EDITOR
grove today             # open today's daily note
grove list              # list notes (non-interactive)
grove version
```

## Keybindings

### List view
| Key | Action |
|-----|--------|
| `j` / `k` | navigate |
| `gg` / `G` | top / bottom |
| `Enter` / `l` | open note |
| `n` | new note |
| `t` | today's daily note |
| `/` | fuzzy search |
| `d` | delete (with confirm) |
| `r` | refresh |
| `q` | quit |

### Note viewer
| Key | Action |
|-----|--------|
| `j` / `k` | scroll |
| `gg` / `G` | top / bottom |
| `d` / `u` | half-page down / up |
| `e` | open in `$EDITOR` |
| `A` | ask AI about this note |
| `q` / `h` / `Esc` | back |

### Search
| Key | Action |
|-----|--------|
| type | filter |
| `Enter` | open top result |
| `ctrl+n` / `ctrl+p` | navigate results |
| `Esc` | cancel |

## AI (Gemini)

grove reuses your [pairy](https://github.com/yash-srivastava19/pairy) config at `~/.config/pairy/config.json`.
No extra setup needed if you already use pairy.

Otherwise, set `GEMINI_API_KEY` or create `~/.config/grove/config.json`:

```json
{
  "api_key": "YOUR_GEMINI_API_KEY",
  "model": "gemini-2.5-flash"
}
```

## Note format

Notes are plain markdown files with YAML frontmatter:

```markdown
---
title: My Note
tags: [work, ideas]
created: 2026-02-25T10:00:00Z
updated: 2026-02-25T10:30:00Z
---

Your note content here.
```

Notes live in `~/.local/share/grove/notes/` by default.

## Configuration

`~/.config/grove/config.json`:

```json
{
  "notes_dir": "~/.local/share/grove/notes",
  "editor": "nvim",
  "ai_enabled": true,
  "api_key": "YOUR_GEMINI_API_KEY",
  "model": "gemini-2.5-flash"
}
```
