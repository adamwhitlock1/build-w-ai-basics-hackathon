package widgets

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestShorten(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"short text is kept", "Crew-11 docks.", 20, "Crew-11 docks."},
		{"exactly at the limit", "abcde", 5, "abcde"},
		{"joins lines", "one\n two\t three", 20, "one two three"},
		{"cuts at a word boundary", "The quick brown fox jumps", 15, "The quick…"},
		{"drops trailing punctuation", "Launch moved, again: details inside", 16, "Launch moved…"},
		{"no period before the ellipsis", "It flew. Then it landed safely.", 13, "It flew…"},
		{"cuts inside one very long word", "Supercalifragilistic", 10, "Supercali…"},
		{"counts characters, not bytes", "Café olé à la mode", 10, "Café olé…"},
	}
	for _, tt := range tests {
		got := shorten(tt.in, tt.limit)
		if got != tt.want {
			t.Errorf("%s: shorten(%q, %d) = %q, want %q", tt.name, tt.in, tt.limit, got, tt.want)
		}
		if n := utf8.RuneCountInString(got); n > tt.limit {
			t.Errorf("%s: result has %d characters, over the limit of %d", tt.name, n, tt.limit)
		}
		if strings.Contains(got, "\n") {
			t.Errorf("%s: result %q has a line break", tt.name, got)
		}
	}
}
