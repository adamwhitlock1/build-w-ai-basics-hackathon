package main

import "testing"

func TestParseBriefer(t *testing.T) {
	for in, want := range map[string]string{"": "genkit", "genkit": "genkit", "legacy": "legacy"} {
		got, err := parseBriefer(in)
		if err != nil || got != want {
			t.Errorf("parseBriefer(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"Genkit", "openai", " legacy"} {
		if _, err := parseBriefer(in); err == nil {
			t.Errorf("parseBriefer(%q) succeeded, want an error", in)
		}
	}
}
