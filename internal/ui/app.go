package ui

import (
	"fmt"
	"os/exec"
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
)

type appState int

const (
	stateList appState = iota
	stateViewer
	stateSearch
	stateNewNote
	stateAIPanel
	stateConfirmDelete
	stateHelp
)

// Messages

type notesLoadedMsg struct {
	notes []*notes.Note
	err   error
}

type editorClosedMsg struct {
	notes    []*notes.Note
	openID   string
	err      error
}

type aiResponseMsg struct {
	response string
	err      error
}

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
	searchInput  textinput.Model
	newNoteInput textinput.Model
	aiInput      textinput.Model

	// Search
	searchQuery string

	// AI
	aiResponse string
	aiLoading  bool
	aiError    string

	// Delete
	deleteTarget *notes.Note

	// Vim g-prefix tracking
	lastKey string

	// Status
	statusMsg     string
	statusIsError bool
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

	vp := viewport.New(80, 20)

	return &App{
		cfg:          cfg,
		store:        store,
		ai:           aiClient,
		searchInput:  si,
		newNoteInput: ni,
		aiInput:      aip,
		viewport:     vp,
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

func (a *App) cmdOpenEditor(note *notes.Note) tea.Cmd {
	noteID := note.ID
	return tea.ExecProcess(exec.Command(a.cfg.Editor, note.Filename), func(err error) tea.Msg {
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
		// Always reload notes even on editor error (file may have been saved)
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
		} else {
			a.aiResponse = msg.response
		}

	case tea.KeyMsg:
		// Clear status on any key
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
		case stateAIPanel:
			return a.updateAIPanel(msg)
		case stateConfirmDelete:
			return a.updateConfirmDelete(msg)
		case stateHelp:
			return a.updateHelp(msg)
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

	case "?":
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
		a.aiResponse = ""
		a.aiError = ""
		a.aiLoading = false
		return a, textinput.Blink

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

	case "?":
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

// ── AI Panel ──────────────────────────────────────────────────────────────────

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
		a.aiResponse = ""
		a.aiError = ""
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
		a.state = stateList
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
	case stateAIPanel:
		return a.viewAIPanel()
	case stateConfirmDelete:
		return a.viewConfirmDelete()
	case stateHelp:
		return a.viewHelp()
	}
	return ""
}

func (a *App) viewList() string {
	var b strings.Builder
	w := a.width

	// Header
	count := fmt.Sprintf("%d notes", len(a.allNotes))
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  —  ") + styleSubtitle.Render(count) + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	listH := a.height - 5
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

	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	// Status / hints
	if a.statusMsg != "" {
		sty := styleSuccess
		if a.statusIsError {
			sty = styleError
		}
		b.WriteString(sty.Render("  " + a.statusMsg))
	} else {
		b.WriteString(styleHint.Render("  j/k  Enter  n new  t daily  / search  d delete  r refresh  ? help  q quit"))
	}

	return b.String()
}

func (a *App) viewViewer() string {
	if a.current == nil {
		return "no note"
	}
	var b strings.Builder
	w := a.width

	editHint := styleDimItem.Render("[e]edit  [A]AI  [q]back")
	title := styleTitle.Render(truncate(a.current.Title, w-26))
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
		b.WriteString(styleHint.Render(fmt.Sprintf("  j/k scroll  gg/G top/bot  d/u half-page  e edit  A ask AI  q back  %d%%", pct)))
	}
	return b.String()
}

func (a *App) viewSearch() string {
	var b strings.Builder
	w := a.width

	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  /  ") + styleSubtitle.Render("search") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")
	b.WriteString(styleInputActive.Width(w - 4).Render(a.searchInput.View()) + "\n")

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

	// Response area
	innerH := a.height - 10
	if innerH < 3 {
		innerH = 3
	}

	var content string
	switch {
	case a.aiLoading:
		content = styleSubtitle.Render("  ⟳  thinking...")
	case a.aiError != "":
		content = styleError.Render("  ✗  " + a.aiError)
	case a.aiResponse != "":
		r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(w-10))
		var rendered string
		if err == nil {
			rendered, err = r.Render(a.aiResponse)
		}
		if err != nil {
			rendered = a.aiResponse
		}
		lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
		if len(lines) > innerH {
			lines = lines[:innerH]
		}
		content = strings.Join(lines, "\n")
	default:
		content = styleSubtitle.Render("  Ask anything about this note...")
	}

	panel := stylePanelBorder.Width(w - 6).Height(innerH).Render(content)
	b.WriteString(panel + "\n\n")

	inputSty := styleInputActive
	if a.aiLoading {
		inputSty = styleInputBorder
	}
	b.WriteString(inputSty.Width(w-4).Render(a.aiInput.View()) + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", w)) + "\n")

	if a.aiLoading {
		b.WriteString(styleHint.Render("  waiting...  Esc cancel"))
	} else {
		b.WriteString(styleHint.Render("  Enter submit  Esc back"))
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
		"    gg / G        top / bottom",
		"    Enter / l     open note",
		"    n             new note",
		"    t             today's daily note",
		"    /             fuzzy search",
		"    d             delete (with confirm)",
		"    r             refresh",
		"    q             quit",
		"",
		styleDivider.Render("  VIEWER"),
		"    j/k           scroll",
		"    gg / G         top / bottom",
		"    d/u           half-page down/up",
		"    e             open in $EDITOR",
		"    A             ask AI about note",
		"    q / h / Esc   back to list",
		"",
		styleDivider.Render("  SEARCH"),
		"    type          filter",
		"    Enter         open",
		"    ctrl+n/p      navigate results",
		"    Esc           cancel",
		"",
		styleDivider.Render("  AI PANEL"),
		"    type          your question",
		"    Enter         send to Gemini",
		"    Esc           back",
	)

	var b strings.Builder
	b.WriteString(styleTitle.Render("grove") + styleDivider.Render("  —  ") + styleSubtitle.Render("help") + "\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n\n")
	b.WriteString(help + "\n\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", a.width)) + "\n")
	b.WriteString(styleHint.Render("  q / Esc / ? to close"))
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
	a.state = stateViewer
	a.reRender()
}

func (a *App) reRender() {
	if a.current == nil {
		return
	}

	content := a.current.Body
	if len(a.current.Tags) > 0 {
		tagLine := styleTag.Render("tags: #" + strings.Join(a.current.Tags, " #"))
		content = tagLine + "\n\n" + content
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(a.viewport.Width-2),
	)
	if err != nil {
		a.viewport.SetContent(content)
		return
	}
	rendered, err := r.Render(content)
	if err != nil {
		a.viewport.SetContent(content)
		return
	}
	a.viewport.SetContent(rendered)
	a.viewport.GotoTop()
}

func (a *App) ensureVisible() {
	listH := a.height - 5
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

