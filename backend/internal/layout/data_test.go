package layout

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// summary describes d's cards as short strings like "Articles articles 2"
// or "Custom custom-7 4 #7" (type, id, span, and widget id), so tests can
// compare whole layouts at a glance.
func summary(d Data) []string {
	out := make([]string, 0, len(d.Root.Props.Blocks))
	for _, b := range d.Root.Props.Blocks {
		s := fmt.Sprintf("%s %s %d", b.Type, b.Props.ID, b.Props.Span)
		if b.Props.WidgetID != 0 {
			s += fmt.Sprintf(" #%d", b.Props.WidgetID)
		}
		out = append(out, s)
	}
	return out
}

// layoutOf builds a Data value from blocks, for test input.
func layoutOf(blocks ...Block) Data {
	return newData(blocks)
}

func articles(id string, span int) Block {
	return Block{Type: TypeArticles, Props: BlockProps{ID: id, Span: span}}
}

func events(id string, span int) Block {
	return Block{Type: TypeLocalEvents, Props: BlockProps{ID: id, Span: span}}
}

func custom(widgetID int64, id string, span int) Block {
	return Block{Type: TypeCustom, Props: BlockProps{ID: id, Span: span, WidgetID: widgetID}}
}

func TestDefault(t *testing.T) {
	// The exact JSON matters: Puck needs "content" and "zones" present and
	// not null, and the curated cards carry no widgetId.
	got, err := json.Marshal(Default(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"root":{"props":{"blocks":[` +
		`{"type":"Articles","props":{"id":"articles","span":2}},` +
		`{"type":"LocalEvents","props":{"id":"events","span":2}}` +
		`]}},"content":[],"zones":{}}`
	if string(got) != want {
		t.Errorf("Default(nil) =\n%s\nwant\n%s", got, want)
	}

	// Saved widgets follow, half width, in the order given (oldest first).
	wantCards := []string{"Articles articles 2", "LocalEvents events 2", "Custom custom-3 2 #3", "Custom custom-1 2 #1"}
	if got := summary(Default([]int64{3, 1})); !slices.Equal(got, wantCards) {
		t.Errorf("Default([3 1]) = %q, want %q", got, wantCards)
	}
}

func TestHeal(t *testing.T) {
	tests := []struct {
		name    string
		saved   Data
		widgets []int64
		want    []string
	}{
		{
			name:    "a layout that matches is unchanged",
			saved:   layoutOf(custom(5, "custom-5", 4), events("events", 1), articles("articles", 3)),
			widgets: []int64{5},
			want:    []string{"Custom custom-5 4 #5", "LocalEvents events 1", "Articles articles 3"},
		},
		{
			name:    "a deleted widget's card is dropped",
			saved:   layoutOf(articles("articles", 2), custom(5, "custom-5", 2), custom(6, "custom-6", 3)),
			widgets: []int64{6},
			want:    []string{"Articles articles 2", "Custom custom-6 3 #6"},
		},
		{
			name:    "a widget with no card gets one at the end",
			saved:   layoutOf(custom(5, "custom-5", 4), articles("articles", 2)),
			widgets: []int64{5, 8, 9},
			want:    []string{"Custom custom-5 4 #5", "Articles articles 2", "Custom custom-8 2 #8", "Custom custom-9 2 #9"},
		},
		{
			name:    "a widget keeps only its first card",
			saved:   layoutOf(custom(5, "a", 1), articles("articles", 2), custom(5, "b", 3), custom(6, "c", 4), custom(5, "d", 2)),
			widgets: []int64{5, 6},
			want:    []string{"Custom a 1 #5", "Articles articles 2", "Custom c 4 #6"},
		},
		{
			name:    "a removed curated card is not put back",
			saved:   layoutOf(events("events", 4)),
			widgets: nil,
			want:    []string{"LocalEvents events 4"},
		},
		{
			name:    "curated cards appear at most once",
			saved:   layoutOf(articles("a1", 1), events("e1", 2), articles("a2", 3), events("e2", 4)),
			widgets: nil,
			want:    []string{"Articles a1 1", "LocalEvents e1 2"},
		},
		{
			name: "unknown cards are dropped; bad widths and ids are fixed",
			saved: layoutOf(
				Block{Type: "Weather", Props: BlockProps{ID: "w", Span: 2}},
				articles("", 0),
				events("x", 9),
				custom(5, "x", 2),
			),
			widgets: []int64{5},
			want:    []string{"Articles articles 2", "LocalEvents x 2", "Custom x-2 2 #5"},
		},
		{
			name:    "an appended card never reuses an id",
			saved:   layoutOf(articles("custom-5", 2)),
			widgets: []int64{5},
			want:    []string{"Articles custom-5 2", "Custom custom-5-2 2 #5"},
		},
		{
			name:    "an empty page stays empty",
			saved:   Data{}, // even with nil slices, as from "{}"
			widgets: nil,
			want:    []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healed := heal(tt.saved, tt.widgets)
			if got := summary(healed); !slices.Equal(got, tt.want) {
				t.Errorf("heal = %q, want %q", got, tt.want)
			}
			if healed.Root.Props.Blocks == nil || healed.Content == nil || healed.Zones == nil {
				t.Errorf("heal left a nil slice or map, which encodes as null: %+v", healed)
			}
		})
	}
}

func TestWithCustomWidget(t *testing.T) {
	saved := layoutOf(articles("articles", 2), custom(5, "custom-5", 4))

	added := withCustomWidget(saved, 6)
	want := []string{"Articles articles 2", "Custom custom-5 4 #5", "Custom custom-6 2 #6"}
	if got := summary(added); !slices.Equal(got, want) {
		t.Errorf("withCustomWidget(6) = %q, want %q", got, want)
	}
	if len(saved.Root.Props.Blocks) != 2 {
		t.Errorf("withCustomWidget changed its input: %q", summary(saved))
	}

	// A widget already on the page isn't added twice.
	if got := summary(withCustomWidget(added, 5)); !slices.Equal(got, want) {
		t.Errorf("withCustomWidget(5) = %q, want it unchanged: %q", got, want)
	}
}

func TestWithoutCustomWidget(t *testing.T) {
	saved := layoutOf(articles("articles", 2), custom(5, "custom-5", 4), custom(6, "custom-6", 1))

	removed := withoutCustomWidget(saved, 5)
	want := []string{"Articles articles 2", "Custom custom-6 1 #6"}
	if got := summary(removed); !slices.Equal(got, want) {
		t.Errorf("withoutCustomWidget(5) = %q, want %q", got, want)
	}
	if len(saved.Root.Props.Blocks) != 3 {
		t.Errorf("withoutCustomWidget changed its input: %q", summary(saved))
	}

	// A widget with no card leaves the layout as it was.
	if got := summary(withoutCustomWidget(removed, 9)); !slices.Equal(got, want) {
		t.Errorf("withoutCustomWidget(9) = %q, want it unchanged: %q", got, want)
	}
	// Removing the last card leaves an empty page, not a null one.
	empty := withoutCustomWidget(layoutOf(custom(5, "custom-5", 2)), 5)
	if empty.Root.Props.Blocks == nil || len(empty.Root.Props.Blocks) != 0 {
		t.Errorf("withoutCustomWidget of the only card = %+v, want an empty, non-nil list", empty.Root.Props.Blocks)
	}
}

// decodeIncoming parses a layout as SaveLayout would.
func decodeIncoming(t *testing.T, layoutJSON string) incomingData {
	t.Helper()
	var in incomingData
	if err := json.Unmarshal([]byte(layoutJSON), &in); err != nil {
		t.Fatalf("decode %s: %v", layoutJSON, err)
	}
	return in
}

// withBlocks wraps the JSON of some cards in a whole layout.
func withBlocks(cards ...string) string {
	return `{"root":{"props":{"blocks":[` + strings.Join(cards, ",") + `]}}}`
}

// repeat returns n copies of s.
func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func TestParseLayout(t *testing.T) {
	owned := map[int64]bool{5: true, 6: true}

	tests := []struct {
		name    string
		layout  string
		want    []string // the saved cards, when the layout is valid
		wantMsg string   // a substring of the message, when it isn't
	}{
		{
			name: "a valid layout",
			layout: withBlocks(
				`{"type":"Custom","props":{"id":"custom-5","span":4,"widgetId":5}}`,
				`{"type":"LocalEvents","props":{"id":"events","span":1}}`,
				`{"type":"Articles","props":{"id":"articles","span":3}}`,
			),
			want: []string{"Custom custom-5 4 #5", "LocalEvents events 1", "Articles articles 3"},
		},
		{name: "an empty page", layout: withBlocks(), want: []string{}},
		{
			name: "Puck's own ids are fine",
			layout: withBlocks(
				`{"type":"Articles","props":{"id":"Articles-0c1d2e3f-4a5b-6c7d-8e9f-a0b1c2d3e4f5","span":2}}`,
			),
			want: []string{"Articles Articles-0c1d2e3f-4a5b-6c7d-8e9f-a0b1c2d3e4f5 2"},
		},
		{
			name:   "a widgetId on a curated card is dropped",
			layout: withBlocks(`{"type":"Articles","props":{"id":"a","span":2,"widgetId":5}}`),
			want:   []string{"Articles a 2"},
		},
		{name: "no blocks", layout: `{"root":{"props":{}}}`, wantMsg: "Send JSON like"},
		{name: "null blocks", layout: `{"root":{"props":{"blocks":null}}}`, wantMsg: "Send JSON like"},
		{name: "no root", layout: `{}`, wantMsg: "Send JSON like"},
		{name: "unknown type", layout: withBlocks(`{"type":"Weather","props":{"id":"w","span":2}}`), wantMsg: `Card 1 has a type we don't know: "Weather".`},
		{name: "no type", layout: withBlocks(`{"props":{"id":"w","span":2}}`), wantMsg: "Card 1 has a type"},
		{name: "no id", layout: withBlocks(`{"type":"Articles","props":{"span":2}}`), wantMsg: "Card 1 needs an id."},
		{name: "blank id", layout: withBlocks(`{"type":"Articles","props":{"id":"  ","span":2}}`), wantMsg: "Card 1 needs an id."},
		{
			name:   "a 100-byte id is fine",
			layout: withBlocks(`{"type":"Articles","props":{"id":"` + strings.Repeat("x", 100) + `","span":2}}`),
			want:   []string{"Articles " + strings.Repeat("x", 100) + " 2"},
		},
		{name: "long id", layout: withBlocks(`{"type":"Articles","props":{"id":"` + strings.Repeat("x", 101) + `","span":2}}`), wantMsg: "id is too long"},
		{
			name: "duplicate ids",
			layout: withBlocks(
				`{"type":"Articles","props":{"id":"same","span":2}}`,
				`{"type":"LocalEvents","props":{"id":"same","span":2}}`,
			),
			wantMsg: `Two cards share the id "same".`,
		},
		{name: "span 0", layout: withBlocks(`{"type":"Articles","props":{"id":"a","span":0}}`), wantMsg: "Card 1's width must be a whole number from 1 to 4."},
		{name: "span 5", layout: withBlocks(`{"type":"Articles","props":{"id":"a","span":5}}`), wantMsg: "width must be"},
		{name: "span -1", layout: withBlocks(`{"type":"Articles","props":{"id":"a","span":-1}}`), wantMsg: "width must be"},
		{name: "span 2.5", layout: withBlocks(`{"type":"Articles","props":{"id":"a","span":2.5}}`), wantMsg: "width must be"},
		{name: "span as a string", layout: withBlocks(`{"type":"Articles","props":{"id":"a","span":"2"}}`), wantMsg: "width must be"},
		{name: "span null", layout: withBlocks(`{"type":"Articles","props":{"id":"a","span":null}}`), wantMsg: "width must be"},
		{name: "no span", layout: withBlocks(`{"type":"Articles","props":{"id":"a"}}`), wantMsg: "width must be"},
		{
			name: "the second card is the bad one",
			layout: withBlocks(
				`{"type":"Articles","props":{"id":"a","span":2}}`,
				`{"type":"LocalEvents","props":{"id":"e","span":7}}`,
			),
			wantMsg: "Card 2's width",
		},
		{name: "Custom with no widgetId", layout: withBlocks(`{"type":"Custom","props":{"id":"c","span":2}}`), wantMsg: "Card 1 doesn't say which widget it shows."},
		{name: "Custom with widgetId 0", layout: withBlocks(`{"type":"Custom","props":{"id":"c","span":2,"widgetId":0}}`), wantMsg: "doesn't say which widget"},
		{name: "Custom with a fractional widgetId", layout: withBlocks(`{"type":"Custom","props":{"id":"c","span":2,"widgetId":5.5}}`), wantMsg: "doesn't say which widget"},
		{name: "someone else's widget", layout: withBlocks(`{"type":"Custom","props":{"id":"c","span":2,"widgetId":7}}`), wantMsg: "That layout mentions a widget that isn't yours."},
		{
			name: "two Articles cards",
			layout: withBlocks(
				`{"type":"Articles","props":{"id":"a1","span":2}}`,
				`{"type":"Articles","props":{"id":"a2","span":2}}`,
			),
			wantMsg: "The layout has more than one Articles card.",
		},
		{
			name: "the same widget twice",
			layout: withBlocks(
				`{"type":"Custom","props":{"id":"c1","span":2,"widgetId":5}}`,
				`{"type":"Custom","props":{"id":"c2","span":2,"widgetId":5}}`,
			),
			wantMsg: "That layout shows the same widget twice.",
		},
		{
			name: "two LocalEvents cards",
			layout: withBlocks(
				`{"type":"LocalEvents","props":{"id":"e1","span":2}}`,
				`{"type":"LocalEvents","props":{"id":"e2","span":2}}`,
			),
			wantMsg: "more than one LocalEvents card",
		},
		{
			name:    "too many cards",
			layout:  withBlocks(repeat(`{"type":"Custom","props":{"id":"c","span":2,"widgetId":5}}`, MaxBlocks+1)...),
			wantMsg: "That layout has too many cards.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, msg, _ := parseLayout(decodeIncoming(t, tt.layout), owned)
			if tt.wantMsg != "" {
				if !strings.Contains(msg, tt.wantMsg) {
					t.Errorf("message = %q, want it to contain %q", msg, tt.wantMsg)
				}
				return
			}
			if msg != "" {
				t.Fatalf("refused a valid layout: %q", msg)
			}
			if cards := summary(got); !slices.Equal(cards, tt.want) {
				t.Errorf("parsed = %q, want %q", cards, tt.want)
			}
		})
	}
}

// Every saved widget gets a card, so a member with more widgets than
// MaxBlocks can still save a page holding all of them.
func TestParseLayoutRoomForEveryWidget(t *testing.T) {
	owned := make(map[int64]bool)
	var cards []string
	for id := int64(1); id <= MaxBlocks+5; id++ {
		owned[id] = true
		cards = append(cards, fmt.Sprintf(`{"type":"Custom","props":{"id":"custom-%d","span":2,"widgetId":%d}}`, id, id))
	}
	cards = append(cards, `{"type":"Articles","props":{"id":"articles","span":2}}`, `{"type":"LocalEvents","props":{"id":"events","span":2}}`)

	got, msg, _ := parseLayout(decodeIncoming(t, withBlocks(cards...)), owned)
	if msg != "" || len(got.Root.Props.Blocks) != MaxBlocks+7 {
		t.Errorf("got %d cards, message %q; want all %d saved", len(got.Root.Props.Blocks), msg, MaxBlocks+7)
	}
}

// Only the fields Data names are kept: extra props, extra root props, and
// anything in "content" or "zones" are dropped.
func TestParseLayoutStripsExtras(t *testing.T) {
	in := decodeIncoming(t, `{
		"root": {"props": {"title": "My page", "blocks": [
			{"type": "Articles", "props": {"id": "articles", "span": 3, "html": "<script>", "puck": {"x": 1}}, "readOnly": {"span": true}}
		]}, "readOnly": {}},
		"content": [{"type": "Articles", "props": {"id": "sneaky", "span": 1}}],
		"zones": {"root:extra": [{"type": "Custom", "props": {"id": "z", "widgetId": 99}}]},
		"anything": "else"
	}`)
	got, msg, _ := parseLayout(in, nil)
	if msg != "" {
		t.Fatalf("refused: %q", msg)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"root":{"props":{"blocks":[{"type":"Articles","props":{"id":"articles","span":3}}]}},"content":[],"zones":{}}`
	if string(encoded) != want {
		t.Errorf("normalised =\n%s\nwant\n%s", encoded, want)
	}
}

// The hint for a long id names the rule the check uses: bytes, not
// characters ("é" is two bytes).
func TestParseLayoutLongIDHint(t *testing.T) {
	in := decodeIncoming(t, withBlocks(`{"type":"Articles","props":{"id":"`+strings.Repeat("é", 51)+`","span":2}}`))
	_, msg, hint := parseLayout(in, nil)
	if msg != "Card 1's id is too long." || hint != "Keep ids to 100 bytes or fewer." {
		t.Errorf("got %q / %q", msg, hint)
	}
}
