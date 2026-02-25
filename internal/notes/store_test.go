package notes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_CreateAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	note, err := s.Create("Hello World", []string{"test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if note.Title != "Hello World" {
		t.Errorf("Title: got %q", note.Title)
	}
	if note.ID != "hello-world" {
		t.Errorf("ID: got %q", note.ID)
	}

	loaded, err := s.Load(note.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Title != "Hello World" {
		t.Errorf("Loaded title: got %q", loaded.Title)
	}
}

func TestStore_LoadAll_sorted(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	// Write notes with explicit frontmatter so Updated times differ
	writeNote := func(id, title string, ts string) {
		content := "---\ntitle: " + title + "\ntags: []\ncreated: " + ts + "\nupdated: " + ts + "\n---\n\nbody\n"
		_ = os.WriteFile(filepath.Join(dir, id+".md"), []byte(content), 0644)
	}

	writeNote("first", "First", "2024-01-01T00:00:00Z")
	writeNote("second", "Second", "2024-06-01T00:00:00Z")
	writeNote("third", "Third", "2024-12-01T00:00:00Z")

	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 notes, got %d", len(all))
	}
	if all[0].Title != "Third" {
		t.Errorf("expected Third first (newest), got %q", all[0].Title)
	}
}

func TestStore_Delete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	note, _ := s.Create("To Delete", nil)
	if err := s.Delete(note.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Load(note.ID)
	if err == nil {
		t.Error("expected error loading deleted note")
	}
}

func TestStore_CreateDaily(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	note, err := s.CreateDaily()
	if err != nil {
		t.Fatalf("CreateDaily: %v", err)
	}

	// Should start with "daily-"
	if len(note.ID) < 6 || note.ID[:6] != "daily-" {
		t.Errorf("daily note ID: got %q", note.ID)
	}

	// Creating again should return the same note (not create new)
	note2, err := s.CreateDaily()
	if err != nil {
		t.Fatalf("CreateDaily again: %v", err)
	}
	if note.ID != note2.ID {
		t.Errorf("expected same daily note, got %q vs %q", note.ID, note2.ID)
	}
}

func TestStore_SlugCollision(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	n1, _ := s.Create("Hello World", nil)
	n2, _ := s.Create("Hello World", nil)

	if n1.ID == n2.ID {
		t.Error("expected different IDs for same title")
	}
	if n2.ID != "hello-world-2" {
		t.Errorf("collision ID: got %q", n2.ID)
	}
}

func TestStore_IgnoresNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	// Create a non-.md file
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0644)
	_, _ = s.Create("Real Note", nil)

	all, _ := s.LoadAll()
	if len(all) != 1 {
		t.Errorf("expected 1 note, got %d", len(all))
	}
}
