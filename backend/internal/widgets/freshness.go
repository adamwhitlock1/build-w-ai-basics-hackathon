package widgets

import (
	"time"

	"kinlight/internal/sources"
)

// How long each kind of widget's results stay fresh. Older results are
// still shown, but marked stale, and the page then asks for new ones in the
// background. News moves faster than local events; described widgets sit in
// between, except weather, which changes by the hour.
const (
	ArticlesFreshFor = 3 * time.Hour
	EventsFreshFor   = 12 * time.Hour
	CustomFreshFor   = 6 * time.Hour
	WeatherFreshFor  = time.Hour
)

// customFreshFor is how long a described widget's items stay fresh, by the
// source they came from: a forecast changes faster than most searches.
func customFreshFor(source string) time.Duration {
	if source == sources.WeatherForecast {
		return WeatherFreshFor
	}
	return CustomFreshFor
}

// refreshGuard is how recent a result must be for a refresh to hand it back
// instead of searching again. It stops a double click, or the same page open
// in two tabs, from paying for the same refresh twice.
const refreshGuard = time.Minute

// Freshness says when a widget's items were last updated, and whether
// they're old enough that the page should fetch new ones. It's embedded in
// the widget responses, so its fields sit at the top level of their JSON.
type Freshness struct {
	UpdatedAt time.Time `json:"updatedAt"` // encodes as RFC 3339
	Stale     bool      `json:"stale"`
}

// freshness reports results updated at updatedAt, as of now, for a widget
// kind that stays fresh for window. They turn stale once they are older
// than window.
func freshness(updatedAt, now time.Time, window time.Duration) Freshness {
	return Freshness{UpdatedAt: updatedAt.UTC(), Stale: now.Sub(updatedAt) > window}
}

// justUpdated is the Freshness of results worked out right now.
func justUpdated(now time.Time) Freshness {
	return Freshness{UpdatedAt: now.UTC()}
}

// recentlyUpdated reports whether results updated at updatedAt are within
// refreshGuard of now, so a refresh should return them rather than search.
func recentlyUpdated(updatedAt, now time.Time) bool {
	return now.Sub(updatedAt) < refreshGuard
}

// timeNow returns the current time to the second. The database keeps times
// to the second, so a time from here reads back unchanged. It stays in local
// time, which the Local Events widget needs to work out what "today" is.
func timeNow() time.Time {
	return time.Now().Truncate(time.Second)
}
