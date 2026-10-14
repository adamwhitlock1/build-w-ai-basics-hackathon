package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
)

var briefingMorning = time.Date(2026, 10, 13, 7, 10, 0, 0, time.UTC)

func fullFacts() llm.BriefingFacts {
	return llm.BriefingFacts{
		Now:      briefingMorning,
		Weather:  "Sunny. High 75°F, low 58°F. 10% chance of rain.",
		Today:    []llm.BriefingEvent{{Time: "All day", Title: "Spring break"}, {Time: "4:00 PM", Title: "Soccer practice", Location: "Lincoln Park"}},
		Articles: []string{"Starlink launch set for Tuesday"},
		Nearby:   []llm.BriefingNearby{{Name: "Farmers market", Date: "Sat, Oct 17", Venue: "Union Station"}},
	}
}

func TestBriefingUserMessage(t *testing.T) {
	tests := []struct {
		name    string
		facts   llm.BriefingFacts
		want    []string
		notWant []string
	}{
		{
			name:  "every section",
			facts: fullFacts(),
			want: []string{
				"Weather today: Sunny. High 75°F",
				"Calendar today:\n- All day Spring break\n- 4:00 PM Soccer practice (Lincoln Park)",
				"News headlines:\n- Starlink launch set for Tuesday",
				"Happening nearby:\n- Farmers market (Sat, Oct 17) at Union Station",
			},
			notWant: []string{"Calendar tomorrow"},
		},
		{
			name:    "calendar only",
			facts:   llm.BriefingFacts{Now: briefingMorning, Today: []llm.BriefingEvent{{Time: "9:00 AM", Title: "Dentist"}}},
			want:    []string{"Calendar today:\n- 9:00 AM Dentist"},
			notWant: []string{"Weather", "News", "nearby", "tomorrow"},
		},
		{
			name: "tomorrow when given",
			facts: llm.BriefingFacts{
				Now:             briefingMorning.Add(12 * time.Hour),
				Tomorrow:        []llm.BriefingEvent{{Time: "9:00 AM", Title: "Dentist"}},
				WeatherTomorrow: "Cloudy. High 68°F.",
			},
			want:    []string{"Weather tomorrow: Cloudy. High 68°F.", "Calendar tomorrow:\n- 9:00 AM Dentist"},
			notWant: []string{"Calendar today", "Weather today"},
		},
		{
			name:    "nothing",
			facts:   llm.BriefingFacts{Now: briefingMorning},
			notWant: []string{"Weather", "Calendar", "News", "nearby"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := llm.BriefingUserMessage(tt.facts)
			for _, w := range tt.want {
				if !strings.Contains(msg, w) {
					t.Errorf("message lacks %q:\n%s", w, msg)
				}
			}
			for _, w := range append(tt.notWant, "@", "Name:", "Location:") {
				if strings.Contains(msg, w) {
					t.Errorf("message contains %q:\n%s", w, msg)
				}
			}
		})
	}
}

// briefingJSON is a model reply holding text.
func briefingJSON(text string) string {
	b, _ := json.Marshal(map[string]string{"briefing": text})
	return string(b)
}

func TestWriteBriefing(t *testing.T) {
	t.Run("good reply", func(t *testing.T) {
		want := "Sunny and 75° today, with soccer at 4. Starlink launches Tuesday."
		fake, client := newFake(t, ok(briefingJSON("  "+want+"\n")))
		got, err := client.WriteBriefing(context.Background(), fullFacts(), sources.Everyone)
		if err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		if got != want {
			t.Errorf("briefing = %q, want %q", got, want)
		}
		req := fake.reqs[0]
		if req.ResponseFormat.JSONSchema.Name != "morning_briefing" {
			t.Errorf("schema name = %q", req.ResponseFormat.JSONSchema.Name)
		}
		if !strings.Contains(req.Messages[0].Content, "Tuesday, October 13, 7:10 AM") {
			t.Errorf("system prompt lacks the local time:\n%s", req.Messages[0].Content)
		}
		if !strings.Contains(req.Messages[1].Content, "Soccer practice") {
			t.Errorf("user message lacks the facts:\n%s", req.Messages[1].Content)
		}
	})

	t.Run("blank reply is retried, then refused", func(t *testing.T) {
		fake, client := newFake(t, ok(briefingJSON("")), ok(briefingJSON("   ")))
		_, err := client.WriteBriefing(context.Background(), fullFacts(), sources.Everyone)
		if !errors.Is(err, llm.ErrNoBriefing) {
			t.Fatalf("err = %v, want ErrNoBriefing", err)
		}
		if len(fake.reqs) != 2 {
			t.Errorf("got %d requests, want 2", len(fake.reqs))
		}
	})

	t.Run("far too long is retried", func(t *testing.T) {
		long := strings.Repeat("word ", 100)
		fake, client := newFake(t, ok(briefingJSON(long)), ok(briefingJSON("Sunny today.")))
		got, err := client.WriteBriefing(context.Background(), fullFacts(), sources.Everyone)
		if err != nil || got != "Sunny today." {
			t.Fatalf("WriteBriefing = %q, %v", got, err)
		}
		if len(fake.reqs) != 2 {
			t.Errorf("got %d requests, want 2", len(fake.reqs))
		}
	})

	t.Run("slightly long is trimmed at a sentence end", func(t *testing.T) {
		first := "Sunny and 75° today, a lovely day to be outside, with a light breeze in the afternoon and clear skies all evening long."
		second := "Soccer practice is at 4 at Lincoln Park, so pack water and your cleats before you head out the door this afternoon."
		third := "And the Starlink launch you were reading about is set for Tuesday morning, weather permitting, from the Cape."
		text := first + " " + second + " " + third
		if n := utf8.RuneCountInString(text); n <= llm.MaxBriefingChars || n > 400 {
			t.Fatalf("test text is %d characters; want 321-400", n)
		}
		_, client := newFake(t, ok(briefingJSON(text)))
		got, err := client.WriteBriefing(context.Background(), fullFacts(), sources.Everyone)
		if err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		if want := first + " " + second; got != want {
			t.Errorf("briefing = %q, want %q", got, want)
		}
	})

	t.Run("kid guidance", func(t *testing.T) {
		for _, aud := range []sources.Audience{sources.Kids, sources.Everyone} {
			fake, client := newFake(t, ok(briefingJSON("Sunny today.")))
			if _, err := client.WriteBriefing(context.Background(), fullFacts(), aud); err != nil {
				t.Fatalf("WriteBriefing: %v", err)
			}
			if got, want := strings.Contains(fake.reqs[0].Messages[0].Content, llm.KidGuidance), aud == sources.Kids; got != want {
				t.Errorf("audience %v: kid guidance = %v, want %v", aud, got, want)
			}
		}
	})

	t.Run("evening leads with tomorrow", func(t *testing.T) {
		facts := fullFacts()
		facts.Now = briefingMorning.Add(12 * time.Hour)
		fake, client := newFake(t, ok(briefingJSON("Dentist at 9 tomorrow.")))
		if _, err := client.WriteBriefing(context.Background(), facts, sources.Everyone); err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		if !strings.Contains(fake.reqs[0].Messages[0].Content, "lead with tomorrow") {
			t.Error("evening system prompt doesn't mention tomorrow")
		}
	})

	t.Run("out of credits", func(t *testing.T) {
		_, client := newFake(t, reply{status: http.StatusPaymentRequired, body: `{"error":{"message":"no credits","code":402}}`})
		_, err := client.WriteBriefing(context.Background(), fullFacts(), sources.Everyone)
		var upstream *llm.UpstreamError
		if !errors.As(err, &upstream) || upstream.Status != http.StatusPaymentRequired {
			t.Fatalf("err = %v, want an UpstreamError with status 402", err)
		}
	})
}
