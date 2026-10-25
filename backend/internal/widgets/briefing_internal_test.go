package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
)

func TestPartOfDay(t *testing.T) {
	tests := []struct {
		hour, min int
		want      string
	}{
		{0, 0, "morning"},
		{11, 59, "morning"},
		{12, 0, "afternoon"},
		{17, 59, "afternoon"},
		{18, 0, "evening"},
		{23, 59, "evening"},
	}
	for _, tt := range tests {
		at := time.Date(2026, 10, 13, tt.hour, tt.min, 0, 0, time.UTC)
		if got := partOfDay(at); got != tt.want {
			t.Errorf("partOfDay(%s) = %q, want %q", at.Format("15:04"), got, tt.want)
		}
	}
}

func TestShouldWrite(t *testing.T) {
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	remembered := func(writes int, ago time.Duration) *rememberedBriefing {
		return &rememberedBriefing{day: briefingDay{Writes: writes}, fingerprint: "old", writtenAt: now.Add(-ago)}
	}
	tests := []struct {
		name        string
		today       *rememberedBriefing
		fingerprint string
		want        bool
	}{
		{"nothing from today", nil, "new", true},
		{"same facts", remembered(1, 3*time.Hour), "old", false},
		{"changed facts, cap reached", remembered(maxBriefingWrites, 3*time.Hour), "new", false},
		{"changed facts, too soon", remembered(1, 3*time.Minute), "new", false},
		{"changed facts, gap passed", remembered(2, 11*time.Minute), "new", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldWrite(tt.today, tt.fingerprint, now); got != tt.want {
				t.Errorf("shouldWrite = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemberLocks(t *testing.T) {
	var locks memberLocks
	timeout := time.After(2 * time.Second)

	// Member 1's lock is held, so a second request for member 1 waits...
	unlock := locks.lock(1)
	got := make(chan struct{})
	go func() {
		release := locks.lock(1)
		close(got)
		release()
	}()
	select {
	case <-got:
		t.Fatal("a second lock on the same member didn't wait")
	case <-time.After(50 * time.Millisecond):
	}

	// ...while member 2 doesn't wait at all.
	other := make(chan struct{})
	go func() {
		release := locks.lock(2)
		close(other)
		release()
	}()
	select {
	case <-other:
	case <-timeout:
		t.Fatal("a different member waited on member 1's lock")
	}

	unlock()
	select {
	case <-got:
	case <-timeout:
		t.Fatal("the waiting request never got the lock")
	}
}

// testResults is a ResultStore in memory, for this package's own tests.
type testResults struct {
	mu   sync.Mutex
	rows map[string]store.CuratedResult
	puts int
}

func newTestResults() *testResults { return &testResults{rows: map[string]store.CuratedResult{}} }

func (m *testResults) GetCuratedResult(_ context.Context, userID int64, kind string) (store.CuratedResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[kind]
	if !ok || r.UserID != userID {
		return store.CuratedResult{}, store.ErrNotFound
	}
	return r, nil
}

func (m *testResults) PutCuratedResult(_ context.Context, r store.CuratedResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	m.rows[r.Kind] = r
	return nil
}

func (m *testResults) put(t *testing.T, userID int64, kind, inputs string, payload any, at time.Time) {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	m.rows[kind] = store.CuratedResult{UserID: userID, Kind: kind, Inputs: inputs, Payload: b, UpdatedAt: at}
}

// countingWeather is a forecast source that counts its calls.
type countingWeather struct {
	results []sources.Result
	err     error
	calls   int
	asked   string
}

func (w *countingWeather) Name() string  { return sources.WeatherForecast }
func (w *countingWeather) Shape() string { return "test forecast" }
func (w *countingWeather) Search(_ context.Context, query string, _ sources.Audience) ([]sources.Result, error) {
	w.calls++
	w.asked = query
	return w.results, w.err
}

func denverForecast(today string) []sources.Result {
	return []sources.Result{
		{Title: "Right now in Denver, Colorado", Snippet: "61°F, feels like 59°F. Clear sky.", Site: "Open-Meteo"},
		{Title: "Today (Tue, Oct 13)", Snippet: today, Site: "Open-Meteo"},
		{Title: "Wed, Oct 14", Snippet: "Cloudy. High 68°F, low 50°F.", Site: "Open-Meteo"},
	}
}

func newTestBriefing(results ResultStore, weather sources.Source) *BriefingHandler {
	return &BriefingHandler{
		weather: weather,
		results: results,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:     timeNow,
	}
}

const memberID = 7

var denver, _ = time.LoadLocation("America/Denver")

func TestWeatherFact(t *testing.T) {
	now := time.Date(2026, 10, 13, 8, 0, 0, 0, denver)
	const sunny = "Sunny. High 75°F, low 58°F. 10% chance of rain."

	t.Run("asked once an hour", func(t *testing.T) {
		weather := &countingWeather{results: denverForecast(sunny)}
		h := newTestBriefing(newTestResults(), weather)
		today, tomorrow := h.weatherFact(context.Background(), memberID, "Denver, CO", sources.Everyone, "2026-10-13", now)
		if today != sunny || tomorrow != "Cloudy. High 68°F, low 50°F." {
			t.Fatalf("forecast = %q, %q; want %q and Wednesday's", today, tomorrow, sunny)
		}
		if weather.asked != "Denver, CO" {
			t.Errorf("asked for %q", weather.asked)
		}
		h.weatherFact(context.Background(), memberID, "denver,  co", sources.Everyone, "2026-10-13", now.Add(50*time.Minute))
		if weather.calls != 1 {
			t.Errorf("calls within the hour = %d, want 1", weather.calls)
		}
		h.weatherFact(context.Background(), memberID, "Denver, CO", sources.Everyone, "2026-10-13", now.Add(61*time.Minute))
		if weather.calls != 2 {
			t.Errorf("calls after the hour = %d, want 2", weather.calls)
		}
	})

	t.Run("new location asks again", func(t *testing.T) {
		weather := &countingWeather{results: denverForecast(sunny)}
		h := newTestBriefing(newTestResults(), weather)
		h.weatherFact(context.Background(), memberID, "Denver, CO", sources.Everyone, "2026-10-13", now)
		h.weatherFact(context.Background(), memberID, "Boulder, CO", sources.Everyone, "2026-10-13", now.Add(time.Minute))
		if weather.calls != 2 {
			t.Errorf("calls = %d, want 2", weather.calls)
		}
	})

	t.Run("failure falls back to earlier today", func(t *testing.T) {
		results := newTestResults()
		results.put(t, memberID, kindBriefingWeather, "denver, co\n2026-10-13", briefingWeather{Forecast: sunny, Tomorrow: "Cloudy."}, now.Add(-2*time.Hour))
		weather := &countingWeather{err: errors.New("open-meteo down")}
		h := newTestBriefing(results, weather)
		if today, tomorrow := h.weatherFact(context.Background(), memberID, "Denver, CO", sources.Everyone, "2026-10-13", now); today != sunny || tomorrow != "Cloudy." {
			t.Errorf("forecast = %q, %q; want the earlier ones", today, tomorrow)
		}
		if weather.calls != 1 {
			t.Errorf("calls = %d, want 1", weather.calls)
		}
	})

	t.Run("failure with nothing earlier", func(t *testing.T) {
		results := newTestResults()
		results.put(t, memberID, kindBriefingWeather, "denver, co\n2026-10-12", briefingWeather{Forecast: sunny}, now.Add(-14*time.Hour))
		h := newTestBriefing(results, &countingWeather{err: errors.New("open-meteo down")})
		if today, tomorrow := h.weatherFact(context.Background(), memberID, "Denver, CO", sources.Everyone, "2026-10-13", now); today != "" || tomorrow != "" {
			t.Errorf("forecast = %q, %q; want none (yesterday's doesn't count)", today, tomorrow)
		}
	})

	t.Run("no place name in the forecast", func(t *testing.T) {
		h := newTestBriefing(newTestResults(), &countingWeather{results: denverForecast(sunny)})
		today, tomorrow := h.weatherFact(context.Background(), memberID, "Denver, CO", sources.Everyone, "2026-10-13", now)
		for _, got := range []string{today, tomorrow} {
			if strings.Contains(got, "Denver") || strings.Contains(got, "61°F") {
				t.Errorf("forecast %q holds the place or the current conditions", got)
			}
		}
	})
}

// factsCase sets up a member's remembered cards for gatherFacts.
type factsCase struct {
	profile store.MemberProfile
	aud     sources.Audience
	now     time.Time
	today   *todayDay
	// The remembered Articles and Local Events, and the inputs they were
	// picked for; empty inputs means "picked for the current profile".
	articles, events             *Widget
	articlesInputs, eventsInputs string
	weather                      *countingWeather
}

func (c factsCase) gather(t *testing.T) briefingFacts {
	t.Helper()
	results := newTestResults()
	if c.today != nil {
		results.put(t, memberID, kindToday, "link\ntz\ndate", c.today, c.now)
	}
	if c.articles != nil {
		inputs := c.articlesInputs
		if inputs == "" {
			inputs = normalizeInput(c.profile.Interests)
		}
		results.put(t, memberID, kindArticles, inputs, c.articles, c.now)
	}
	if c.events != nil {
		inputs := c.eventsInputs
		if inputs == "" {
			inputs = normalizeInput(c.profile.Interests) + "\n" + normalizeInput(c.profile.Location)
		}
		results.put(t, memberID, kindEvents, inputs, c.events, c.now)
	}
	weather := c.weather
	if weather == nil {
		weather = &countingWeather{results: denverForecast("Sunny. High 75°F, low 58°F.")}
	}
	h := newTestBriefing(results, weather)
	return h.gatherFacts(context.Background(), memberID, c.profile, c.aud, denver, c.now)
}

func at(hour, min int) time.Time { return time.Date(2026, 10, 13, hour, min, 0, 0, denver) }

func event(title string, start, end time.Time) TodayEvent {
	return TodayEvent{Title: title, Start: start, End: end}
}

func allDay(title string) TodayEvent {
	return TodayEvent{Title: title, Start: at(0, 0), End: at(0, 0).AddDate(0, 0, 1), AllDay: true}
}

func sampleDay() *todayDay {
	tomorrow := at(9, 0).AddDate(0, 0, 1)
	return &todayDay{
		Date: "2026-10-13", TimeZone: "America/Denver",
		Today:    []TodayEvent{allDay("Spring break"), event("Breakfast", at(7, 0), at(8, 0)), event("Soccer", at(16, 0), at(17, 30))},
		Tomorrow: []TodayEvent{event("Dentist", tomorrow, tomorrow.Add(time.Hour))},
	}
}

func items(titles ...string) []llm.Item {
	out := make([]llm.Item, len(titles))
	for i, title := range titles {
		out[i] = llm.Item{Title: title, Date: "Sat, Oct 17", Label: "Union Station"}
	}
	return out
}

func describe(events []llm.BriefingEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Time + " " + e.Title
	}
	return out
}

func TestGatherFactsCalendar(t *testing.T) {
	morning := factsCase{now: at(9, 0), today: sampleDay()}

	t.Run("ended events leave the prompt", func(t *testing.T) {
		f := morning.gather(t)
		if got, want := describe(f.prompt.Today), []string{"All day Spring break", "4:00 PM Soccer"}; !slices.Equal(got, want) {
			t.Errorf("today = %v, want %v", got, want)
		}
		if f.prompt.Tomorrow != nil {
			t.Errorf("tomorrow in the morning = %v, want none", f.prompt.Tomorrow)
		}
		if !slices.Equal(f.uses, []string{usesCalendar}) {
			t.Errorf("uses = %v", f.uses)
		}
	})

	t.Run("but not the fingerprint", func(t *testing.T) {
		later := morning
		later.now = at(10, 30) // still morning, breakfast long over
		if morning.gather(t).fingerprint != later.gather(t).fingerprint {
			t.Error("the fingerprint changed as the morning went on")
		}
	})

	t.Run("evening adds tomorrow", func(t *testing.T) {
		evening := morning
		evening.now = at(19, 0)
		f := evening.gather(t)
		if got, want := describe(f.prompt.Tomorrow), []string{"9:00 AM Dentist"}; !slices.Equal(got, want) {
			t.Errorf("tomorrow = %v, want %v", got, want)
		}
		if got, want := describe(f.prompt.Today), []string{"All day Spring break"}; !slices.Equal(got, want) {
			t.Errorf("today = %v, want %v", got, want)
		}
	})

	for _, tt := range []struct {
		name   string
		change func(*todayDay)
	}{
		{"another day", func(d *todayDay) { d.Date = "2026-10-12" }},
		{"another time zone", func(d *todayDay) { d.TimeZone = "America/New_York" }},
		{"calendar gone", func(d *todayDay) { d.EmptyReason = EmptyCalendarGone }},
	} {
		t.Run("ignored: "+tt.name, func(t *testing.T) {
			c := morning
			c.today = sampleDay()
			tt.change(c.today)
			if f := c.gather(t); len(f.uses) != 0 || f.prompt.Today != nil {
				t.Errorf("used the calendar: uses %v, today %v", f.uses, f.prompt.Today)
			}
		})
	}
}

func TestGatherFactsArticlesAndNearby(t *testing.T) {
	profile := store.MemberProfile{Interests: "space, soccer", Location: "Denver, CO"}
	base := factsCase{
		profile: profile, now: at(9, 0),
		articles: &Widget{Items: items("A1", "A2", "A3", "A4")},
		events:   &Widget{Items: items("E1", "", "E2", "E3", "E4")},
	}

	t.Run("at most three of each", func(t *testing.T) {
		f := base.gather(t)
		if !slices.Equal(f.prompt.Articles, []string{"A1", "A2", "A3"}) {
			t.Errorf("articles = %v", f.prompt.Articles)
		}
		want := []llm.BriefingNearby{
			{Name: "E1", Date: "Sat, Oct 17", Venue: "Union Station"},
			{Name: "E2", Date: "Sat, Oct 17", Venue: "Union Station"},
			{Name: "E3", Date: "Sat, Oct 17", Venue: "Union Station"},
		}
		if !slices.Equal(f.prompt.Nearby, want) {
			t.Errorf("nearby = %v", f.prompt.Nearby)
		}
		if !slices.Equal(f.uses, []string{usesWeather, usesArticles, usesNearby}) {
			t.Errorf("uses = %v", f.uses)
		}
	})

	t.Run("picked for old interests", func(t *testing.T) {
		c := base
		c.articlesInputs = "cooking"
		c.eventsInputs = "cooking\ndenver, co"
		f := c.gather(t)
		if f.prompt.Articles != nil || f.prompt.Nearby != nil {
			t.Errorf("used stale picks: %v %v", f.prompt.Articles, f.prompt.Nearby)
		}
	})

	t.Run("kid-safety must match", func(t *testing.T) {
		kid := base
		kid.aud = sources.Kids // the remembered picks are a parent's
		if f := kid.gather(t); f.prompt.Articles != nil || f.prompt.Nearby != nil {
			t.Errorf("a kid got a parent's picks: %v %v", f.prompt.Articles, f.prompt.Nearby)
		}

		parent := base
		parent.articles = &Widget{Items: items("A1"), KidSafe: true}
		parent.events = &Widget{Items: items("E1"), KidSafe: true}
		if f := parent.gather(t); f.prompt.Articles != nil || f.prompt.Nearby != nil {
			t.Errorf("a parent got a kid's picks: %v %v", f.prompt.Articles, f.prompt.Nearby)
		}

		kidsOwn := kid
		kidsOwn.articles = &Widget{Items: items("A1"), KidSafe: true}
		if f := kidsOwn.gather(t); !slices.Equal(f.prompt.Articles, []string{"A1"}) {
			t.Errorf("a kid's own kid-safe articles = %v", f.prompt.Articles)
		}
	})

	t.Run("evening weather is tomorrow's", func(t *testing.T) {
		evening := base
		evening.now = at(19, 0)
		f := evening.gather(t)
		if f.prompt.Weather != "" || f.prompt.WeatherTomorrow != "Cloudy. High 68°F, low 50°F." {
			t.Errorf("evening weather = %q / tomorrow %q", f.prompt.Weather, f.prompt.WeatherTomorrow)
		}
		if morning := base.gather(t); morning.prompt.Weather == "" || morning.prompt.WeatherTomorrow != "" {
			t.Errorf("morning weather = %q / tomorrow %q", morning.prompt.Weather, morning.prompt.WeatherTomorrow)
		}
	})

	t.Run("no location, no weather or nearby", func(t *testing.T) {
		c := base
		c.profile.Location = ""
		weather := &countingWeather{}
		c.weather = weather
		f := c.gather(t)
		if f.prompt.Weather != "" || f.prompt.Nearby != nil || weather.calls != 0 {
			t.Errorf("weather %q, nearby %v, forecast calls %d", f.prompt.Weather, f.prompt.Nearby, weather.calls)
		}
		if !slices.Equal(f.uses, []string{usesArticles}) {
			t.Errorf("uses = %v", f.uses)
		}
	})

	t.Run("nothing at all", func(t *testing.T) {
		f := factsCase{now: at(9, 0)}.gather(t)
		if len(f.uses) != 0 {
			t.Errorf("uses = %v, want none", f.uses)
		}
	})
}

func TestFingerprint(t *testing.T) {
	full := factsCase{
		profile:  store.MemberProfile{Interests: "space", Location: "Denver, CO"},
		now:      at(9, 0),
		today:    sampleDay(),
		articles: &Widget{Items: items("A1")},
	}
	first := full.gather(t).fingerprint
	if again := full.gather(t).fingerprint; again != first {
		t.Fatalf("the same facts gave %q, then %q", first, again)
	}

	changes := []struct {
		name   string
		change func(*factsCase)
	}{
		{"an event title", func(c *factsCase) { c.today = sampleDay(); c.today.Today[2].Title = "Soccer game" }},
		{"the forecast", func(c *factsCase) {
			c.weather = &countingWeather{results: denverForecast("Rain. High 50°F, low 41°F.")}
		}},
		{"the part of day", func(c *factsCase) { c.now = at(13, 0) }},
		{"a headline", func(c *factsCase) { c.articles = &Widget{Items: items("A2")} }},
	}
	for _, tt := range changes {
		t.Run(tt.name, func(t *testing.T) {
			c := full
			tt.change(&c)
			if c.gather(t).fingerprint == first {
				t.Error("the fingerprint didn't change")
			}
		})
	}
}
