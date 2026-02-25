package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yash-srivastava19/grove/internal/ai"
	"github.com/yash-srivastava19/grove/internal/config"
	"github.com/yash-srivastava19/grove/internal/notes"
	"github.com/yash-srivastava19/grove/internal/ui"
)

const version = "0.1.0"

const usage = `grove — your knowledge garden in the terminal

Usage:
  grove                    open TUI
  grove new <title>        create note, open in $EDITOR
  grove today              open today's daily note in $EDITOR
  grove add <text>         append quick thought to today's note
  grove list               list all notes
  grove version

TUI keys:
  j/k  navigate    Enter open    n new    t today
  /    search      d delete      e edit   A ask AI
  ?    help        q quit
`

func main() {
	cfg, err := config.Load()
	if err != nil {
		die("config error: %v", err)
	}

	store := notes.NewStore(cfg.NotesDir)

	// First run: create welcome note if vault is empty
	ensureWelcome(store)

	args := os.Args[1:]

	if len(args) == 0 {
		runTUI(cfg, store)
		return
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("grove " + version)

	case "help", "--help", "-h":
		fmt.Print(usage)

	case "new", "n":
		title := strings.Join(args[1:], " ")
		if title == "" {
			die("usage: grove new <title>")
		}
		note, err := store.Create(title, nil)
		if err != nil {
			die("create: %v", err)
		}
		launchEditor(cfg.Editor, note.Filename)

	case "today", "t":
		note, err := store.CreateDaily()
		if err != nil {
			die("daily: %v", err)
		}
		launchEditor(cfg.Editor, note.Filename)

	case "add", "a":
		// Quick append to today's note — zero friction thought capture
		text := strings.Join(args[1:], " ")
		if text == "" {
			die("usage: grove add <text>")
		}
		note, err := store.CreateDaily()
		if err != nil {
			die("daily: %v", err)
		}
		timestamp := time.Now().Format("15:04")
		line := fmt.Sprintf("\n- %s %s", timestamp, text)
		f, err := os.OpenFile(note.Filename, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			die("append: %v", err)
		}
		_, _ = fmt.Fprintln(f, line)
		f.Close()
		fmt.Printf("added to %s\n", note.ID)

	case "list", "ls":
		all, err := store.LoadAll()
		if err != nil {
			die("list: %v", err)
		}
		for _, n := range all {
			fmt.Printf("%-40s  %s\n", n.ID, n.Title)
		}

	default:
		fmt.Fprintf(os.Stderr, "grove: unknown command %q\n\n", args[0])
		fmt.Print(usage)
		os.Exit(1)
	}
}

func runTUI(cfg *config.Config, store *notes.Store) {
	aiClient := ai.NewClient(cfg.GeminiKey, cfg.GeminiModel)
	app := ui.New(cfg, store, aiClient)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		die("%v", err)
	}
}

func ensureWelcome(store *notes.Store) {
	all, err := store.LoadAll()
	if err != nil || len(all) > 0 {
		return
	}
	// First run — create a welcome note so the vault isn't empty
	note, err := store.Create("Welcome to grove", []string{"grove"})
	if err != nil {
		return
	}
	note.Body = `## Welcome to grove 🌿

Your knowledge garden in the terminal. Notes are plain markdown files — yours forever.

### Quick start

| Key | Action |
|-----|--------|
| **n** | new note |
| **t** | today's daily note |
| **/** | fuzzy search |
| **e** | edit in $EDITOR |
| **A** | ask AI about this note |
| **?** | full help |

### From the command line

` + "```sh" + `
grove today          # open today's daily note
grove add "idea"     # append a quick thought to today's note
grove new "title"    # create and open a note
grove list           # list all notes
` + "```" + `

### Tips

- Use **daily notes** (` + "`t`" + `) as your inbox. Dump everything there, clean up later.
- Use **tags** in frontmatter: ` + "`tags: [work, ideas]`" + ` — searchable from ` + "`/`" + `.
- **AI** (` + "`A`" + `) uses your Gemini key from pairy config — ask anything about a note.
- Notes live in ` + "`~/.local/share/grove/notes/`" + ` — plain ` + "`.md`" + ` files, no lock-in.

Happy gardening.
`
	_ = store.Save(note)
}

func launchEditor(editor, path string) {
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		die("editor: %v", err)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "grove: "+format+"\n", args...)
	os.Exit(1)
}
