package widgets_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/widgets"
)

// These tests call Briefings directly, with no HTTP request, as the agent
// will. The fakes and the clock are the briefing handler tests' (see
// briefing_test.go).

// briefingsRig is a Briefings with fake dependencies and a movable clock.
type briefingsRig struct {
	briefings *widgets.Briefings
	briefer   *fakeBriefer
	results   *memResults
	now       time.Time
}

func newBriefingsRig() *briefingsRig {
	rig := &briefingsRig{briefer: &fakeBriefer{text: "Sunny and 75° today."}, results: newMemResults(), now: briefingNow}
	rig.briefings = widgets.NewBriefings(rig.briefer, sources.NewRegistry(denverWeather()), withLocation, rig.results, discardLogger)
	widgets.SetBriefingsClock(rig.briefings, func() time.Time { return rig.now })
	return rig
}

// today asks for alice's briefing in Denver.
func (rig *briefingsRig) today(t *testing.T) (widgets.BriefingWidget, error) {
	t.Helper()
	denver, err := time.LoadLocation(briefingTZ)
	if err != nil {
		t.Fatal(err)
	}
	return rig.briefings.Today(context.Background(), alice, alice, denver)
}

func TestBriefingsFreshWrite(t *testing.T) {
	rig := newBriefingsRig()
	w, err := rig.today(t)
	if err != nil {
		t.Fatal(err)
	}
	// alice isn't a parent, so her briefing is kid-safe.
	if w.Text != "Sunny and 75° today. (#1)" || w.Date != "2026-10-13" || !w.KidSafe || w.EmptyReason != "" {
		t.Errorf("briefing = %+v", w)
	}
	if len(w.Uses) != 1 || w.Uses[0] != "weather" || !w.UpdatedAt.Equal(briefingNow) {
		t.Errorf("uses %v, updated %v; want [weather] at the clock's time", w.Uses, w.UpdatedAt)
	}
	if _, ok := rig.results.rows[resultKey(alice.ID, "briefing")]; !ok {
		t.Error("the briefing wasn't remembered")
	}
}

func TestBriefingsRememberedWhenFactsUnchanged(t *testing.T) {
	rig := newBriefingsRig()
	first, err := rig.today(t)
	if err != nil {
		t.Fatal(err)
	}
	rig.now = briefingNow.Add(30 * time.Minute)
	again, err := rig.today(t)
	if err != nil {
		t.Fatal(err)
	}
	if rig.briefer.calls != 1 || again.Text != first.Text || !again.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("calls %d, second briefing %+v; want the first one, unwritten", rig.briefer.calls, again)
	}
}

func TestBriefingsRewriteCap(t *testing.T) {
	rig := newBriefingsRig()
	if _, err := rig.today(t); err != nil {
		t.Fatal(err)
	}
	// The facts change every 11 minutes: rewritten until the daily cap of 4.
	var w widgets.BriefingWidget
	for i := 1; i <= 5; i++ {
		rig.now = briefingNow.Add(time.Duration(11*i) * time.Minute)
		putToday(t, rig.results, alice.ID, rig.now, fmt.Sprintf("Soccer %d", i))
		var err error
		if w, err = rig.today(t); err != nil {
			t.Fatal(err)
		}
	}
	if rig.briefer.calls != 4 || w.Text != "Sunny and 75° today. (#4)" {
		t.Errorf("calls %d, text %q; want 4 writes, then the fourth kept", rig.briefer.calls, w.Text)
	}
}

func TestBriefingsFailureIsAStepError(t *testing.T) {
	rig := newBriefingsRig()
	rig.briefer.err = fmt.Errorf("write briefing: %w", llm.ErrNoBriefing)
	_, err := rig.today(t)
	step, ok := errors.AsType[*widgets.StepError](err)
	if !ok || step.Step != widgets.StepBriefing || !errors.Is(err, llm.ErrNoBriefing) {
		t.Errorf("err = %v, want a briefing StepError wrapping ErrNoBriefing", err)
	}
}
