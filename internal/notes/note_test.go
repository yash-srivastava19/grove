package notes

import (
	"testing"
	"time"
)

func TestParseFrontmatter_full(t *testing.T) {
	content := "---\ntitle: Test Note\ntags: [work, ideas]\ncreated: 2024-01-01T00:00:00Z\nupdated: 2024-06-01T00:00:00Z\n---\n\nBody text here."
	meta, body := ParseFrontmatter(content)

	if meta["title"] != "Test Note" {
		t.Errorf("title: got %q", meta["title"])
	}
	if meta["tags"] != "[work, ideas]" {
		t.Errorf("tags: got %q", meta["tags"])
	}
	if body != "Body text here." {
		t.Errorf("body: got %q", body)
	}
}

func TestParseFrontmatter_noFrontmatter(t *testing.T) {
	content := "# Just a plain note\n\nNo frontmatter here."
	meta, body := ParseFrontmatter(content)

	if len(meta) != 0 {
		t.Errorf("expected empty meta, got %v", meta)
	}
	if body != content {
		t.Errorf("body should be full content when no frontmatter")
	}
}

func TestParseFrontmatter_emptyBody(t *testing.T) {
	content := "---\ntitle: Empty\ntags: []\n---\n"
	meta, body := ParseFrontmatter(content)

	if meta["title"] != "Empty" {
		t.Errorf("title: got %q", meta["title"])
	}
	if body != "" {
		t.Errorf("body should be empty, got %q", body)
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"[work, ideas]", []string{"work", "ideas"}},
		{"[]", nil},
		{"[single]", []string{"single"}},
		{"[tag1, tag2, tag3]", []string{"tag1", "tag2", "tag3"}},
	}

	for _, tt := range tests {
		got := parseTags(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("parseTags(%q): got %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("parseTags(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestNoteFromRaw(t *testing.T) {
	raw := "---\ntitle: My Note\ntags: [go, cli]\ncreated: 2024-01-01T00:00:00Z\nupdated: 2024-06-01T00:00:00Z\n---\n\n# Hello\n\nWorld."
	n := NoteFromRaw("my-note", "/path/my-note.md", raw, time.Now())

	if n.ID != "my-note" {
		t.Errorf("ID: got %q", n.ID)
	}
	if n.Title != "My Note" {
		t.Errorf("Title: got %q", n.Title)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "go" || n.Tags[1] != "cli" {
		t.Errorf("Tags: got %v", n.Tags)
	}
	if n.Body != "# Hello\n\nWorld." {
		t.Errorf("Body: got %q", n.Body)
	}
}

func TestNoteFromRaw_noFrontmatter(t *testing.T) {
	raw := "# Plain Note\n\nNo frontmatter."
	n := NoteFromRaw("plain-note", "/path/plain-note.md", raw, time.Now())

	if n.Title != "plain-note" {
		t.Errorf("Title should fall back to ID, got %q", n.Title)
	}
	if n.Body != raw {
		t.Errorf("Body should be full content")
	}
}

func TestBuildFrontmatter(t *testing.T) {
	n := &Note{
		Title:   "Test",
		Tags:    []string{"a", "b"},
		Created: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	fm := BuildFrontmatter(n)
	if fm == "" {
		t.Error("BuildFrontmatter returned empty string")
	}
	// Round-trip
	meta, _ := ParseFrontmatter(fm + "body")
	if meta["title"] != "Test" {
		t.Errorf("round-trip title: got %q", meta["title"])
	}
}
