package aikit

import (
	"context"
)

// Tool names, as the model and the tool notes see them.
const (
	ToolWeather = "weather"
	ToolMyDay   = "my_day"
	ToolMyPicks = "my_picks"
	ToolSearch  = "search"
)

// MaxSearches is how many searches one kid message may make.
const MaxSearches = 2

// KidTools is what the kid agent can look up while replying to one
// message, for the kid who sent it. Each function already knows which kid
// (the chat service builds them per message), so no tool takes a member
// from the model. A nil function is a tool not offered.
type KidTools struct {
	// Weather returns the forecast for the kid's saved location, or a
	// sentence saying there's no location.
	Weather func(ctx context.Context) (string, error)
	// MyDay returns the kid's calendar events for today and tomorrow.
	MyDay func(ctx context.Context) (string, error)
	// MyPicks returns the kid's interests and their page's remembered
	// articles and events.
	MyPicks func(ctx context.Context) (string, error)
	// Search runs a kid-safe web search. nil in Homework help.
	Search func(ctx context.Context, query string) ([]SearchResult, error)
	// OnTool is told each time a tool is used, for the notes kids and
	// parents see. It may be nil.
	OnTool func(ToolNote)
}

// offered lists the tools t offers, in a fixed order.
func (t *KidTools) offered() []string {
	if t == nil {
		return nil
	}
	return offeredTools(
		ToolWeather, t.Weather != nil,
		ToolMyDay, t.MyDay != nil,
		ToolMyPicks, t.MyPicks != nil,
		ToolSearch, t.Search != nil,
	)
}

// build returns the Genkit tools for one kid reply.
func (t *KidTools) build(run *toolRun) []toolRef {
	if t == nil {
		return nil
	}
	b := toolBuilder{run: run, onTool: t.OnTool}
	var tools []toolRef
	if t.Weather != nil {
		tools = append(tools, b.text(ToolWeather, "Gets the weather forecast for today and the next few days where the kid lives.",
			"weather forecast", "forecast", t.Weather, "The weather isn't available right now."))
	}
	if t.MyDay != nil {
		tools = append(tools, b.text(ToolMyDay, "Gets the kid's own calendar events for today and tomorrow.",
			"the kid's calendar", "events", t.MyDay, "The calendar isn't available right now."))
	}
	if t.MyPicks != nil {
		tools = append(tools, b.text(ToolMyPicks, "Gets the kid's interests and the articles and local events already picked for their page.",
			"the kid's page", "picks", t.MyPicks, "Their picks aren't available right now."))
	}
	if t.Search != nil {
		tools = append(tools, b.search(
			"Searches the web, with strict safe-search, for current facts or news. Never use it to find the answer to an assignment.",
			MaxSearches, false, t.Search))
	}
	return tools
}
