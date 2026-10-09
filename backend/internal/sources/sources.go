// Package sources defines the data sources a widget can pull from and a
// Registry the LLM chooses among. Each Source hides one search backend behind
// the same small interface, so adding a provider later is a matter of writing
// one type and registering it.
package sources

import (
	"context"
	"fmt"
	"strings"
)

// Source is one searchable data source, such as Brave's news vertical.
type Source interface {
	// Name is the stable identifier the LLM uses to pick this source,
	// e.g. "news_search".
	Name() string
	// Search runs query against the source and returns normalized results.
	Search(ctx context.Context, query string) ([]Result, error)
	// Shape is a one-line hint for the LLM describing what this source
	// returns and what it is good for.
	Shape() string
}

// Result is one search hit, normalized across providers. Empty strings mean
// the provider did not supply that field.
type Result struct {
	Title         string
	URL           string
	Snippet       string
	Date          string // human-readable, e.g. "2 days ago" or "2025-05-01"
	Thumbnail     string // image URL
	ExtraSnippets []string
}

// Registry holds the available sources, keyed by name.
type Registry struct {
	byName map[string]Source
	names  []string // registration order, so prompts and listings are stable
}

// NewRegistry returns a Registry containing srcs. It panics if two sources
// share a name, since that is a programming error caught at startup.
func NewRegistry(srcs ...Source) *Registry {
	r := &Registry{byName: make(map[string]Source, len(srcs))}
	for _, s := range srcs {
		if _, dup := r.byName[s.Name()]; dup {
			panic(fmt.Sprintf("sources: duplicate source name %q", s.Name()))
		}
		r.byName[s.Name()] = s
		r.names = append(r.names, s.Name())
	}
	return r
}

// Get returns the source registered under name. The two-value "comma ok"
// form lets callers tell "not found" apart from a zero value.
func (r *Registry) Get(name string) (Source, bool) {
	s, ok := r.byName[name]
	return s, ok
}

// Names returns the registered source names in registration order.
func (r *Registry) Names() []string {
	// Return a copy so callers can't mutate the registry's slice.
	return append([]string(nil), r.names...)
}

// Describe returns a bulleted list of sources and their shape hints, ready to
// paste into an LLM prompt.
func (r *Registry) Describe() string {
	var b strings.Builder
	for _, name := range r.names {
		fmt.Fprintf(&b, "- %s: %s\n", name, r.byName[name].Shape())
	}
	return b.String()
}
