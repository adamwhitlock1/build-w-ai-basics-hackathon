package widgets_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"kinlight/internal/auth"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// fakeBriefer writes canned briefings and records what it was given.
type fakeBriefer struct {
	mu    sync.Mutex
	text  string
	err   error
	calls int
	facts []llm.BriefingFacts
	auds  []sources.Audience
}

func (f *fakeBriefer) WriteBriefing(_ context.Context, facts llm.BriefingFacts, aud sources.Audience) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.facts = append(f.facts, facts)
	f.auds = append(f.auds, aud)
	if f.err != nil {
		return "", f.err
	}
	return fmt.Sprintf("%s (#%d)", f.text, f.calls), nil
}

func (f *fakeBriefer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// briefingNow is the tests' clock: a Tuesday morning in Denver.
var briefingNow = time.Date(2026, 10, 13, 7, 30, 0, 0, time.FixedZone("MDT", -6*3600))

const briefingTZ = "America/Denver"

// denverWeather answers forecasts for "Denver, CO".
func denverWeather() *querySource {
	return &querySource{name: sources.WeatherForecast, results: map[string][]sources.Result{
		"Denver, CO": {
			{Title: "Right now in Denver, Colorado", Snippet: "61°F. Clear sky."},
			{Title: "Today (Tue, Oct 13)", Snippet: "Sunny. High 75°F, low 58°F."},
		},
	}}
}

// briefingSetup is one BriefingHandler with its fakes and a movable clock.
type briefingSetup struct {
	h       *widgets.BriefingHandler
	briefer *fakeBriefer
	results *memResults
	weather *querySource
	now     time.Time
}

func newBriefingSetup(profiles widgets.ProfileStore) *briefingSetup {
	s := &briefingSetup{
		briefer: &fakeBriefer{text: "Sunny and 75° today."},
		results: newMemResults(),
		weather: denverWeather(),
		now:     briefingNow,
	}
	s.h = widgets.NewBriefingHandler(s.briefer, sources.NewRegistry(s.weather), profiles, s.results, discardLogger)
	widgets.SetBriefingClock(s.h, func() time.Time { return s.now })
	return s
}

// get asks for user's (or, with subject, subject's) briefing.
func (s *briefingSetup) get(t *testing.T, user store.User, subject *store.User) (*httptest.ResponseRecorder, widgets.BriefingWidget) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/widgets/briefing?tz="+briefingTZ, nil)
	ctx := auth.WithUser(req.Context(), user)
	if subject != nil {
		ctx = auth.WithSubject(ctx, *subject)
	}
	rec := httptest.NewRecorder()
	s.h.Briefing(rec, req.WithContext(ctx))
	var w widgets.BriefingWidget
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &w); err != nil {
			t.Fatalf("decode %s: %v", rec.Body, err)
		}
		if w.Stale {
			t.Errorf("briefing reported stale: %s", rec.Body)
		}
		if w.Uses == nil {
			t.Errorf("uses is null: %s", rec.Body)
		}
	}
	return rec, w
}

// remembered is the member's remembered briefing payload.
func (s *briefingSetup) remembered(t *testing.T, userID int64) (store.CuratedResult, map[string]any) {
	t.Helper()
	r, ok := s.results.rows[resultKey(userID, "briefing")]
	if !ok {
		t.Fatal("no briefing remembered")
	}
	var payload map[string]any
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	return r, payload
}

// withLocation is a profile with a location, so there's always weather.
var withLocation = fakeProfiles{profile: store.MemberProfile{Location: "Denver, CO"}}

func TestBriefingNothingYet(t *testing.T) {
	s := newBriefingSetup(fakeProfiles{})
	rec, w := s.get(t, alice, nil)
	if rec.Code != http.StatusOK || w.EmptyReason != widgets.EmptyNothingYet || w.Text != "" {
		t.Fatalf("got %d %+v, want 200 nothing_yet", rec.Code, w)
	}
	if s.briefer.calls != 0 || s.results.puts != 0 {
		t.Errorf("AI calls %d, remembered %d; want neither", s.briefer.calls, s.results.puts)
	}
	if w.Date != "2026-10-13" {
		t.Errorf("date = %q", w.Date)
	}
}

func TestBriefingWrittenOnceAndRemembered(t *testing.T) {
	s := newBriefingSetup(withLocation)
	rec, w := s.get(t, alice, nil)
	if rec.Code != http.StatusOK || w.Text != "Sunny and 75° today. (#1)" || w.EmptyReason != "" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if len(w.Uses) != 1 || w.Uses[0] != "weather" {
		t.Errorf("uses = %v, want [weather]", w.Uses)
	}
	if !w.UpdatedAt.Equal(briefingNow) {
		t.Errorf("updatedAt = %v, want %v", w.UpdatedAt, briefingNow)
	}
	if got := s.briefer.facts[0].Weather; got != "Sunny. High 75°F, low 58°F." {
		t.Errorf("weather fact = %q", got)
	}
	if _, payload := s.remembered(t, alice.ID); payload["writes"] != 1.0 || payload["timeZone"] != briefingTZ {
		t.Errorf("remembered %v", payload)
	}

	// An hour later, nothing has changed: no new call.
	s.now = briefingNow.Add(time.Hour)
	_, again := s.get(t, alice, nil)
	if s.briefer.calls != 1 || again.Text != w.Text || !again.UpdatedAt.Equal(briefingNow) {
		t.Errorf("second visit: calls %d, %+v", s.briefer.calls, again)
	}
}

func TestBriefingRewrittenWhenFactsChange(t *testing.T) {
	s := newBriefingSetup(withLocation)
	s.get(t, alice, nil)

	// Today's events arrive 11 minutes later.
	s.now = briefingNow.Add(11 * time.Minute)
	putToday(t, s.results, alice.ID, s.now, "Soccer")
	_, w := s.get(t, alice, nil)
	if s.briefer.calls != 2 || w.Text != "Sunny and 75° today. (#2)" {
		t.Fatalf("calls %d, text %q; want a rewrite", s.briefer.calls, w.Text)
	}
	if _, payload := s.remembered(t, alice.ID); payload["writes"] != 2.0 {
		t.Errorf("writes = %v, want 2", payload["writes"])
	}
	if facts := s.briefer.facts[1]; len(facts.Today) != 1 || facts.Today[0].Title != "Soccer" {
		t.Errorf("rewrite facts today = %+v", facts.Today)
	}
}

func TestBriefingTooSoonToRewrite(t *testing.T) {
	s := newBriefingSetup(withLocation)
	s.get(t, alice, nil)
	s.now = briefingNow.Add(3 * time.Minute)
	putToday(t, s.results, alice.ID, s.now, "Soccer")
	if _, w := s.get(t, alice, nil); s.briefer.calls != 1 || w.Text != "Sunny and 75° today. (#1)" {
		t.Errorf("calls %d, text %q; want the first briefing", s.briefer.calls, w.Text)
	}
}

func TestBriefingDailyCap(t *testing.T) {
	s := newBriefingSetup(withLocation)
	s.get(t, alice, nil)
	r, payload := s.remembered(t, alice.ID)
	payload["writes"] = 4
	r.Payload, _ = json.Marshal(payload)
	r.UpdatedAt = briefingNow.Add(-time.Hour)
	s.results.rows[resultKey(alice.ID, "briefing")] = r

	putToday(t, s.results, alice.ID, s.now, "Soccer")
	if _, w := s.get(t, alice, nil); s.briefer.calls != 1 || w.Text != "Sunny and 75° today. (#1)" {
		t.Errorf("calls %d, text %q; want no write past the cap", s.briefer.calls, w.Text)
	}
}

func TestBriefingNewDay(t *testing.T) {
	s := newBriefingSetup(withLocation)
	s.get(t, alice, nil)
	r, payload := s.remembered(t, alice.ID)
	payload["writes"] = 4
	payload["date"] = "2026-10-12"
	r.Payload, _ = json.Marshal(payload)
	s.results.rows[resultKey(alice.ID, "briefing")] = r

	s.now = briefingNow.Add(time.Minute)
	_, w := s.get(t, alice, nil)
	if s.briefer.calls != 2 || w.Text != "Sunny and 75° today. (#2)" {
		t.Fatalf("calls %d, text %q; want a new day's briefing", s.briefer.calls, w.Text)
	}
	if _, payload := s.remembered(t, alice.ID); payload["writes"] != 1.0 || payload["date"] != "2026-10-13" {
		t.Errorf("remembered %v, want writes 1 for today", payload)
	}
}

func TestBriefingKidSafe(t *testing.T) {
	s := newBriefingSetup(withLocation)
	_, w := s.get(t, kid, nil)
	if !w.KidSafe || s.briefer.auds[0] != sources.Kids {
		t.Errorf("kid: kidSafe %v, audience %v", w.KidSafe, s.briefer.auds[0])
	}

	// The parent's own briefing isn't kid-safe.
	_, pw := s.get(t, parent, nil)
	if pw.KidSafe || s.briefer.auds[1] != sources.Everyone {
		t.Errorf("parent: kidSafe %v, audience %v", pw.KidSafe, s.briefer.auds[1])
	}
}

func TestBriefingParentsBriefingNotSentToKid(t *testing.T) {
	s := newBriefingSetup(withLocation)
	// The member was a parent when the briefing was written...
	asParent := store.User{ID: kid.ID, Email: kid.Email, IsAdmin: true}
	s.get(t, asParent, nil)
	// ...and is a kid now.
	_, w := s.get(t, kid, nil)
	if s.briefer.calls != 2 || !w.KidSafe {
		t.Errorf("calls %d, kidSafe %v; want a new kid-safe briefing", s.briefer.calls, w.KidSafe)
	}
}

func TestBriefingParentViewsKid(t *testing.T) {
	s := newBriefingSetup(withLocation)
	_, own := s.get(t, kid, nil)
	k := kid
	_, viewed := s.get(t, parent, &k)
	if s.briefer.calls != 1 || viewed.Text != own.Text || !viewed.KidSafe {
		t.Errorf("calls %d, parent saw %+v; want the kid's remembered briefing", s.briefer.calls, viewed)
	}
}

func TestBriefingFailureKeepsEarlierOne(t *testing.T) {
	s := newBriefingSetup(withLocation)
	s.get(t, alice, nil)
	s.briefer.err = &llm.UpstreamError{Reason: "OpenRouter is having trouble", Status: http.StatusBadGateway, Err: errors.New("502")}
	s.now = briefingNow.Add(time.Hour)
	putToday(t, s.results, alice.ID, s.now, "Soccer")
	rec, w := s.get(t, alice, nil)
	if rec.Code != http.StatusOK || w.Text != "Sunny and 75° today. (#1)" {
		t.Errorf("got %d %q; want the earlier briefing", rec.Code, w.Text)
	}
}

func TestBriefingFailureWithNothingEarlier(t *testing.T) {
	s := newBriefingSetup(withLocation)
	s.briefer.err = fmt.Errorf("write briefing: %w", llm.ErrNoBriefing)
	rec, _ := s.get(t, alice, nil)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var body struct{ Error, Hint string }
	json.Unmarshal(rec.Body.Bytes(), &body)
	if !strings.Contains(body.Error, "briefing") || body.Hint == "" {
		t.Errorf("error body = %s", rec.Body)
	}
	if s.results.puts != 1 { // only the forecast was remembered
		t.Errorf("puts = %d, want 1 (the forecast)", s.results.puts)
	}
}

func TestBriefingSendsNothingPersonal(t *testing.T) {
	s := newBriefingSetup(fakeProfiles{profile: store.MemberProfile{Interests: "space", Location: "Denver, CO"}})
	putToday(t, s.results, alice.ID, s.now, "Soccer")
	s.get(t, alice, nil)
	facts, _ := json.Marshal(s.briefer.facts[0])
	for _, secret := range []string{alice.Email, "alice", "Denver", calSecret} {
		if strings.Contains(strings.ToLower(string(facts)), strings.ToLower(secret)) {
			t.Errorf("facts sent to the AI contain %q: %s", secret, facts)
		}
	}
}

func TestBriefingNeedsLogin(t *testing.T) {
	s := newBriefingSetup(withLocation)
	rec := httptest.NewRecorder()
	s.h.Briefing(rec, httptest.NewRequest(http.MethodGet, "/api/widgets/briefing", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestBriefingConcurrentRequestsWriteOnce(t *testing.T) {
	s := newBriefingSetup(withLocation)
	var wg sync.WaitGroup
	texts := make([]string, 5)
	for i := range texts {
		wg.Go(func() {
			_, w := s.get(t, alice, nil)
			texts[i] = w.Text
		})
	}
	wg.Wait()
	if n := s.briefer.callCount(); n != 1 {
		t.Errorf("AI calls = %d, want 1", n)
	}
	for _, text := range texts {
		if text != texts[0] {
			t.Errorf("requests saw different briefings: %q", texts)
			break
		}
	}
}

// putToday remembers a Today card for userID with one timed event this
// evening, fetched at now.
func putToday(t *testing.T, results *memResults, userID int64, now time.Time, title string) {
	t.Helper()
	loc, _ := time.LoadLocation(briefingTZ)
	start := time.Date(2026, 10, 13, 16, 0, 0, 0, loc)
	payload, _ := json.Marshal(map[string]any{
		"emptyReason": "", "provider": "Google Calendar", "date": "2026-10-13", "timeZone": briefingTZ,
		"today":    []widgets.TodayEvent{{Title: title, Start: start, End: start.Add(90 * time.Minute)}},
		"tomorrow": []widgets.TodayEvent{},
	})
	results.rows[resultKey(userID, "today")] = store.CuratedResult{UserID: userID, Kind: "today", Inputs: "x", Payload: payload, UpdatedAt: now}
}
