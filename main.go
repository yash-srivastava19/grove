package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yash-srivastava19/grove/internal/ai"
	"github.com/yash-srivastava19/grove/internal/config"
	"github.com/yash-srivastava19/grove/internal/notes"
	"github.com/yash-srivastava19/grove/internal/templates"
	"github.com/yash-srivastava19/grove/internal/ui"
)

const version = "0.1.0"

const usage = `grove — your knowledge garden in the terminal

Usage:
  grove                              open TUI
  grove new [--template T] <title>   create note, open in $EDITOR
  grove today                        open today's daily note in $EDITOR
  grove add <text>                   append quick thought to today's note
  grove search <query>               search notes (non-interactive)
  grove list                         list all notes
  grove ask <question>               ask AI about your entire vault
  grove stats                        show vault statistics
  grove version

Templates: default, meeting, brainstorm, research

TUI keys:
  j/k  navigate    Enter open    n new    N new with template    t today
  /    search      d delete      e edit   A ask AI               @ vault AI
  L    links       ?    help     q quit
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
		// Parse optional --template flag
		tmplName := "default"
		rest := args[1:]
		for i := 0; i < len(rest); i++ {
			if rest[i] == "--template" || rest[i] == "-t" {
				if i+1 >= len(rest) {
					die("--template requires a name (default, meeting, brainstorm, research)")
				}
				tmplName = rest[i+1]
				rest = append(rest[:i], rest[i+2:]...)
				break
			}
			if strings.HasPrefix(rest[i], "--template=") {
				tmplName = strings.TrimPrefix(rest[i], "--template=")
				rest = append(rest[:i], rest[i+1:]...)
				break
			}
		}
		title := strings.Join(rest, " ")
		if title == "" {
			die("usage: grove new [--template T] <title>")
		}
		note, err := store.Create(title, nil)
		if err != nil {
			die("create: %v", err)
		}
		date := time.Now().Format("2006-01-02")
		note.Body = templates.Get(tmplName, title, date)
		if err := store.Save(note); err != nil {
			die("save: %v", err)
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

	case "search", "s":
		query := strings.Join(args[1:], " ")
		if query == "" {
			die("usage: grove search <query>")
		}
		all, err := store.LoadAll()
		if err != nil {
			die("search: %v", err)
		}
		found := 0
		ql := strings.ToLower(query)
		for _, n := range all {
			haystack := strings.ToLower(n.Title + " " + strings.Join(n.Tags, " ") + " " + n.Body)
			if strings.Contains(haystack, ql) {
				fmt.Printf("%-40s  %s\n", n.ID, n.Title)
				found++
			}
		}
		if found == 0 {
			fmt.Fprintf(os.Stderr, "no notes match %q\n", query)
			os.Exit(1)
		}

	case "ask":
		question := strings.Join(args[1:], " ")
		if question == "" {
			die("usage: grove ask <question>")
		}
		if cfg.GeminiKey == "" {
			fmt.Fprintln(os.Stderr, "grove: no Gemini API key configured (check ~/.config/pairy/config.json or set GEMINI_API_KEY)")
			os.Exit(1)
		}
		all, err := store.LoadAll()
		if err != nil {
			die("load notes: %v", err)
		}
		aiClient := ai.NewClient(cfg.GeminiKey, cfg.GeminiModel)
		ctx := make([]ai.NoteContext, len(all))
		for i, n := range all {
			ctx[i] = ai.NoteContext{Title: n.Title, Tags: n.Tags, Body: n.Body}
		}
		answer, err := aiClient.AskVault(ctx, question)
		if err != nil {
			die("AI error: %v", err)
		}
		fmt.Println(answer)

	case "stats":
		all, err := store.LoadAll()
		if err != nil {
			die("load notes: %v", err)
		}
		if len(all) == 0 {
			fmt.Println("no notes yet")
			return
		}
		totalWords := 0
		tagCount := map[string]int{}
		oldest := all[0]
		newest := all[0]
		for _, n := range all {
			totalWords += len(strings.Fields(n.Body))
			for _, t := range n.Tags {
				tagCount[t]++
			}
			if n.Created.Before(oldest.Created) {
				oldest = n
			}
			if n.Created.After(newest.Created) {
				newest = n
			}
		}

		// Top 5 tags
		type tagFreq struct {
			tag   string
			count int
		}
		var tagList []tagFreq
		for t, c := range tagCount {
			tagList = append(tagList, tagFreq{t, c})
		}
		sort.Slice(tagList, func(i, j int) bool {
			return tagList[i].count > tagList[j].count
		})
		if len(tagList) > 5 {
			tagList = tagList[:5]
		}

		fmt.Printf("notes:       %d\n", len(all))
		fmt.Printf("words:       %d\n", totalWords)
		fmt.Printf("oldest note: %s\n", oldest.Title)
		fmt.Printf("newest note: %s\n", newest.Title)
		if len(tagList) > 0 {
			fmt.Print("top tags:    ")
			for i, tf := range tagList {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Printf("%s (%d)", tf.tag, tf.count)
			}
			fmt.Println()
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
	note.Body = `## Welcome to grove

Your knowledge garden in the terminal. Notes are plain markdown files — yours forever.

### Quick start

| Key | Action |
|-----|--------|
| **n** | new note |
| **N** | new note with template |
| **t** | today's daily note |
| **/** | fuzzy search |
| **e** | edit in $EDITOR |
| **A** | ask AI about this note |
| **@** | vault-wide AI |
| **L** | links panel |
| **?** | full help |

### From the command line

` + "```sh" + `
grove today                      # open today's daily note
grove add "idea"                 # append a quick thought to today's note
grove new "title"                # create and open a note
grove new --template meeting "Title"  # use a template
grove list                       # list all notes
grove ask "what did I write about auth?"  # AI search across vault
grove stats                      # show vault statistics
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
	// Support editor config with args, e.g. "code --wait"
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
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
