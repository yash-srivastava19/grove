package notes

import (
	"strings"
	"time"
)

type Note struct {
	ID       string    // filename without extension
	Title    string
	Tags     []string
	Created  time.Time
	Updated  time.Time
	Body     string // content after frontmatter
	Raw      string // full file content
	Filename string // full path
}

// ParseFrontmatter extracts title/tags/dates from YAML-like frontmatter.
// Format:
//
//	---
//	title: My Note
//	tags: [tag1, tag2]
//	created: 2024-01-01T00:00:00Z
//	updated: 2024-01-01T00:00:00Z
//	---
func ParseFrontmatter(content string) (meta map[string]string, body string) {
	meta = make(map[string]string)
	// Normalize CRLF to LF
	content = strings.ReplaceAll(content, "\r\n", "\n")
	body = content

	if !strings.HasPrefix(content, "---") {
		return
	}

	end := strings.Index(content[3:], "\n---")
	if end == -1 {
		return
	}
	end += 3 // account for offset

	fm := content[4:end] // skip leading ---\n
	body = strings.TrimLeft(content[end+4:], "\n")

	for _, line := range strings.Split(fm, "\n") {
		if idx := strings.Index(line, ":"); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			meta[key] = val
		}
	}
	return
}

func parseTags(raw string) []string {
	raw = strings.Trim(raw, "[]")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		t = strings.Trim(t, "\"'")
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func BuildFrontmatter(n *Note) string {
	tags := "[]"
	if len(n.Tags) > 0 {
		quoted := make([]string, len(n.Tags))
		for i, t := range n.Tags {
			quoted[i] = t
		}
		tags = "[" + strings.Join(quoted, ", ") + "]"
	}
	return "---\ntitle: " + n.Title +
		"\ntags: " + tags +
		"\ncreated: " + n.Created.UTC().Format(time.RFC3339) +
		"\nupdated: " + n.Updated.UTC().Format(time.RFC3339) +
		"\n---\n\n"
}

func NoteFromRaw(id, filename, raw string, modTime time.Time) *Note {
	meta, body := ParseFrontmatter(raw)

	title := meta["title"]
	if title == "" {
		title = id
	}

	created := modTime
	if t, err := time.Parse(time.RFC3339, meta["created"]); err == nil {
		created = t
	}

	updated := modTime
	if t, err := time.Parse(time.RFC3339, meta["updated"]); err == nil {
		updated = t
	}

	return &Note{
		ID:       id,
		Title:    title,
		Tags:     parseTags(meta["tags"]),
		Created:  created,
		Updated:  updated,
		Body:     body,
		Raw:      raw,
		Filename: filename,
	}
}
