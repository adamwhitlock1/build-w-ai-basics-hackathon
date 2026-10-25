package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"porchlight/internal/aikit"
	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// Tools looks things up for one kid, for the agent's tools. Every method
// takes the kid whose data it reads: Send passes the kid who's chatting,
// and nothing the model says can change that.
type Tools interface {
	Weather(ctx context.Context, kid store.User) (string, error)
	MyDay(ctx context.Context, kid store.User, loc *time.Location) (string, error)
	MyPicks(ctx context.Context, kid store.User) (string, error)
	Search(ctx context.Context, kid store.User, query string) ([]aikit.SearchResult, error)
}

// ToolSources is Tools on Porchlight's own data and sources: the kid's
// profile, the Today card's calendar, their page's remembered picks, the
// forecast source, and kid-safe web search.
type ToolSources struct {
	Profiles interface {
		GetMemberProfile(ctx context.Context, userID int64) (store.MemberProfile, error)
	}
	Agenda   *widgets.Agenda
	Results  widgets.ResultStore
	Forecast sources.Source // the weather_forecast source
	Web      sources.Source // web search; searched with sources.Kids
}

// maxPicks is how many articles and events my_picks hands the model.
const maxPicks = 5

func (t *ToolSources) profile(ctx context.Context, kid store.User) (store.MemberProfile, error) {
	p, err := t.Profiles.GetMemberProfile(ctx, kid.ID)
	if errors.Is(err, store.ErrNotFound) {
		return store.MemberProfile{UserID: kid.ID}, nil
	}
	return p, err
}

// Weather returns the forecast for kid's saved location, one line per day.
func (t *ToolSources) Weather(ctx context.Context, kid store.User) (string, error) {
	p, err := t.profile(ctx, kid)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(p.Location) == "" {
		return "They haven't saved a location, so there's no forecast. You could ask where they are.", nil
	}
	results, err := t.Forecast.Search(ctx, p.Location, sources.Kids)
	if err != nil {
		return "", err
	}
	var lines []string
	for _, r := range results {
		lines = append(lines, r.Title+": "+r.Snippet)
	}
	if len(lines) == 0 {
		return "No forecast came back for " + p.Location + ".", nil
	}
	return "Forecast for " + p.Location + ":\n" + strings.Join(lines, "\n"), nil
}

// MyDay returns kid's calendar events for today and tomorrow in loc, as
// their Today card has them.
func (t *ToolSources) MyDay(ctx context.Context, kid store.User, loc *time.Location) (string, error) {
	day, err := t.Agenda.Day(ctx, kid, loc, widgets.DayOptions{})
	if err != nil {
		return "", err
	}
	switch day.EmptyReason {
	case "":
	case widgets.EmptyNoCalendar:
		return "They haven't connected a calendar.", nil
	default:
		return "Their calendar isn't available right now.", nil
	}
	now := time.Now().In(loc)
	var b strings.Builder
	fmt.Fprintf(&b, "It's %s now.\n", now.Format("Monday 3:04 PM"))
	writeDay(&b, "Today", day.Today, day.MoreToday)
	writeDay(&b, "Tomorrow", day.Tomorrow, day.MoreTomorrow)
	return strings.TrimRight(b.String(), "\n"), nil
}

func writeDay(b *strings.Builder, label string, events []widgets.TodayEvent, more int) {
	if len(events) == 0 {
		fmt.Fprintf(b, "%s: nothing on the calendar.\n", label)
		return
	}
	fmt.Fprintf(b, "%s:\n", label)
	for _, e := range events {
		when := "All day"
		if !e.AllDay {
			when = e.Start.Format("3:04 PM") + " to " + e.End.Format("3:04 PM")
		}
		line := "- " + when + ": " + e.Title
		if e.Location != "" {
			line += " (" + e.Location + ")"
		}
		b.WriteString(line + "\n")
	}
	if more > 0 {
		fmt.Fprintf(b, "- and %d more\n", more)
	}
}

// MyPicks returns kid's interests and their page's remembered articles and
// local events. It never searches.
func (t *ToolSources) MyPicks(ctx context.Context, kid store.User) (string, error) {
	p, err := t.profile(ctx, kid)
	if err != nil {
		return "", err
	}
	articles, events, err := widgets.RememberedPicks(ctx, t.Results, kid, p)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if p.Interests != "" {
		b.WriteString("Interests: " + p.Interests + "\n")
	} else {
		b.WriteString("They haven't saved any interests.\n")
	}
	writeItems(&b, "Articles on their page", articles)
	writeItems(&b, "Events near them", events)
	return strings.TrimRight(b.String(), "\n"), nil
}

func writeItems(b *strings.Builder, label string, items []llm.Item) {
	if len(items) == 0 {
		return
	}
	b.WriteString(label + ":\n")
	for i, it := range items {
		if i == maxPicks {
			break
		}
		line := "- " + it.Title
		if it.Date != "" {
			line += " (" + it.Date + ")"
		}
		if it.Label != "" {
			line += ", " + it.Label
		}
		b.WriteString(line + "\n")
	}
}

// Search runs a web search with kids' strict safe-search.
func (t *ToolSources) Search(ctx context.Context, _ store.User, query string) ([]aikit.SearchResult, error) {
	results, err := t.Web.Search(ctx, query, sources.Kids)
	if err != nil {
		return nil, err
	}
	out := make([]aikit.SearchResult, len(results))
	for i, r := range results {
		out[i] = aikit.SearchResult{Title: r.Title, Site: r.Site, Date: r.Date, Snippet: r.Snippet}
	}
	return out, nil
}
