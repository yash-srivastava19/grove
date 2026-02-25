package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) LoadAll() ([]*Note, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var notes []*Note
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		note, err := s.loadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		notes = append(notes, note)
	}

	// Sort by updated time, newest first
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Updated.After(notes[j].Updated)
	})

	return notes, nil
}

func (s *Store) Load(id string) (*Note, error) {
	return s.loadFile(filepath.Join(s.dir, id+".md"))
}

func (s *Store) loadFile(path string) (*Note, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	id := strings.TrimSuffix(filepath.Base(path), ".md")
	return NoteFromRaw(id, path, string(data), info.ModTime()), nil
}

func (s *Store) Save(note *Note) error {
	note.Updated = time.Now()
	content := BuildFrontmatter(note) + note.Body
	note.Raw = content
	return os.WriteFile(note.Filename, []byte(content), 0644)
}

func (s *Store) Create(title string, tags []string) (*Note, error) {
	id := slugify(title)
	// Avoid collisions
	base := id
	for i := 2; ; i++ {
		path := filepath.Join(s.dir, id+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}

	now := time.Now()
	note := &Note{
		ID:       id,
		Title:    title,
		Tags:     tags,
		Created:  now,
		Updated:  now,
		Body:     "",
		Filename: filepath.Join(s.dir, id+".md"),
	}

	if err := s.Save(note); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *Store) CreateDaily() (*Note, error) {
	today := time.Now().Format("2006-01-02")
	id := "daily-" + today
	path := filepath.Join(s.dir, id+".md")

	if _, err := os.Stat(path); err == nil {
		return s.loadFile(path)
	}

	return s.Create("Daily "+today, []string{"daily"})
}

func (s *Store) Delete(id string) error {
	return os.Remove(filepath.Join(s.dir, id+".md"))
}

func (s *Store) Reload(note *Note) (*Note, error) {
	return s.loadFile(note.Filename)
}

func slugify(title string) string {
	s := strings.ToLower(title)
	var out strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			out.WriteRune('-')
		}
	}
	result := strings.Trim(out.String(), "-")
	// collapse consecutive dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	if result == "" {
		result = fmt.Sprintf("note-%d", time.Now().Unix())
	}
	return result
}
