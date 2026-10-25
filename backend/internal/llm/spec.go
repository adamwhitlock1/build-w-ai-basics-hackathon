package llm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"kinlight/internal/sources"
)

// Archetype names a way to display a widget's items.
type Archetype string

// The display archetypes the frontend knows how to render.
const (
	ArchetypeList     Archetype = "list"
	ArchetypeCards    Archetype = "cards"
	ArchetypeHero     Archetype = "hero"
	ArchetypeKeyValue Archetype = "key_value"
	ArchetypeTable    Archetype = "table"
)

// Archetypes lists every valid Archetype, in the order shown to the model.
var Archetypes = []Archetype{ArchetypeList, ArchetypeCards, ArchetypeHero, ArchetypeKeyValue, ArchetypeTable}

// Valid reports whether a is one of Archetypes.
func (a Archetype) Valid() bool { return slices.Contains(Archetypes, a) }

// WidgetSpec is the model's plan for a widget: which source to search, with
// what query, and how to display the results.
//
// The struct doubles as the JSON Schema sent to the model, so every field is
// required (no omitempty) and the description tags become schema descriptions.
type WidgetSpec struct {
	Title     string    `json:"title" description:"Short widget title in Title Case, 2-5 words, e.g. SpaceX Launches"`
	Source    string    `json:"source" description:"Name of the data source to search"`
	Query     string    `json:"query" description:"Concise search query for that source, 2-8 words, no quotes or operators"`
	Archetype Archetype `json:"archetype" description:"How to display the results"`
}

// Catalog is what PickWidgetSpec needs to know about the available data
// sources. *sources.Registry satisfies it without mentioning it: Go
// interfaces are satisfied implicitly, so the consumer defines what it needs.
type Catalog interface {
	Names() []string
	Describe() string
}

// PickWidgetSpec asks the model to turn prompt into a WidgetSpec using one of
// the sources in catalog. location is the member's saved location ("" if
// none); the model uses it when the prompt needs a place but names none. If
// the model's answer is invalid even after one retry, the error wraps
// ErrInvalidSpec. Failures talking to OpenRouter are returned as
// *UpstreamError.
func (c *Client) PickWidgetSpec(ctx context.Context, prompt string, catalog Catalog, aud sources.Audience, location string) (WidgetSpec, error) {
	names := catalog.Names()
	schema, err := specSchema(names)
	if err != nil {
		return WidgetSpec{}, err
	}

	call := structuredCall{
		name:   "widget_spec",
		schema: schema,
		messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: ForAudience(specSystemPrompt(catalog.Describe(), location, time.Now()), aud)},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	}
	spec, err := callJSON(ctx, c, call, func(s *WidgetSpec) error {
		return s.validate(names)
	})
	if err != nil {
		return WidgetSpec{}, fmt.Errorf("pick widget spec: %w", err)
	}
	return spec, nil
}

// validate trims s in place and checks it against the known source names.
func (s *WidgetSpec) validate(sourceNames []string) error {
	s.Title = strings.TrimSpace(s.Title)
	s.Source = strings.TrimSpace(s.Source)
	s.Query = strings.TrimSpace(s.Query)
	s.Archetype = Archetype(strings.TrimSpace(string(s.Archetype)))

	var problems []error
	if !slices.Contains(sourceNames, s.Source) {
		problems = append(problems, fmt.Errorf("unknown source %q (want one of %s)", s.Source, strings.Join(sourceNames, ", ")))
	}
	if !s.Archetype.Valid() {
		problems = append(problems, fmt.Errorf("unknown archetype %q", s.Archetype))
	}
	if s.Title == "" {
		problems = append(problems, errors.New("title is empty"))
	}
	if s.Query == "" {
		problems = append(problems, errors.New("query is empty"))
	}
	if len(problems) == 0 {
		return nil
	}
	// %w with errors.Join keeps every problem in the message and lets callers
	// still match ErrInvalidSpec with errors.Is.
	return fmt.Errorf("%w: %w", ErrInvalidSpec, errors.Join(problems...))
}

// specSchema builds the JSON Schema for WidgetSpec, with the source and
// archetype fields restricted to the values we actually support.
func specSchema(sourceNames []string) (*jsonschema.Definition, error) {
	schema, err := jsonschema.GenerateSchemaForType(WidgetSpec{})
	if err != nil {
		return nil, fmt.Errorf("generate widget spec schema: %w", err)
	}
	archetypes := make([]string, len(Archetypes))
	for i, a := range Archetypes {
		archetypes[i] = string(a)
	}
	setEnum(schema, "source", sourceNames)
	setEnum(schema, "archetype", archetypes)
	return schema, nil
}

// setEnum restricts one property of schema to values. Properties stores
// Definitions by value, so we copy one out, change it, and store it back.
func setEnum(schema *jsonschema.Definition, property string, values []string) {
	p := schema.Properties[property]
	p.Enum = values
	schema.Properties[property] = p
}

// specSystemPrompt explains the job, the sources, and the archetypes. It
// includes today's date so the model can make "upcoming" queries concrete,
// and the member's saved location, when there is one, so "weather" or "near
// me" can name a place.
func specSystemPrompt(sourceList, location string, now time.Time) string {
	return `You design small widgets for Porchlight, a family homepage. A family member describes a widget in one sentence; you decide where its data comes from and how it looks.

Today is ` + now.Format("Monday, January 2, 2006") + `.

Pick exactly one data source:
` + sourceList + `
Pick exactly one display archetype:
- list: a vertical list; each row shows a title (linked), a date, and a snippet. Good for headlines, launches, schedules.
- cards: a grid of small cards with an optional image, a title, and a snippet. Good for products, places, recipes, articles with pictures.
- hero: one featured item in big text with a snippet below and an attribution label. Good for a quote, word, or fact of the day.
- key_value: rows of label -> value. Good for stats, scores, prices, measurements.
- table: columns of title, date, label, and value. Good for comparing several things across a few attributes.

Write the query the way a person would type it into a search engine. Add the current year when timeliness matters (e.g. "upcoming SpaceX launches ` + strconv.Itoa(now.Year()) + `").
Give the widget a short Title Case title of 2-5 words.` + locationRule(location) + `
Reply only with JSON that matches the schema.`
}

// locationRule tells the model where the member lives, or returns "" when
// they haven't said. The location is quoted with %q so stray quotes or line
// breaks in it can't change the instructions around it.
func locationRule(location string) string {
	if location == "" {
		return ""
	}
	return fmt.Sprintf("\nThe member's saved location is %q. When the request needs a place and names none (\"weather\", \"near me\"), use this location in the query. A place named in the request wins.", location)
}
