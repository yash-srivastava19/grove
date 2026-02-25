package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yash-srivastava19/grove/internal/ai"
	"github.com/yash-srivastava19/grove/internal/config"
	"github.com/yash-srivastava19/grove/internal/notes"
	"github.com/yash-srivastava19/grove/internal/ui"
)

const version = "0.1.0"

const usage = `grove — your knowledge garden in the terminal

Usage:
  grove                    open grove TUI
  grove new <title>        create note and open in $EDITOR
  grove today              open today's daily note
  grove list               list all notes (non-interactive)
  grove version            print version

Keybindings (TUI):
  j/k          navigate / scroll
  Enter        open note
  n            new note
  t            today's daily note
  /            fuzzy search
  d            delete note
  e            edit in $EDITOR
  A            ask AI (Gemini)
  ?            help
  q            quit
`

func main() {
	cfg, err := config.Load()
	if err != nil {
		die("config error: %v", err)
	}

	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			fmt.Println("grove " + version)
			return

		case "help", "--help", "-h":
			fmt.Print(usage)
			return

		case "new", "n":
			title := strings.Join(args[1:], " ")
			if title == "" {
				die("usage: grove new <title>")
			}
			store := notes.NewStore(cfg.NotesDir)
			note, err := store.Create(title, nil)
			if err != nil {
				die("create: %v", err)
			}
			fmt.Fprintf(os.Stderr, "created: %s\n", note.Filename)
			launchEditor(cfg.Editor, note.Filename)
			return

		case "today", "t":
			store := notes.NewStore(cfg.NotesDir)
			note, err := store.CreateDaily()
			if err != nil {
				die("create daily: %v", err)
			}
			fmt.Fprintf(os.Stderr, "opening: %s\n", note.Filename)
			launchEditor(cfg.Editor, note.Filename)
			return

		case "list", "ls":
			store := notes.NewStore(cfg.NotesDir)
			all, err := store.LoadAll()
			if err != nil {
				die("list: %v", err)
			}
			for _, n := range all {
				fmt.Printf("%-40s  %s\n", n.ID, n.Title)
			}
			return

		default:
			fmt.Fprintf(os.Stderr, "grove: unknown command %q\n\n", args[0])
			fmt.Print(usage)
			os.Exit(1)
		}
	}

	// TUI mode
	store := notes.NewStore(cfg.NotesDir)
	aiClient := ai.NewClient(cfg.GeminiKey, cfg.GeminiModel)
	app := ui.New(cfg, store, aiClient)

	p := tea.NewProgram(app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		die("%v", err)
	}
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
