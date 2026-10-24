package widgets

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
)

// The errors the widget services return when there's no widget to give
// back. The services log them where they happen, with whose widget it was;
// writeServiceError only turns them into a response.
var (
	// ErrNoSuchWidget means the member has no saved widget with that id:
	// it never existed, it's someone else's, or it was deleted.
	ErrNoSuchWidget = errors.New("no such widget")
	// ErrUnknownSource means the planner chose a source that isn't
	// registered.
	ErrUnknownSource = errors.New("the planner chose an unknown source")
	// ErrSourceGone means a saved widget's source isn't registered any
	// more, so it can't be refreshed.
	ErrSourceGone = errors.New("the widget's source isn't available any more")
	// ErrNotSaved means a widget was built, but saving it failed.
	ErrNotSaved = errors.New("the widget was built but couldn't be saved")
)

// The steps a StepError can name. Each is a call to something outside
// Porchlight: an AI model, a search, or a calendar.
const (
	StepPlanning        = "planning" // describe-a-widget choosing a source and query
	StepSearch          = "search"
	StepArticlePlanning = "article planning"
	StepEventPlanning   = "event planning"
	StepEventExtraction = "event extraction"
	StepBriefing        = "briefing writing"
	StepCalendar        = "calendar"
)

// StepError is a failed step of a widget service, such as a search or an AI
// call. Err is what the step returned: a *llm.UpstreamError, a search
// error, or the context's error when the caller has gone.
type StepError struct {
	Step string // one of the Step constants
	Err  error
	// logAttrs go on the failure's log line, before "err": whose widget it
	// was, for instance. They are never shown to anyone.
	logAttrs []any
}

func (e *StepError) Error() string { return e.Step + ": " + e.Err.Error() }

// Unwrap lets errors.Is and errors.As see the step's own error, such as a
// *llm.UpstreamError or sources.ErrRateLimited.
func (e *StepError) Unwrap() error { return e.Err }

// stepError returns a StepError whose log line carries logAttrs, given as
// key-value pairs as for slog.
func stepError(step string, err error, logAttrs ...any) *StepError {
	return &StepError{Step: step, Err: err, logAttrs: logAttrs}
}

// writeServiceError answers a request whose widget service returned err. It
// writes the statuses, messages and hints the handlers have always sent, so
// moving the work into services changed nothing the page sees.
//
// A failed step is logged here, as the handlers always did; every other
// error was logged by the service where it happened.
func writeServiceError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	// errors.AsType (Go 1.26) is errors.As with the target type as a type
	// argument: it returns the *StepError in err's chain, if there is one.
	if step, ok := errors.AsType[*StepError](err); ok {
		writeStepError(w, r, logger, step)
		return
	}
	switch {
	case errors.Is(err, ErrNoSuchWidget):
		writeWidgetNotFound(w)
	case errors.Is(err, ErrUnknownSource):
		respond.Error(w, http.StatusUnprocessableEntity, "Couldn't turn that into a widget.", "Couldn't pick a source for that prompt — try rephrasing")
	case errors.Is(err, ErrSourceGone):
		respond.Error(w, http.StatusUnprocessableEntity, "This widget's data source isn't available any more.", "Remove it and describe it again.")
	case errors.Is(err, ErrNotSaved):
		respond.Error(w, http.StatusInternalServerError, "Your widget was ready, but we couldn't save it.", "Try again in a moment.")
	default:
		writeInternalError(w)
	}
}

// writeStepError logs a failed step and answers it. When the request itself
// has ended, the step failed because of that, and is answered as such.
func writeStepError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, e *StepError) {
	if writeIfCancelled(w, r, logger, e.Err) {
		return
	}
	switch e.Step {
	case StepSearch:
		writeSearchError(w, r, logger, e.Err)
	case StepPlanning:
		writePlanningError(w, r, logger, e.Err)
	case StepBriefing:
		logger.ErrorContext(r.Context(), e.Step+" failed", append(e.logAttrs, "err", e.Err)...)
		if upstream, ok := errors.AsType[*llm.UpstreamError](e.Err); ok && upstream.Status != 0 {
			// OpenRouter itself said no (a bad key, no credits, a rate
			// limit): its reason and hint are what to fix.
			respond.Error(w, http.StatusBadGateway, "The AI service had a problem: "+upstream.Reason+".", upstream.Hint)
			return
		}
		respond.Error(w, http.StatusBadGateway, "Couldn't write your briefing right now.", "Try again in a moment.")
	case StepCalendar:
		// The calendar package keeps the link out of its errors, so err is
		// safe to log.
		logger.WarnContext(r.Context(), "couldn't load the calendar", append(e.logAttrs, "err", e.Err)...)
		respond.Error(w, http.StatusBadGateway, "Couldn't load the calendar right now.", "Try again in a moment.")
	case StepArticlePlanning, StepEventPlanning, StepEventExtraction:
		writeCuratedAIError(w, r, logger, e)
	default:
		logger.ErrorContext(r.Context(), e.Step+" failed", append(e.logAttrs, "err", e.Err)...)
		writeInternalError(w)
	}
}

// writeIfCancelled handles a request whose own context has ended because the
// client went away or the server is shutting down. Neither is an upstream
// failure, so it logs at Info rather than Error. It reports whether it wrote
// a response.
func writeIfCancelled(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) bool {
	if r.Context().Err() == nil {
		return false
	}
	logger.InfoContext(r.Context(), "request cancelled before the widget was ready", "err", err)
	respond.Error(w, http.StatusServiceUnavailable, "The server stopped working on that request.", "Try again in a moment.")
	return true
}

// writePlanningError answers a describe-a-widget plan that failed.
func writePlanningError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var upstream *llm.UpstreamError
	switch {
	case errors.Is(err, llm.ErrInvalidSpec):
		logger.WarnContext(r.Context(), "no valid widget spec", "err", err)
		respond.Error(w, http.StatusUnprocessableEntity, "Couldn't turn that into a widget.", "Couldn't pick a source for that prompt — try rephrasing")
	case errors.As(err, &upstream):
		logger.ErrorContext(r.Context(), "llm call failed", "err", err)
		respond.Error(w, http.StatusBadGateway, "The AI service had a problem: "+upstream.Reason+".", upstream.Hint)
	default:
		logger.ErrorContext(r.Context(), "unexpected planner error", "err", err)
		writeInternalError(w)
	}
}

// writeSearchError answers a failed search, from Brave or the weather
// source.
func writeSearchError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	switch {
	// The weather errors come first: a weather timeout wraps both
	// ErrForecastUnavailable and context.DeadlineExceeded, and must not read
	// as a Brave one.
	case errors.Is(err, sources.ErrPlaceNotFound):
		logger.InfoContext(r.Context(), "weather place not found", "err", err)
		respond.Error(w, http.StatusUnprocessableEntity, "Couldn't find that place for the forecast.",
			"Name a town, like “weather in Denver, CO”, or add your location to your profile.")
		return
	case errors.Is(err, sources.ErrForecastUnavailable):
		logger.ErrorContext(r.Context(), "weather forecast failed", "err", err)
		respond.Error(w, http.StatusBadGateway, "The weather service (Open-Meteo) isn't answering right now.", "Try again in a moment.")
		return
	}
	logger.ErrorContext(r.Context(), "search failed", "err", err)
	switch {
	case errors.Is(err, sources.ErrRateLimited):
		respond.Error(w, http.StatusBadGateway, "Brave Search is busy right now.", "Wait a few seconds and try again.")
	case errors.Is(err, context.DeadlineExceeded):
		respond.Error(w, http.StatusBadGateway, "Brave Search took too long to answer.", "Try again in a moment.")
	default:
		respond.Error(w, http.StatusBadGateway, "Couldn't fetch results from Brave Search.",
			"Try again in a moment. If it keeps failing, check BRAVE_API_KEY in backend/.env.")
	}
}

// writeCuratedAIError answers a failed AI step of the curated widgets, such
// as event extraction.
func writeCuratedAIError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, e *StepError) {
	logger.ErrorContext(r.Context(), e.Step+" failed", append(e.logAttrs, "err", e.Err)...)
	if upstream, ok := errors.AsType[*llm.UpstreamError](e.Err); ok {
		// Hints about OpenRouter itself (its key, credits, rate limit) fit
		// here; others, such as "Try describing the widget differently",
		// were written for describe-a-widget, and the member described
		// nothing.
		hint := "Try refreshing in a moment."
		if upstream.Status != 0 {
			hint = upstream.Hint
		}
		respond.Error(w, http.StatusBadGateway, "The AI service had a problem: "+upstream.Reason+".", hint)
		return
	}
	respond.Error(w, http.StatusBadGateway, "Couldn't pick out events from the search results.", "Try again in a moment.")
}

func writeWidgetNotFound(w http.ResponseWriter) {
	respond.Error(w, http.StatusNotFound, "That widget doesn't exist.", "")
}

// writeInternalError writes the generic 500 for a failure on our side.
func writeInternalError(w http.ResponseWriter) {
	respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
}
