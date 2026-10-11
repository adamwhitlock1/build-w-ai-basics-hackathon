package family

import (
	"slices"
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"parent@example.com", "parent@example.com", true},
		{"  Parent@Example.COM ", "parent@example.com", true},
		{"a@b.co", "a@b.co", true},
		{"", "", false},
		{"parent", "parent", false},
		{"parent@example", "parent@example", false},
		{"@example.com", "@example.com", false},
		{"parent@.com", "parent@.com", false},
		{"parent@example.", "parent@example.", false},
		{"a@b@example.com", "a@b@example.com", false},
		{"pa rent@example.com", "pa rent@example.com", false},
		{strings.Repeat("a", 250) + "@example.com", strings.Repeat("a", 250) + "@example.com", false},
	}
	for _, tt := range tests {
		got, ok := normalizeEmail(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("normalizeEmail(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestCleanEmails(t *testing.T) {
	tests := []struct {
		name    string
		raw     []string
		want    []string
		wantBad string
	}{
		{"none", nil, nil, ""},
		{"lowercases and trims", []string{" Co@Example.com "}, []string{"co@example.com"}, ""},
		{"skips blanks", []string{"", "  ", "co@example.com"}, []string{"co@example.com"}, ""},
		{"dedupes", []string{"co@example.com", "CO@example.com"}, []string{"co@example.com"}, ""},
		{"skips own email", []string{"me@example.com", "co@example.com"}, []string{"co@example.com"}, ""},
		{"reports a bad entry", []string{"co@example.com", " nope "}, nil, "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, bad := cleanEmails(tt.raw, "me@example.com")
			if !slices.Equal(got, tt.want) || bad != tt.wantBad {
				t.Errorf("cleanEmails(%q) = %q, %q; want %q, %q", tt.raw, got, bad, tt.want, tt.wantBad)
			}
		})
	}
}

func TestPasswordProblem(t *testing.T) {
	tests := []struct {
		password string
		wantOK   bool
	}{
		{"short", false},
		{"12345678", true},
		{"pässwörd", true}, // 8 characters, 10 bytes
		{strings.Repeat("a", 72), true},
		{strings.Repeat("a", 73), false},
	}
	for _, tt := range tests {
		msg, _ := passwordProblem(tt.password)
		if ok := msg == ""; ok != tt.wantOK {
			t.Errorf("passwordProblem(%q) = %q; want ok = %v", tt.password, msg, tt.wantOK)
		}
	}
}
