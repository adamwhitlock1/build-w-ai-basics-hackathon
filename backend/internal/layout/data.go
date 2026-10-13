package layout

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The kinds of card a homepage can hold. They match the component names in
// the frontend's Puck config (frontend/src/puck/config.tsx).
const (
	TypeToday       = "Today"
	TypeArticles    = "Articles"
	TypeLocalEvents = "LocalEvents"
	TypeCustom      = "Custom"
)

// Card widths, in quarters of the page: the grid has four columns, and a
// card spans 1 to 4 of them.
const (
	MinSpan     = 1
	MaxSpan     = 4
	DefaultSpan = 2
)

// MaxBlocks is how many cards a saved layout may hold, unless the member
// has more saved widgets than that (see parseLayout).
const MaxBlocks = 50

// maxIDBytes caps a card's id. Our ids are short ("custom-12"), and so are
// the ones Puck makes up.
const maxIDBytes = 100

// Data is a homepage layout in the shape Puck reads and writes:
//
//	{"root": {"props": {"blocks": [...]}}, "content": [], "zones": {}}
//
// Our Puck config keeps every card in the root's "blocks" slot (a slot is
// a field that holds other components), so the top-level "content" list and
// the legacy "zones" are always empty. Puck still expects to find them.
//
// It is also exactly what we store and send back: only the fields named in
// these structs are ever encoded, so anything else a browser sends is
// dropped (see parseLayout).
type Data struct {
	Root    Root               `json:"root"`
	Content []Block            `json:"content"` // always empty
	Zones   map[string][]Block `json:"zones"`   // always empty
}

// Root is Puck's root component: the page itself.
type Root struct {
	Props RootProps `json:"props"`
}

// RootProps holds the page's cards, in order.
type RootProps struct {
	Blocks []Block `json:"blocks"`
}

// Block is one card on the page.
type Block struct {
	Type  string     `json:"type"` // TypeToday, TypeArticles, TypeLocalEvents, or TypeCustom
	Props BlockProps `json:"props"`
}

// BlockProps are a card's settings.
type BlockProps struct {
	// ID tells cards apart; it is unique within a layout. Puck needs one on
	// every component.
	ID   string `json:"id"`
	Span int    `json:"span"` // width in quarters, MinSpan to MaxSpan
	// WidgetID is the custom_widgets row a Custom card shows. omitempty
	// leaves it out of the JSON for the curated cards, where it is 0.
	WidgetID int64 `json:"widgetId,omitempty"`
}

// newData wraps blocks in a complete Data value. Making the slices and the
// map here means they encode as [] and {}, never null, which Puck can't
// read.
func newData(blocks []Block) Data {
	if blocks == nil {
		blocks = []Block{}
	}
	return Data{
		Root:    Root{Props: RootProps{Blocks: blocks}},
		Content: []Block{},
		Zones:   map[string][]Block{},
	}
}

// Default is the layout of someone who has never arranged their page: the
// Today card across the top, Articles and Local Events side by side at half
// width, then a half-width card for each saved widget. widgetIDs are the
// member's saved widgets, oldest first.
func Default(widgetIDs []int64) Data {
	curated := []Block{
		todayBlock(),
		{Type: TypeArticles, Props: BlockProps{Span: DefaultSpan}},
		{Type: TypeLocalEvents, Props: BlockProps{Span: DefaultSpan}},
	}
	// heal gives the curated cards their ids, and appends a card for every
	// widget not already on the page, which here is all of them.
	return heal(newData(curated), widgetIDs)
}

// todayBlock is the Today card where a page first gets it: full width.
func todayBlock() Block {
	return Block{Type: TypeToday, Props: BlockProps{ID: "today", Span: MaxSpan}}
}

// customBlock is the card for a newly added saved widget.
func customBlock(widgetID int64) Block {
	b := Block{Type: TypeCustom, Props: BlockProps{Span: DefaultSpan, WidgetID: widgetID}}
	b.Props.ID = defaultID(b)
	return b
}

// defaultID is the id we give a card: "today", "articles", "events", or
// "custom-<widget id>". Ids made in the browser by Puck look different,
// and that's fine: they only need to be unique.
func defaultID(b Block) string {
	switch b.Type {
	case TypeToday:
		return "today"
	case TypeArticles:
		return "articles"
	case TypeLocalEvents:
		return "events"
	default:
		return "custom-" + strconv.FormatInt(b.Props.WidgetID, 10)
	}
}

// heal fixes a saved layout so it matches the widgets the member has now.
// widgetIDs are their saved widgets, oldest first.
//
//   - A Custom card whose widget no longer exists is dropped, and so is a
//     second card for the same widget: each widget has one card.
//   - A saved widget with no card gets one at the end, so a widget can
//     never go missing from the page (for example, if putting it on the
//     layout failed when it was created).
//   - Today, Articles and Local Events appear at most once each. The page
//     can't remove them, so Today missing means the layout was saved before
//     the Today card existed: it's put back at the top, full width. Articles
//     or Local Events missing is left as it is.
//   - Cards of unknown types are dropped, and widths and ids are made valid.
//     Only this package writes layouts, so these shouldn't be needed, but a
//     page that still loads beats an error.
func heal(d Data, widgetIDs []int64) Data {
	exists := make(map[int64]bool, len(widgetIDs))
	for _, id := range widgetIDs {
		exists[id] = true
	}

	blocks := make([]Block, 0, len(d.Root.Props.Blocks)+len(widgetIDs))
	usedIDs := make(map[string]bool)
	curatedSeen := make(map[string]bool) // TypeToday / TypeArticles / TypeLocalEvents already kept
	widgetShown := make(map[int64]bool)  // widgets that already have a card

	// Ranging over a slice of structs gives a copy of each element, so
	// changing b below doesn't touch d.
	for _, b := range d.Root.Props.Blocks {
		switch b.Type {
		case TypeToday, TypeArticles, TypeLocalEvents:
			if curatedSeen[b.Type] {
				continue
			}
			curatedSeen[b.Type] = true
			b.Props.WidgetID = 0
		case TypeCustom:
			if !exists[b.Props.WidgetID] || widgetShown[b.Props.WidgetID] {
				continue
			}
			widgetShown[b.Props.WidgetID] = true
		default:
			continue
		}
		if b.Props.Span < MinSpan || b.Props.Span > MaxSpan {
			b.Props.Span = DefaultSpan
		}
		if b.Props.ID == "" {
			b.Props.ID = defaultID(b)
		}
		b.Props.ID = claimID(usedIDs, b.Props.ID)
		blocks = append(blocks, b)
	}

	for _, id := range widgetIDs {
		if !widgetShown[id] {
			b := customBlock(id)
			b.Props.ID = claimID(usedIDs, b.Props.ID)
			blocks = append(blocks, b)
		}
	}
	if !curatedSeen[TypeToday] {
		b := todayBlock()
		b.Props.ID = claimID(usedIDs, b.Props.ID)
		blocks = append([]Block{b}, blocks...)
	}
	return newData(blocks)
}

// withCustomWidget returns d with a card for widgetID added at the end,
// unless the widget already has one.
func withCustomWidget(d Data, widgetID int64) Data {
	usedIDs := make(map[string]bool, len(d.Root.Props.Blocks))
	for _, b := range d.Root.Props.Blocks {
		if b.Type == TypeCustom && b.Props.WidgetID == widgetID {
			return d
		}
		usedIDs[b.Props.ID] = true
	}
	b := customBlock(widgetID)
	b.Props.ID = claimID(usedIDs, b.Props.ID)
	// Copying into a new slice leaves the caller's d unchanged: append could
	// otherwise write into spare room at the end of d's backing array.
	blocks := append(append([]Block{}, d.Root.Props.Blocks...), b)
	return newData(blocks)
}

// withoutCustomWidget returns d without the card for widgetID, for when the
// widget is deleted. Like withCustomWidget, it leaves d itself unchanged.
func withoutCustomWidget(d Data, widgetID int64) Data {
	blocks := make([]Block, 0, len(d.Root.Props.Blocks))
	for _, b := range d.Root.Props.Blocks {
		if b.Type == TypeCustom && b.Props.WidgetID == widgetID {
			continue
		}
		blocks = append(blocks, b)
	}
	return newData(blocks)
}

// claimID returns want, or want with "-2", "-3", ... added if it is already
// in used, and records the result in used.
func claimID(used map[string]bool, want string) string {
	id := want
	for n := 2; used[id]; n++ {
		id = want + "-" + strconv.Itoa(n)
	}
	used[id] = true
	return id
}

// incomingData is a layout as a browser sent it: untrusted until
// parseLayout has checked it. Only the parts we use are decoded, so
// anything else in the JSON is ignored here and never stored.
type incomingData struct {
	Root struct {
		Props struct {
			// A missing or null "blocks" decodes as a nil slice, which
			// parseLayout refuses; [] is a valid, empty page.
			Blocks []incomingBlock `json:"blocks"`
		} `json:"props"`
	} `json:"root"`
}

// incomingBlock is one card as a browser sent it. The numbers stay raw
// JSON until parseLayout checks them: decoding 2.5 straight into an int
// would fail the whole request with a message about Go types, and we'd
// rather say which card is wrong.
type incomingBlock struct {
	Type  string `json:"type"`
	Props struct {
		ID       string          `json:"id"`
		Span     json.RawMessage `json:"span"`
		WidgetID json.RawMessage `json:"widgetId"`
	} `json:"props"`
}

// parseLayout checks a layout a member wants to save and returns the
// normalised version to store: the same cards, holding only the fields
// Data names. owned is the set of the member's saved widget ids. If the
// layout can't be saved it returns a message and hint for the member
// instead, and Data is meaningless.
func parseLayout(in incomingData, owned map[int64]bool) (d Data, msg, hint string) {
	cards := in.Root.Props.Blocks
	if cards == nil {
		return Data{}, saveExample, ""
	}
	// GET /api/me/homepage gives every saved widget a card, so someone with
	// more widgets than MaxBlocks must still be able to save their page.
	// max is built in since Go 1.21.
	if limit := max(MaxBlocks, len(owned)+3); len(cards) > limit {
		return Data{}, "That layout has too many cards.", fmt.Sprintf("A page holds up to %d.", limit)
	}

	blocks := make([]Block, 0, len(cards))
	usedIDs := make(map[string]bool, len(cards))
	curatedSeen := make(map[string]bool) // TypeToday / TypeArticles / TypeLocalEvents already seen
	widgetSeen := make(map[int64]bool)   // widgets that already have a card
	for i, c := range cards {
		card := fmt.Sprintf("Card %d", i+1) // people count from 1

		switch c.Type {
		case TypeToday, TypeArticles, TypeLocalEvents, TypeCustom:
		default:
			return Data{}, fmt.Sprintf("%s has a type we don't know: %q.", card, c.Type), "Use Today, Articles, LocalEvents, or Custom."
		}

		id := c.Props.ID
		switch {
		case strings.TrimSpace(id) == "":
			return Data{}, card + " needs an id.", ""
		case len(id) > maxIDBytes:
			// len counts bytes, and a letter like "é" takes two, so the hint
			// talks about bytes rather than characters.
			return Data{}, card + "'s id is too long.", fmt.Sprintf("Keep ids to %d bytes or fewer.", maxIDBytes)
		case usedIDs[id]:
			return Data{}, fmt.Sprintf("Two cards share the id %q.", id), "Give each card its own id."
		}
		usedIDs[id] = true

		span, ok := wholeNumber(c.Props.Span)
		if !ok || span < MinSpan || span > MaxSpan {
			return Data{}, fmt.Sprintf("%s's width must be a whole number from %d to %d.", card, MinSpan, MaxSpan), "Widths are counted in quarters of the page."
		}

		b := Block{Type: c.Type, Props: BlockProps{ID: id, Span: int(span)}}
		switch c.Type {
		case TypeToday, TypeArticles, TypeLocalEvents:
			// Any widgetId sent with a curated card is dropped: b leaves it 0.
			if curatedSeen[c.Type] {
				return Data{}, fmt.Sprintf("The layout has more than one %s card.", c.Type), "Keep one of each."
			}
			curatedSeen[c.Type] = true
		case TypeCustom:
			widgetID, ok := wholeNumber(c.Props.WidgetID)
			if !ok || widgetID <= 0 {
				return Data{}, card + " doesn't say which widget it shows.", `Give it a "widgetId".`
			}
			if !owned[widgetID] {
				// The same answer for someone else's widget and a missing
				// one, so ids can't be probed.
				return Data{}, "That layout mentions a widget that isn't yours.", "Reload the page and try again."
			}
			// One card per widget, as heal keeps it. It also means a layout
			// holds at most len(owned)+3 cards, so whatever GET sends can
			// always be saved back under the limit above.
			if widgetSeen[widgetID] {
				return Data{}, "That layout shows the same widget twice.", "Keep one card for each widget."
			}
			widgetSeen[widgetID] = true
			b.Props.WidgetID = widgetID
		}
		blocks = append(blocks, b)
	}
	return newData(blocks), "", ""
}

// wholeNumber reads raw as a JSON whole number. It reports false for
// anything else: a missing value, a fraction like 2.5, or a string like
// "2". (JSON null reads as 0, which every caller's range check refuses.)
func wholeNumber(raw json.RawMessage) (int64, bool) {
	var n int64
	if len(raw) == 0 || json.Unmarshal(raw, &n) != nil {
		return 0, false
	}
	return n, true
}
