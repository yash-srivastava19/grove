package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const geminiURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Available() bool {
	return c.apiKey != ""
}

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Role  string        `json:"role,omitempty"`
	Parts []geminiPart  `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// NoteContext holds the data sent to AskVault.
type NoteContext struct {
	Title string
	Tags  []string
	Body  string
}

// AskVault sends all notes as context and answers a vault-wide question.
// If there are more than 20 notes, each body is truncated to 500 chars.
func (c *Client) AskVault(notesCtx []NoteContext, question string) (string, error) {
	if !c.Available() {
		return "", fmt.Errorf("no Gemini API key configured (check ~/.config/pairy/config.json or set GEMINI_API_KEY)")
	}

	const maxNotes = 20
	const truncateAt = 500

	var sb strings.Builder
	for i, n := range notesCtx {
		body := n.Body
		if len(notesCtx) > maxNotes && len(body) > truncateAt {
			body = body[:truncateAt] + "..."
		}
		tags := ""
		if len(n.Tags) > 0 {
			tags = " [" + strings.Join(n.Tags, ", ") + "]"
		}
		sb.WriteString(fmt.Sprintf("--- Note %d: %s%s ---\n%s\n\n", i+1, n.Title, tags, body))
	}

	prompt := fmt.Sprintf(
		"You are a personal knowledge assistant. Answer based on the user's notes vault. Be specific and cite which note titles you're drawing from.\n\nNOTES:\n%s\nQUESTION: %s",
		sb.String(),
		question,
	)

	req := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: prompt}}},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(geminiURL, c.model, c.apiKey)
	resp, err := c.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result geminiResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse error: %w\nraw: %s", err, string(data))
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	var parts []string
	for _, p := range result.Candidates[0].Content.Parts {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, ""), nil
}

func (c *Client) Ask(noteTitle, noteContent, question string) (string, error) {
	if !c.Available() {
		return "", fmt.Errorf("no Gemini API key configured (check ~/.config/pairy/config.json or set GEMINI_API_KEY)")
	}

	system := `You are a helpful assistant embedded in grove, a terminal note-taking app.
You help the user think through their notes, ask clarifying questions, and surface unstated assumptions.
Be concise. Push back when reasoning has gaps. Ask one probing question when useful.`

	contextBlock := fmt.Sprintf("Note: %s\n\n%s", noteTitle, noteContent)
	if len(contextBlock) > 4000 {
		contextBlock = contextBlock[:4000] + "\n... (truncated)"
	}

	userPrompt := fmt.Sprintf("Context from my note:\n\n%s\n\nQuestion: %s", contextBlock, question)

	req := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: system}},
		},
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: userPrompt}}},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(geminiURL, c.model, c.apiKey)
	resp, err := c.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result geminiResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse error: %w\nraw: %s", err, string(data))
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	var parts []string
	for _, p := range result.Candidates[0].Content.Parts {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, ""), nil
}
