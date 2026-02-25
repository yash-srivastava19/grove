package templates

import (
	"strings"
	"testing"
)

func TestGet_variableSubstitution(t *testing.T) {
	body := Get("meeting", "Sprint Planning", "2024-01-15")
	if !strings.Contains(body, "Sprint Planning") {
		t.Error("expected {{title}} to be replaced with 'Sprint Planning'")
	}
	if !strings.Contains(body, "2024-01-15") {
		t.Error("expected {{date}} to be replaced with '2024-01-15'")
	}
	if strings.Contains(body, "{{title}}") {
		t.Error("{{title}} placeholder should have been replaced")
	}
	if strings.Contains(body, "{{date}}") {
		t.Error("{{date}} placeholder should have been replaced")
	}
}

func TestGet_allTemplatesNonEmpty(t *testing.T) {
	nonDefault := []string{"meeting", "brainstorm", "research"}
	for _, name := range nonDefault {
		body := Get(name, "Title", "2024-01-15")
		if body == "" {
			t.Errorf("template %q returned empty body", name)
		}
	}
}

func TestGet_defaultIsEmpty(t *testing.T) {
	body := Get("default", "Title", "2024-01-15")
	if body != "" {
		t.Errorf("default template should return empty body, got %q", body)
	}
}

func TestGet_unknownFallsToDefault(t *testing.T) {
	body := Get("nonexistent", "Title", "2024-01-15")
	if body != "" {
		t.Errorf("unknown template should fall back to default (empty), got %q", body)
	}
}

func TestGet_meetingHasExpectedSections(t *testing.T) {
	body := Get("meeting", "My Meeting", "2024-01-15")
	for _, section := range []string{"## Agenda", "## Notes", "## Action Items"} {
		if !strings.Contains(body, section) {
			t.Errorf("meeting template missing section %q", section)
		}
	}
}

func TestGet_brainstormHasExpectedSections(t *testing.T) {
	body := Get("brainstorm", "Big Idea", "2024-01-15")
	for _, section := range []string{"## Core idea", "## Branches", "## Keep / discard"} {
		if !strings.Contains(body, section) {
			t.Errorf("brainstorm template missing section %q", section)
		}
	}
}

func TestGet_researchHasExpectedSections(t *testing.T) {
	body := Get("research", "Study", "2024-01-15")
	for _, section := range []string{"## Question", "## Sources", "## Conclusion"} {
		if !strings.Contains(body, section) {
			t.Errorf("research template missing section %q", section)
		}
	}
}

func TestNames_allPresent(t *testing.T) {
	expected := map[string]bool{
		"default":    false,
		"meeting":    false,
		"brainstorm": false,
		"research":   false,
	}
	for _, name := range Names {
		expected[name] = true
	}
	for name, found := range expected {
		if !found {
			t.Errorf("Names slice is missing %q", name)
		}
	}
}
