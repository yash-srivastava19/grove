package ui

import (
	"testing"
	"time"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello w…"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expected)
		}
	}
}

func TestHumanTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		t        time.Time
		expected string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m"},
		{now.Add(-3 * time.Hour), "3h"},
		{now.Add(-2 * 24 * time.Hour), "2d"},
		{now.Add(-10 * 24 * time.Hour), "1w"},
	}

	for _, tt := range tests {
		got := humanTime(tt.t)
		if got != tt.expected {
			t.Errorf("humanTime(%v ago) = %q, want %q", time.Since(tt.t).Round(time.Second), got, tt.expected)
		}
	}
}
