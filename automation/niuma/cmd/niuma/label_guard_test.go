package main

import (
	"testing"
)

func TestIsInAllowlist(t *testing.T) {
	tests := []struct {
		name      string
		actor     string
		allowlist []string
		want      bool
	}{
		{
			name:      "actor in allowlist",
			actor:     "github-actions[bot]",
			allowlist: []string{"github-actions[bot]", "niuma-bot"},
			want:      true,
		},
		{
			name:      "actor not in allowlist",
			actor:     "random-user",
			allowlist: []string{"github-actions[bot]", "niuma-bot"},
			want:      false,
		},
		{
			name:      "empty allowlist",
			actor:     "github-actions[bot]",
			allowlist: []string{},
			want:      false,
		},
		{
			name:      "nil allowlist",
			actor:     "github-actions[bot]",
			allowlist: nil,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInAllowlist(tt.actor, tt.allowlist)
			if got != tt.want {
				t.Errorf("isInAllowlist(%q, %v) = %v, want %v", tt.actor, tt.allowlist, got, tt.want)
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"github-actions[bot],niuma-bot", []string{"github-actions[bot]", "niuma-bot"}},
		{"  a ,  , b  ", []string{"a", "b"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := splitAndTrim(tt.input, ",")
		if len(got) != len(tt.want) {
			t.Errorf("splitAndTrim(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitAndTrim(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFormatLabelGuardMarker(t *testing.T) {
	got := formatLabelGuardMarker("dry-run", 42, "labeled", "bot:fix", "random-user")
	want := "<!-- BOT:LABEL_GUARD mode=dry-run issue=42 action=labeled label=bot:fix actor=random-user -->"
	if got != want {
		t.Errorf("formatLabelGuardMarker() = %q, want %q", got, want)
	}
}
