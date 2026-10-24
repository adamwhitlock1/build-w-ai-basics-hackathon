package widgets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
)

// Every error a widget service returns is answered with the status, message
// and hint the handlers sent before the services existed. The expected
// values are the handlers' messages, copied as they were.
func TestWriteServiceError(t *testing.T) {
	const (
		generic     = "Something went wrong on our side."
		genericHint = "Check the backend console for details."
		again       = "Try again in a moment."
	)
	noCredits := &llm.UpstreamError{Reason: "OpenRouter is out of credits", Hint: "Add credits at openrouter.ai.", Status: http.StatusPaymentRequired, Err: errors.New("402")}
	slowModel := &llm.UpstreamError{Reason: "the model took too long to answer", Hint: "Try describing the widget differently.", Err: context.DeadlineExceeded}
	weatherTimeout := fmt.Errorf("%w: %w", sources.ErrForecastUnavailable, context.DeadlineExceeded)

	tests := []struct {
		name       string
		err        error
		cancelled  bool // the request's context has ended
		wantStatus int
		wantError  string
		wantHint   string
		wantLog    string // a substring of the log; "" means nothing is logged
	}{
		// Errors the service logged itself.
		{name: "no such widget", err: fmt.Errorf("refresh widget 3: %w", ErrNoSuchWidget),
			wantStatus: 404, wantError: "That widget doesn't exist."},
		{name: "no such widget, request gone", err: ErrNoSuchWidget, cancelled: true,
			wantStatus: 404, wantError: "That widget doesn't exist."},
		{name: "unknown source", err: fmt.Errorf("%w %q", ErrUnknownSource, "tv_listings"),
			wantStatus: 422, wantError: "Couldn't turn that into a widget.", wantHint: "Couldn't pick a source for that prompt — try rephrasing"},
		{name: "source gone", err: ErrSourceGone,
			wantStatus: 422, wantError: "This widget's data source isn't available any more.", wantHint: "Remove it and describe it again."},
		{name: "not saved", err: fmt.Errorf("%w: %w", ErrNotSaved, errors.New("disk full")),
			wantStatus: 500, wantError: "Your widget was ready, but we couldn't save it.", wantHint: again},
		{name: "anything else", err: errors.New("load widget: database is locked"),
			wantStatus: 500, wantError: generic, wantHint: genericHint},

		// The request's own context ended: whatever the step, the same 503.
		{name: "cancelled planning", err: &StepError{Step: StepPlanning, Err: context.Canceled}, cancelled: true,
			wantStatus: 503, wantError: "The server stopped working on that request.", wantHint: again,
			wantLog: `level=INFO msg="request cancelled before the widget was ready" err="context canceled"`},
		{name: "cancelled search", err: &StepError{Step: StepSearch, Err: sources.ErrRateLimited}, cancelled: true,
			wantStatus: 503, wantError: "The server stopped working on that request.", wantHint: again,
			wantLog: `level=INFO msg="request cancelled before the widget was ready"`},
		{name: "cancelled briefing", err: stepError(StepBriefing, noCredits, "user_id", 8), cancelled: true,
			wantStatus: 503, wantError: "The server stopped working on that request.", wantHint: again,
			wantLog: `level=INFO msg="request cancelled before the widget was ready"`},

		// Describe-a-widget's plan.
		{name: "no valid spec", err: &StepError{Step: StepPlanning, Err: fmt.Errorf("pick spec: %w", llm.ErrInvalidSpec)},
			wantStatus: 422, wantError: "Couldn't turn that into a widget.", wantHint: "Couldn't pick a source for that prompt — try rephrasing",
			wantLog: `level=WARN msg="no valid widget spec"`},
		{name: "planning upstream", err: &StepError{Step: StepPlanning, Err: slowModel},
			wantStatus: 502, wantError: "The AI service had a problem: the model took too long to answer.", wantHint: "Try describing the widget differently.",
			wantLog: `level=ERROR msg="llm call failed"`},
		{name: "planning other", err: &StepError{Step: StepPlanning, Err: errors.New("boom")},
			wantStatus: 500, wantError: generic, wantHint: genericHint,
			wantLog: `level=ERROR msg="unexpected planner error" err=boom`},

		// Searches.
		{name: "place not found", err: &StepError{Step: StepSearch, Err: sources.ErrPlaceNotFound},
			wantStatus: 422, wantError: "Couldn't find that place for the forecast.", wantHint: "Name a town, like “weather in Denver, CO”, or add your location to your profile.",
			wantLog: `level=INFO msg="weather place not found"`},
		{name: "weather timeout", err: &StepError{Step: StepSearch, Err: weatherTimeout},
			wantStatus: 502, wantError: "The weather service (Open-Meteo) isn't answering right now.", wantHint: again,
			wantLog: `level=ERROR msg="weather forecast failed"`},
		{name: "rate limited", err: &StepError{Step: StepSearch, Err: errors.Join(sources.ErrRateLimited, errors.New("other query"))},
			wantStatus: 502, wantError: "Brave Search is busy right now.", wantHint: "Wait a few seconds and try again.",
			wantLog: `level=ERROR msg="search failed"`},
		{name: "search timeout", err: &StepError{Step: StepSearch, Err: context.DeadlineExceeded},
			wantStatus: 502, wantError: "Brave Search took too long to answer.", wantHint: again,
			wantLog: `level=ERROR msg="search failed"`},
		{name: "search other", err: &StepError{Step: StepSearch, Err: errors.New("brave: 500")},
			wantStatus: 502, wantError: "Couldn't fetch results from Brave Search.", wantHint: "Try again in a moment. If it keeps failing, check BRAVE_API_KEY in backend/.env.",
			wantLog: `level=ERROR msg="search failed"`},

		// The curated widgets' AI steps.
		{name: "event planning refused", err: &StepError{Step: StepEventPlanning, Err: noCredits},
			wantStatus: 502, wantError: "The AI service had a problem: OpenRouter is out of credits.", wantHint: "Add credits at openrouter.ai.",
			wantLog: `level=ERROR msg="event planning failed"`},
		{name: "event extraction slow", err: &StepError{Step: StepEventExtraction, Err: slowModel},
			wantStatus: 502, wantError: "The AI service had a problem: the model took too long to answer.", wantHint: "Try refreshing in a moment.",
			wantLog: `level=ERROR msg="event extraction failed"`},
		{name: "event extraction other", err: &StepError{Step: StepEventExtraction, Err: errors.New("no events")},
			wantStatus: 502, wantError: "Couldn't pick out events from the search results.", wantHint: again,
			wantLog: `level=ERROR msg="event extraction failed" err="no events"`},

		// The briefing.
		{name: "briefing refused", err: stepError(StepBriefing, noCredits, "user_id", 8),
			wantStatus: 502, wantError: "The AI service had a problem: OpenRouter is out of credits.", wantHint: "Add credits at openrouter.ai.",
			wantLog: `level=ERROR msg="briefing writing failed" user_id=8 err=`},
		{name: "briefing slow", err: stepError(StepBriefing, slowModel, "user_id", 8),
			wantStatus: 502, wantError: "Couldn't write your briefing right now.", wantHint: again,
			wantLog: `level=ERROR msg="briefing writing failed" user_id=8`},

		// The Today card's calendar.
		{name: "calendar", err: stepError(StepCalendar, errors.New("calendar unavailable"), "user_id", 8, "provider", "Google Calendar"),
			wantStatus: 502, wantError: "Couldn't load the calendar right now.", wantHint: again,
			wantLog: `level=WARN msg="couldn't load the calendar" user_id=8 provider="Google Calendar" err="calendar unavailable"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelled {
				cancel()
			}
			req := httptest.NewRequest(http.MethodGet, "/api/widgets/test", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			var logs bytes.Buffer
			writeServiceError(rec, req, slog.New(slog.NewTextHandler(&logs, nil)), tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body respond.ErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", rec.Body, err)
			}
			if body.Error != tt.wantError || body.Hint != tt.wantHint {
				t.Errorf("body = %+v, want error %q and hint %q", body, tt.wantError, tt.wantHint)
			}
			switch log := logs.String(); {
			case tt.wantLog == "" && log != "":
				t.Errorf("logged %q, want nothing (the service logs these)", log)
			case !strings.Contains(log, tt.wantLog):
				t.Errorf("log = %q, want it to contain %q", log, tt.wantLog)
			case strings.Count(log, "\n") > 1:
				t.Errorf("log = %q, want one line", log)
			}
		})
	}
}

// A StepError keeps its step's error reachable, so callers such as the
// agent can tell an OpenRouter refusal or a rate limit apart.
func TestStepErrorUnwraps(t *testing.T) {
	upstream := &llm.UpstreamError{Reason: "rate limited", Status: 429, Err: errors.New("429")}
	err := fmt.Errorf("events: %w", &StepError{Step: StepEventExtraction, Err: upstream})
	if got, ok := errors.AsType[*llm.UpstreamError](err); !ok || got != upstream {
		t.Errorf("errors.AsType found %v, want the upstream error", got)
	}
	step, ok := errors.AsType[*StepError](err)
	if !ok || step.Step != StepEventExtraction {
		t.Fatalf("errors.AsType found %v, want the step error", step)
	}
	if want := "event extraction: openrouter: rate limited: 429"; step.Error() != want {
		t.Errorf("Error() = %q, want %q", step.Error(), want)
	}
}
