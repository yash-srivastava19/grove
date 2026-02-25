package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"github.com/yash-srivastava19/grove/internal/ai"
	"github.com/yash-srivastava19/grove/internal/config"
	"github.com/yash-srivastava19/grove/internal/notes"
	"github.com/yash-srivastava19/grove/internal/templates"
)

type appState int

const (
	stateList appState = iota
	stateViewer
	stateSearch
	stateNewNote
	stateTemplatePicker // N key: pick template
	stateTemplateTitle  // after template picked: enter title
	stateAIPanel
	stateConfirmDelete
	stateHelp
	stateLinks    // L key: wiki-links panel
	stateVaultAI  // @ key: vault-wide AI
)

// ── Messages ──────────────────────────────────────────────────────────────────

type notesLoadedMsg struct {
	notes []*notes.Note
	err   error
}

type editorClosedMsg struct {
	notes  []*notes.Note
	openID string
	err    error
}

type aiResponseMsg struct {
	response string
	err      error
}

type vaultAIResponseMsg struct {
	response string
	err      error
}

// ── App struct ────────────────────────────────────────────────────────────────

// App is the main Bubble Tea model.
type App struct {
	cfg   *config.Config
	store *notes.Store
	ai    *ai.Client

	state  appState
	width  int
	height int

	// List
	allNotes   []*notes.Note
	filtered   []*notes.Note
	cursor     int
	listOffset int

	// Viewer
	current  *notes.Note
	viewport viewport.Model

	// Inputs
	searchInput     textinput.Model
	newNoteInput    textinput.Model
	aiInput         textinput.Model
	vaultAIInput    textinput.Model
	templateTitleIn textinput.Model

	// Search
	searchQuery string

	// AI (per-note)
	aiHistory []aiEntry
	aiLoading bool
	aiError   string

	// Vault AI
	vaultAIHistory []aiEntry
	vaultAILoading bool
	vaultAIError   string

	// Delete
	deleteTarget *notes.Note

	// Vim g-prefix tracking
	lastKey string

	// Help: remember which state to return to
	prevState appState

	// Status
	statusMsg     string
	statusIsError bool

	// Template picker
	templateCursor   int
	selectedTemplate string

	// Links panel
	linksCursor  int
	linksOut     []string      // outgoing link targets
	linksBack    []*notes.Note // backlinks

	// Rendered lines for paragraph navigation
	renderedLines []string
}

type aiEntry struct {
	question string
	answer   string
}

func New(cfg *config.Config, store *notes.Store, aiClient *ai.Client) *App {
	si := textinput.New()
	si.Placeholder = "search notes..."
	si.CharLimit = 200

	ni := textinput.New()
	ni.Placeholder = "note title..."
	ni.CharLimit = 200

	aip := textinput.New()
	aip.Placeholder = "ask about this note..."
	aip.CharLimit = 500

	vaip := textinput.New()
	vaip.Placeholder = "ask about your entire vault..."
	vaip.CharLimit = 500

	tti := textinput.New()
	tti.Placeholder = "note title..."
	tti.CharLimit = 200

	vp := viewport.New(80, 20)

	return &App{
		cfg:             cfg,
		store:           store,
		ai:              aiClient,
		searchInput:     si,
		newNoteInput:    ni,
		aiInput:         aip,
		vaultAIInput:    vaip,
		templateTitleIn: tti,
		viewport:        vp,
	}
}

func (a *App) Init() tea.Cmd {
	return a.cmdLoadNotes()
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (a *App) cmdLoadNotes() tea.Cmd {
	return func() tea.Msg {
		ns, err := a.store.LoadAll()
		return notesLoadedMsg{notes: ns, err: err}
	}
}

func editorCmd(editor, path string) *exec.Cmd {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...)
}

func (a *App) cmdOpenEditor(note *notes.Note) tea.Cmd {
	noteID := note.ID
	return tea.ExecProcess(editorCmd(a.cfg.Editor, note.Filename), func(err error) tea.Msg {
		ns, loadErr := a.store.LoadAll()
		if loadErr != nil {
			return editorClosedMsg{err: loadErr}
		}
		return editorClosedMsg{notes: ns, openID: noteID, err: err}
	})
}

func (a *App) cmdAskAI(note *notes.Note, question string) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.ai.Ask(note.Title, note.Body, question)
		return aiResponseMsg{response: resp, err: err}
	}
}

func (a *App) cmdAskVault(question string) tea.Cmd {
	all := a.allNotes
	return func() tea.Msg {
		ctx := make([]ai.NoteContext, len(all))
		for i, n := range all {
			ctx[i] = ai.NoteContext{Title: n.Title, Tags: n.Tags, Body: n.Body}
		}
		resp, err := a.ai.AskVault(ctx, question)
		return vaultAIResponseMsg{response: resp, err: err}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.viewport.Width = a.width - 2
		a.viewport.Height = a.height - 6
		if a.current != nil {
			a.reRender()
		}

	case notesLoadedMsg:
		if msg.err != nil {
			a.setStatus("error loading notes: "+msg.err.Error(), true)
			return a, nil
		}
		a.allNotes = msg.notes
		a.filtered = msg.notes
		if a.cursor >= len(a.filtered) {
			a.cursor = max(0, len(a.filtered)-1)
		}

	case editorClosedMsg:
		if msg.err != nil {
			a.setStatus("editor: "+msg.err.Error(), true)
		}
		if msg.notes != nil {
			a.allNotes = msg.notes
			a.filtered = msg.notes
		}
		if msg.openID != "" {
			for _, n := range msg.notes {
				if n.ID == msg.openID {
					a.current = n
					a.state = stateViewer
					a.reRender()
					return a, nil
				}
			}
		}
		a.lastKey = ""
		a.state = stateList

	case aiResponseMsg:
		a.aiLoading = false
		if msg.err != nil {
			a.aiError = msg.err.Error()
		} else if len(a.aiHistory) > 0 {
			a.aiHistory[len(a.aiHistory)-1].answer = msg.response
		}

	case vaultAIResponseMsg:
		a.vaultAILoading = false
		if msg.err != nil {
			a.vaultAIError = msg.err.Error()
		} else if len(a.vaultAIHistory) > 0 {
			a.vaultAIHistory[len(a.vaultAIHistory)-1].answer = msg.response
		}

	case tea.KeyMsg:
		a.statusMsg = ""

		switch a.state {
		case stateList:
			return a.updateList(msg)
		case stateViewer:
			return a.updateViewer(msg)
		case stateSearch:
			return a.updateSearch(msg)
		case stateNewNote:
			return a.updateNewNote(msg)
		case stateTemplatePicker:
			return a.updateTemplatePicker(msg)
		case stateTemplateTitle:
			return a.updateTemplateTitle(msg)
		case stateAIPanel:
			return a.updateAIPanel(msg)
		case stateConfirmDelete:
			return a.updateConfirmDelete(msg)
		case stateHelp:
			return a.updateHelp(msg)
		case stateLinks:
			return a.updateLinks(msg)
		case stateVaultAI:
			return a.updateVaultAI(msg)
		}
	}

	return a, nil
}

// ── List ──────────────────────────────────────────────────────────────────────

func (a *App) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prev := a.lastKey
	a.lastKey = msg.String()

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "j", "down":
		if a.cursor < len(a.filtered)-1 {
			a.cursor++
			a.ensureVisible()
		}

	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
			a.ensureVisible()
		}

	case "g":
		if prev == "g" {
			a.cursor = 0
			a.listOffset = 0
			a.lastKey = ""
		}

	case "G":
		if len(a.filtered) > 0 {
			a.cursor = len(a.filtered) - 1
			a.ensureVisible()
		}

	case "enter", "l":
		if len(a.filtered) > 0 {
			a.openNote(a.filtered[a.cursor])
		}

	case "n":
		a.state = stateNewNote
		a.newNoteInput.SetValue("")
		a.newNoteInput.Focus()
		return a, textinput.Blink

	case "N":
		// New note with template picker
		a.state = stateTemplatePicker
		a.templateCursor = 0

	case "t":
		note, err := a.store.CreateDaily()
		if err != nil {
			a.setStatus("error: "+err.Error(), true)
			return a, nil
		}
		return a, a.cmdOpenEditor(note)

	case "/":
		a.state = stateSearch
		a.searchInput.SetValue("")
		a.searchInput.Focus()
		a.searchQuery = ""
		a.filtered = a.allNotes
		a.cursor = 0
		a.lastKey = ""
		return a, textinput.Blink

	case "d":
		if len(a.filtered) > 0 {
			a.deleteTarget = a.filtered[a.cursor]
			a.state = stateConfirmDelete
		}

	case "r":
		return a, a.cmdLoadNotes()

	case "@":
		if !a.ai.Available() {
			a.setStatus("no Gemini API key — check ~/.config/pairy/config.json", true)
			return a, nil
		}
		a.state = stateVaultAI
		a.vaultAIInput.SetValue("")
		a.vaultAIInput.Focus()
		a.vaultAIError = ""
		a.vaultAILoading = false
		return a, textinput.Blink

	case "?":
		a.prevState = stateList
		a.state = stateHelp
	}

	return a, nil
}

// ── Viewer ────────────────────────────────────────────────────────────────────

func (a *App) updateViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prev := a.lastKey
	a.lastKey = msg.String()

	switch msg.String() {
	case "q", "h", "esc":
		a.state = stateList

	case "e":
		if a.current != nil {
			return a, a.cmdOpenEditor(a.current)
		}

	case "A", "a":
		if !a.ai.Available() {
			a.setStatus("no Gemini API key — check ~/.config/pairy/config.json", true)
			return a, nil
		}
		a.state = stateAIPanel
		a.aiInput.SetValue("")
		a.aiInput.Focus()
		a.aiError = ""
		a.aiLoading = false
		return a, textinput.Blink

	case "L":
		if a.current != nil {
			a.openLinksPanel()
		}

	case "g":
		if prev == "g" {
			a.viewport.GotoTop()
			a.lastKey = ""
		}

	case "G":
		a.viewport.GotoBottom()

	case "j", "down":
		a.viewport.ScrollDown(1)

	case "k", "up":
		a.viewport.ScrollUp(1)

	case "d", "ctrl+d":
		a.viewport.ScrollDown(a.viewport.Height / 2)

	case "u", "ctrl+u":
		a.viewport.ScrollUp(a.viewport.Height / 2)

	case "ctrl+f", "pgdown":
		a.viewport.ScrollDown(a.viewport.Height)

	case "ctrl+b", "pgup":
		a.viewport.ScrollUp(a.viewport.Height)

	case "}":
		a.jumpParagraph(1)

	case "{":
		a.jumpParagraph(-1)

	case "?":
		a.prevState = stateViewer
		a.state = stateHelp
	}

	return a, nil
}

// ── Search ────────────────────────────────────────────────────────────────────

func (a *App) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.state = stateList
		a.filtered = a.allNotes
		a.cursor = 0
		a.searchInput.Blur()
		return a, nil

	case "enter":
		if len(a.filtered) > 0 {
			a.openNote(a.filtered[a.cursor])
		}
		return a, nil

	case "ctrl+n", "down":
		if a.cursor < len(a.filtered)-1 {
			a.cursor++
		}
		return a, nil

	case "ctrl+p", "up":
		if a.cursor > 0 {
			a.cursor--
		}
		return a, nil
	}

	var cmd tea.Cmd
	a.searchInput, cmd = a.searchInput.Update(msg)
	q := a.searchInput.Value()
	if q != a.searchQuery {
		a.searchQuery = q
		a.cursor = 0
		a.runSearch(q)
	}
	return a, cmd
}

func (a *App) runSearch(query string) {
	if query == "" {
		a.filtered = a.allNotes
		return
	}
	targets := make([]string, len(a.allNotes))
	for i, n := range a.allNotes {
		targets[i] = n.Title + " " + strings.Join(n.Tags, " ") + " " + n.Body
	}
	matches := fuzzy.Find(query, targets)
	result := make([]*notes.Note, 0, len(matches))
	for _, m := range matches {
		result = append(result, a.allNotes[m.Index])
	}
	a.filtered = result
}

// ── New Note ──────────────────────────────────────────────────────────────────

func (a *App) updateNewNote(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.state = stateList
		a.newNoteInput.Blur()
		return a, nil

	case "enter":
		title := strings.TrimSpace(a.newNoteInput.Value())
		a.newNoteInput.Blur()
		if title == "" {
			a.state = stateList
			return a, nil
		}
		note, err := a.store.Create(title, nil)
		if err != nil {
			a.setStatus("error: "+err.Error(), true)
			a.state = stateList
			return a, nil
		}
		return a, a.cmdOpenEditor(note)
	}

	var cmd tea.Cmd
	a.newNoteInput, cmd = a.newNoteInput.Update(msg)
	return a, cmd
}

// ── Template Picker ───────────────────────────────────────────────────────────

func (a *App) updateTemplatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		a.state = stateList
		return a, nil

	case "j", "down":
		if a.templateCursor < len(templates.Names)-1 {
			a.templateCursor++
		}

	case "k", "up":
		if a.templateCursor > 0 {
			a.templateCursor--
		}

	case "enter", "l":
		a.selectedTemplate = templates.Names[a.templateCursor]
		a.state = stateTemplateTitle
		a.templateTitleIn.SetValue("")
		a.templateTitleIn.Focus()
		return a, textinput.Blink
	}

	return a, nil
}

func (a *App) updateTemplateTitle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.state = stateTemplatePicker
		a.templateTitleIn.Blur()
		return a, nil

	case "enter":
		title := strings.TrimSpace(a.templateTitleIn.Value())
		a.templateTitleIn.Blur()
		if title == "" {
			a.state = stateList
			return a, nil
		}
		note, err := a.store.Create(title, nil)
		if err != nil {
			a.setStatus("error: "+err.Error(), true)
			a.state = stateList
			return a, nil
		}
		date := time.Now().Format("2006-01-02")
		note.Body = templates.Get(a.selectedTemplate, title, date)
		if err := a.store.Save(note); err != nil {
			a.setStatus("save error: "+err.Error(), true)
			a.state = stateList
			return a, nil
		}
		return a, a.cmdOpenEditor(note)
	}

	var cmd tea.Cmd
	a.templateTitleIn, cmd = a.templateTitleIn.Update(msg)
	return a, cmd
}

// ── AI Panel (per-note) ───────────────────────────────────────────────────────

func (a *App) updateAIPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if !a.aiLoading {
			a.state = stateViewer
			a.aiInput.Blur()
		}
		return a, nil

	case "enter":
		q := strings.TrimSpace(a.aiInput.Value())
		if q == "" || a.aiLoading {
			return a, nil
		}
		a.aiLoading = true
		a.aiError = ""
		a.aiHistory = append(a.aiHistory, aiEntry{question: q})
		a.aiInput.SetValue("")
		return a, a.cmdAskAI(a.current, q)
	}

	if !a.aiLoading {
		var cmd tea.Cmd
		a.aiInput, cmd = a.aiInput.Update(msg)
		return a, cmd
	}
	return a, nil
}

// ── Confirm Delete ────────────────────────────────────────────────────────────

func (a *App) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		if a.deleteTarget != nil {
			title := a.deleteTarget.Title
			if err := a.store.Delete(a.deleteTarget.ID); err != nil {
				a.setStatus("delete failed: "+err.Error(), true)
			} else {
				a.setStatus("deleted "+title, false)
			}
			a.deleteTarget = nil
		}
		a.state = stateList
		return a, a.cmdLoadNotes()

	case "n", "N", "esc", "q":
		a.deleteTarget = nil
		a.state = stateList
	}
	return a, nil
}

// ── Help ──────────────────────────────────────────────────────────────────────

func (a *App) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "?":
		a.state = a.prevState
	}
	return a, nil
}

// ── Links Panel ───────────────────────────────────────────────────────────────

func (a *App) openLinksPanel() {
	if a.current == nil {
		return
	}
	a.linksOut = notes.ExtractLinks(a.current.Body)
	a.linksBack = notes.Backlinks(a.current.Title, a.allNotes)
	a.linksCursor = 0
	a.state = stateLinks
}

func (a *App) updateLinks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalEntries := len(a.linksOut) + len(a.linksBack)

	switch msg.String() {
	case "esc", "q", "h":
		a.state = stateViewer
		return a, nil

	case "j", "down":
		if a.linksCursor < totalEntries-1 {
			a.linksCursor++
		}

	case "k", "up":
		if a.linksCursor > 0 {
			a.linksCursor--
		}

	case "enter", "l":
		// Determine which note to open
		var targetTitle string
		if a.linksCursor < len(a.linksOut) {
			targetTitle = a.linksOut[a.linksCursor]
		} else {
			idx := a.linksCursor - len(a.linksOut)
			if idx < len(a.linksBack) {
				// open by ID
				a.openNote(a.linksBack[idx])
				return a, nil
			}
		}
		if targetTitle != "" {
			// find note by title
			for _, n := range a.allNotes {
				if strings.EqualFold(n.Title, targetTitle) {
					a.openNote(n)
					return a, nil
				}
			}
			a.setStatus("note not found: "+targetTitle, true)
			a.state = stateViewer
		}
	}

	return a, nil
}

// ── Vault AI ──────────────────────────────────────────────────────────────────

func (a *App) updateVaultAI(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if !a.vaultAILoading {
			a.state = stateList
			a.vaultAIInput.Blur()
		}
		return a, nil

	case "enter":
		q := strings.TrimSpace(a.vaultAIInput.Value())
		if q == "" || a.vaultAILoading {
			return a, nil
		}
		a.vaultAILoading = true
		a.vaultAIError = ""
		a.vaultAIHistory = append(a.vaultAIHistory, aiEntry{question: q})
		a.vaultAIInput.SetValue("")
		return a, a.cmdAskVault(q)
	}

	if !a.vaultAILoading {
		var cmd tea.Cmd
		a.vaultAIInput, cmd = a.vaultAIInput.Update(msg)
		return a, cmd
	}
	return a, nil
}

// ── Views ─────────────────────────────────────────────────────────────────────

func (a *App) View() string {
	if a.width == 0 {
		return "loading..."
	}
	switch a.state {
	case stateList:
		return a.viewList()
	case stateViewer:
		return a.viewViewer()
	case stateSearch:
		return a.viewSearch()
	case stateNewNote:
		return a.viewNewNote()
	case stateTemplatePicker:
		return a.viewTemplatePicker()
	case stateTemplateTitle:
		return a.viewTemplateTitle()
	case stateAIPanel:
		return a.viewAIPanel()
	case stateConfirmDelete:
		return a.viewConfirmDelete()
	case stateHelp:
		return a.viewHelp()
	case stateLinks:
		return a.viewLinks()
	case stateVaultAI:
		return a.viewVaultAI()
	}
	return ""
}

func (a *App) viewList() string {
	var b strings.Builder
	w := a.width

	count := fmt.Sprintf("%d notes", len(a.allNotes))
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  —  ") + styleSubtitle.Render(count) + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	// Reserve 1 extra line for note preview
	listH := a.height - 6
	if listH < 1 {
		listH = 1
	}

	if len(a.filtered) == 0 {
		empty := styleSubtitle.Render("\n  no notes — press n to create one, t for today's daily note")
		b.WriteString(empty + "\n")
	} else {
		end := min(a.listOffset+listH, len(a.filtered))
		for i := a.listOffset; i < end; i++ {
			n := a.filtered[i]
			age := humanTime(n.Updated)
			maxTitle := w - len(age) - 6
			if maxTitle < 10 {
				maxTitle = 10
			}
			title := truncate(n.Title, maxTitle)
			pad := w - 4 - len([]rune(title)) - len(age)
			if pad < 1 {
				pad = 1
			}
			spacer := strings.Repeat(" ", pad)

			if i == a.cursor {
				b.WriteString("  " + styleSelectedItem.Render("▸ "+title) + spacer + styleDimItem.Render(age) + "\n")
			} else {
				b.WriteString("    " + styleNormalItem.Render(title) + spacer + styleDimItem.Render(age) + "\n")
			}
		}
	}

	// Pad to fill height
	shown := min(listH, len(a.filtered))
	for i := shown; i < listH; i++ {
		b.WriteString("\n")
	}

	// Note preview: first non-empty non-heading line of highlighted note
	preview := ""
	if len(a.filtered) > 0 && a.cursor < len(a.filtered) {
		preview = notePreview(a.filtered[a.cursor].Body, w-4)
	}
	if preview != "" {
		b.WriteString(styleDimItem.Render("  " + preview) + "\n")
	} else {
		b.WriteString("\n")
	}

	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	if a.statusMsg != "" {
		sty := styleSuccess
		if a.statusIsError {
			sty = styleError
		}
		b.WriteString(sty.Render("  " + a.statusMsg))
	} else {
		b.WriteString(styleHint.Render("  j/k · Enter · n/N new · t daily · / search · d del · @ AI · ? help · q"))
	}

	return b.String()
}

func (a *App) viewViewer() string {
	if a.current == nil {
		return "no note"
	}
	var b strings.Builder
	w := a.width

	editHint := styleDimItem.Render("[e]edit  [A]AI  [L]links  [q]back")
	title := styleTitle.Render(truncate(a.current.Title, w-36))
	b.WriteString("  " + title + "  " + editHint + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")
	b.WriteString(a.viewport.View() + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	if a.statusMsg != "" {
		sty := styleSuccess
		if a.statusIsError {
			sty = styleError
		}
		b.WriteString(sty.Render("  " + a.statusMsg))
	} else {
		pct := int(a.viewport.ScrollPercent() * 100)
		wc := wordCount(a.current.Body)
		pos := ""
		for i, n := range a.filtered {
			if a.current != nil && n.ID == a.current.ID {
				pos = fmt.Sprintf(" (%d/%d)", i+1, len(a.filtered))
				break
			}
		}
		b.WriteString(styleHint.Render(fmt.Sprintf("  j/k  gg/G  {/}  d/u  e edit  A AI  L links  q back%s  %d words  %d%%", pos, wc, pct)))
	}
	return b.String()
}

func (a *App) viewSearch() string {
	var b strings.Builder
	w := a.width

	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  /  ") + styleSubtitle.Render("search") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")
	b.WriteString(styleInputActive.Width(w-4).Render(a.searchInput.View()) + "\n")

	listH := a.height - 7
	if listH < 1 {
		listH = 1
	}

	if len(a.filtered) == 0 {
		msg := "  no results"
		if a.searchQuery == "" {
			msg = "  no notes yet — press Esc, then n to create one"
		}
		b.WriteString(styleSubtitle.Render("\n"+msg) + "\n")
	} else {
		end := min(listH, len(a.filtered))
		for i := 0; i < end; i++ {
			n := a.filtered[i]
			age := humanTime(n.Updated)
			title := truncate(n.Title, w-len(age)-6)
			pad := w - 4 - len([]rune(title)) - len(age)
			if pad < 1 {
				pad = 1
			}
			if i == a.cursor {
				b.WriteString("  " + styleSelectedItem.Render("▸ "+title) + strings.Repeat(" ", pad) + styleDimItem.Render(age) + "\n")
			} else {
				b.WriteString("    " + styleNormalItem.Render(title) + strings.Repeat(" ", pad) + styleDimItem.Render(age) + "\n")
			}
		}
	}

	shown := min(listH, max(len(a.filtered), 1))
	for i := shown; i < listH; i++ {
		b.WriteString("\n")
	}

	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")
	b.WriteString(styleHint.Render("  type to search  Enter open  ctrl+n/p navigate  Esc cancel"))
	return b.String()
}

func (a *App) viewNewNote() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  +  ") + styleSubtitle.Render("new note") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n\n")
	b.WriteString(styleHint.Render("  Note title:") + "\n")
	b.WriteString(styleInputActive.Width(a.width-4).Render(a.newNoteInput.View()) + "\n\n")
	b.WriteString(styleHint.Render("  Enter to create and open in $EDITOR  ·  Esc to cancel"))
	return b.String()
}

func (a *App) viewTemplatePicker() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  +  ") + styleSubtitle.Render("new note — choose template") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n\n")
	for i, name := range templates.Names {
		if i == a.templateCursor {
			b.WriteString("  " + styleSelectedItem.Render("▸ "+name) + "\n")
		} else {
			b.WriteString("    " + styleNormalItem.Render(name) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n")
	b.WriteString(styleHint.Render("  j/k navigate  Enter select  Esc cancel"))
	return b.String()
}

func (a *App) viewTemplateTitle() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  +  ") + styleSubtitle.Render("new note — "+a.selectedTemplate+" template") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n\n")
	b.WriteString(styleHint.Render("  Note title:") + "\n")
	b.WriteString(styleInputActive.Width(a.width-4).Render(a.templateTitleIn.View()) + "\n\n")
	b.WriteString(styleHint.Render("  Enter to create and open in $EDITOR  ·  Esc to go back"))
	return b.String()
}

func (a *App) viewAIPanel() string {
	if a.current == nil {
		return a.viewViewer()
	}
	var b strings.Builder
	w := a.width

	aiLabel := styleAILabel.Render("[ AI ]")
	title := styleTitle.Render(truncate(a.current.Title, w-10))
	b.WriteString("  " + title + "  " + aiLabel + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	innerH := a.height - 10
	if innerH < 3 {
		innerH = 3
	}

	var lines []string
	r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(w-10))

	if len(a.aiHistory) == 0 && !a.aiLoading {
		lines = []string{styleSubtitle.Render("  Ask anything about this note...")}
	} else {
		for _, entry := range a.aiHistory {
			lines = append(lines, styleAILabel.Render("  Q: ")+styleNormalItem.Render(entry.question))
			if entry.answer != "" {
				rendered := entry.answer
				if r != nil {
					if out, err := r.Render(entry.answer); err == nil {
						rendered = strings.TrimRight(out, "\n")
					}
				}
				for _, l := range strings.Split(rendered, "\n") {
					lines = append(lines, l)
				}
			}
			lines = append(lines, "")
		}
		if a.aiLoading {
			lines = append(lines, styleSubtitle.Render("  thinking..."))
		}
		if a.aiError != "" {
			lines = append(lines, styleError.Render("  error: "+a.aiError))
		}
	}

	if len(lines) > innerH {
		lines = lines[len(lines)-innerH:]
	}
	content := strings.Join(lines, "\n")

	panel := stylePanelBorder.Width(w - 6).Height(innerH).Render(content)
	b.WriteString(panel + "\n\n")

	inputSty := styleInputActive
	if a.aiLoading {
		inputSty = styleInputBorder
	}
	b.WriteString(inputSty.Width(w-4).Render(a.aiInput.View()) + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	if a.aiLoading {
		b.WriteString(styleHint.Render("  waiting for Gemini..."))
	} else {
		b.WriteString(styleHint.Render("  Enter submit  Esc back to note"))
	}
	return b.String()
}

func (a *App) viewConfirmDelete() string {
	if a.deleteTarget == nil {
		return a.viewList()
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("grove") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n\n")
	b.WriteString(styleConfirm.Render(fmt.Sprintf("  Delete \"%s\"?", a.deleteTarget.Title)) + "\n\n")
	b.WriteString(styleNormalItem.Render("  y") + styleHint.Render(" yes   ") + styleNormalItem.Render("n / Esc") + styleHint.Render(" cancel") + "\n")
	return b.String()
}

func (a *App) viewHelp() string {
	help := lipgloss.JoinVertical(lipgloss.Left,
		styleDivider.Render("  LIST"),
		"    j/k          navigate",
		"    gg / G       top / bottom",
		"    Enter / l    open note",
		"    n            new note",
		"    N            new note with template",
		"    t            today's daily note",
		"    /            fuzzy search",
		"    d            delete (with confirm)",
		"    @            vault-wide AI",
		"    r            refresh",
		"    q            quit",
		"",
		styleDivider.Render("  VIEWER"),
		"    j/k          scroll",
		"    gg / G       top / bottom",
		"    { / }        prev / next paragraph",
		"    d/u          half-page down/up",
		"    e            open in $EDITOR",
		"    A            ask AI about note",
		"    L            links panel (wiki-links)",
		"    q / h / Esc  back to list",
		"",
		styleDivider.Render("  SEARCH"),
		"    type         filter",
		"    Enter        open",
		"    ctrl+n/p     navigate results",
		"    Esc          cancel",
		"",
		styleDivider.Render("  AI PANEL"),
		"    type         your question",
		"    Enter        send to Gemini",
		"    Esc          back",
		"",
		styleDivider.Render("  LINKS PANEL"),
		"    j/k          navigate",
		"    Enter        open linked note",
		"    Esc / q      back to viewer",
		"",
		styleDivider.Render("  VAULT AI  (@)"),
		"    type         your question",
		"    Enter        send to Gemini",
		"    Esc          back to list",
	)

	var b strings.Builder
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  —  ") + styleSubtitle.Render("help") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n\n")
	b.WriteString(help + "\n\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n")
	b.WriteString(styleHint.Render("  q / Esc / ? to close"))
	return b.String()
}

func (a *App) viewLinks() string {
	if a.current == nil {
		return a.viewViewer()
	}
	var b strings.Builder
	w := a.width

	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  —  ") + styleSubtitle.Render("links: "+a.current.Title) + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n\n")

	idx := 0

	// Outgoing links
	b.WriteString(styleAILabel.Render("  → outgoing links") + "\n")
	if len(a.linksOut) == 0 {
		b.WriteString(styleDimItem.Render("    (none)") + "\n")
	} else {
		for _, target := range a.linksOut {
			// Check if note exists
			found := false
			for _, n := range a.allNotes {
				if strings.EqualFold(n.Title, target) {
					found = true
					break
				}
			}
			label := "[[" + target + "]]"
			if !found {
				label = label + " (not found)"
			}
			if idx == a.linksCursor {
				b.WriteString("  " + styleSelectedItem.Render("▸ "+label) + "\n")
			} else if !found {
				b.WriteString("    " + styleDimItem.Render(label) + "\n")
			} else {
				b.WriteString("    " + styleNormalItem.Render(label) + "\n")
			}
			idx++
		}
	}

	b.WriteString("\n")

	// Backlinks
	b.WriteString(styleAILabel.Render("  ← backlinks") + "\n")
	if len(a.linksBack) == 0 {
		b.WriteString(styleDimItem.Render("    (none)") + "\n")
	} else {
		for _, n := range a.linksBack {
			label := n.Title
			if idx == a.linksCursor {
				b.WriteString("  " + styleSelectedItem.Render("▸ "+label) + "\n")
			} else {
				b.WriteString("    " + styleNormalItem.Render(label) + "\n")
			}
			idx++
		}
	}

	b.WriteString("\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")
	b.WriteString(styleHint.Render("  j/k navigate  Enter open  Esc back to viewer"))
	return b.String()
}

func (a *App) viewVaultAI() string {
	var b strings.Builder
	w := a.width

	vaultLabel := styleAILabel.Render("[ vault AI ]")
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  —  ") + vaultLabel + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	innerH := a.height - 10
	if innerH < 3 {
		innerH = 3
	}

	var lines []string
	r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(w-10))

	if len(a.vaultAIHistory) == 0 && !a.vaultAILoading {
		lines = []string{styleSubtitle.Render("  Ask anything about your vault...")}
	} else {
		for _, entry := range a.vaultAIHistory {
			lines = append(lines, styleAILabel.Render("  Q: ")+styleNormalItem.Render(entry.question))
			if entry.answer != "" {
				rendered := entry.answer
				if r != nil {
					if out, err := r.Render(entry.answer); err == nil {
						rendered = strings.TrimRight(out, "\n")
					}
				}
				for _, l := range strings.Split(rendered, "\n") {
					lines = append(lines, l)
				}
			}
			lines = append(lines, "")
		}
		if a.vaultAILoading {
			lines = append(lines, styleSubtitle.Render("  thinking..."))
		}
		if a.vaultAIError != "" {
			lines = append(lines, styleError.Render("  error: "+a.vaultAIError))
		}
	}

	if len(lines) > innerH {
		lines = lines[len(lines)-innerH:]
	}
	content := strings.Join(lines, "\n")

	panel := stylePanelBorder.Width(w - 6).Height(innerH).Render(content)
	b.WriteString(panel + "\n\n")

	inputSty := styleInputActive
	if a.vaultAILoading {
		inputSty = styleInputBorder
	}
	b.WriteString(inputSty.Width(w-4).Render(a.vaultAIInput.View()) + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	if a.vaultAILoading {
		b.WriteString(styleHint.Render(fmt.Sprintf("  waiting for Gemini...  (%d notes in context)", len(a.allNotes))))
	} else {
		b.WriteString(styleHint.Render(fmt.Sprintf("  Enter submit  Esc back  (%d notes)", len(a.allNotes))))
	}
	return b.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (a *App) openNote(note *notes.Note) {
	loaded, err := a.store.Load(note.ID)
	if err != nil {
		a.setStatus("error: "+err.Error(), true)
		return
	}
	a.current = loaded
	a.aiHistory = nil
	a.state = stateViewer
	a.reRender()
}

func (a *App) reRender() {
	if a.current == nil {
		return
	}

	// Preprocess wiki-links: replace [[target]] with `[[target]]` so glamour
	// renders them as inline code — visually distinct without breaking layout.
	body := preprocessLinks(a.current.Body)

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(a.viewport.Width-2),
	)
	rendered := body
	if err == nil {
		if out, err2 := r.Render(body); err2 == nil {
			rendered = out
		}
	}

	// Prepend tag line after glamour render so ANSI codes stay clean.
	if len(a.current.Tags) > 0 {
		tagLine := styleTag.Render("tags: #" + strings.Join(a.current.Tags, " #"))
		rendered = tagLine + "\n\n" + rendered
	}

	a.renderedLines = strings.Split(rendered, "\n")
	a.viewport.SetContent(rendered)
	a.viewport.GotoTop()
}

func (a *App) ensureVisible() {
	listH := a.height - 6
	if listH < 1 {
		listH = 1
	}
	if a.cursor < a.listOffset {
		a.listOffset = a.cursor
	}
	if a.cursor >= a.listOffset+listH {
		a.listOffset = a.cursor - listH + 1
	}
}

func (a *App) jumpParagraph(direction int) {
	lines := a.renderedLines
	if len(lines) == 0 {
		return
	}
	cur := a.viewport.YOffset
	i := cur

	if direction > 0 {
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			i++
		}
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
	} else {
		i--
		for i > 0 && strings.TrimSpace(lines[i]) == "" {
			i--
		}
		for i > 0 && strings.TrimSpace(lines[i-1]) != "" {
			i--
		}
	}
	a.viewport.SetYOffset(i)
}

func (a *App) setStatus(msg string, isErr bool) {
	a.statusMsg = msg
	a.statusIsError = isErr
}

func humanTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		return t.Format("Jan 2")
	}
}

func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// notePreview returns the first non-empty, non-heading line of a note body,
// truncated to maxW characters.
func notePreview(body string, maxW int) string {
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		return truncate(l, maxW)
	}
	return ""
}

// wordCount counts words in a string.
func wordCount(s string) int {
	return len(strings.Fields(s))
}

// preprocessLinks replaces [[target]] with `[[target]]` for glamour rendering.
var wikiLinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

func preprocessLinks(body string) string {
	return wikiLinkRe.ReplaceAllString(body, "`[[$1]]`")
}

