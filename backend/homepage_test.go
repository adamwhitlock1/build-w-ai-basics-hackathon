package main

// End-to-end tests of the homepage layout: GET and PUT /api/me/homepage,
// and how new widgets join the layout. They reuse the server and client
// helpers in routes_test.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"porchlight/internal/layout"
	"porchlight/internal/respond"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// cards describes a layout's cards as short strings like
// "Articles articles 2" or "Custom custom-7 4 #7" (type, id, span, widget
// id), so whole layouts compare at a glance.
func cards(d layout.Data) []string {
	out := []string{}
	for _, b := range d.Root.Props.Blocks {
		s := fmt.Sprintf("%s %s %d", b.Type, b.Props.ID, b.Props.Span)
		if b.Props.WidgetID != 0 {
			s += fmt.Sprintf(" #%d", b.Props.WidgetID)
		}
		out = append(out, s)
	}
	return out
}

// card is a curated card as the browser would send it. span is an any so
// tests can send bad values like 2.5 or "2".
func card(typ, id string, span any) map[string]any {
	return map[string]any{"type": typ, "props": map[string]any{"id": id, "span": span}}
}

// custom is a Custom card as the browser would send it.
func custom(widgetID int64, id string, span any) map[string]any {
	return map[string]any{"type": "Custom", "props": map[string]any{"id": id, "span": span, "widgetId": widgetID}}
}

// blocks wraps cards in a whole Puck layout.
func blocks(cards ...map[string]any) map[string]any {
	if cards == nil {
		cards = []map[string]any{} // [] rather than null
	}
	return map[string]any{
		"root":    map[string]any{"props": map[string]any{"blocks": cards}},
		"content": []any{},
		"zones":   map[string]any{},
	}
}

// homepage fetches the client's homepage.
func (c *client) homepage() layout.Homepage {
	c.t.Helper()
	var page layout.Homepage
	c.do("GET", "/api/me/homepage", nil, http.StatusOK, &page)
	return page
}

// addWidget describes a widget with the fake planner and returns it.
func (c *client) addWidget() widgets.SavedWidget {
	c.t.Helper()
	var w widgets.SavedWidget
	c.do("POST", "/api/widgets/from-prompt", map[string]string{"prompt": "show me upcoming SpaceX launches"}, http.StatusCreated, &w)
	return w
}

func customCard(id int64, span int) string {
	return fmt.Sprintf("Custom custom-%d %d #%d", id, span, id)
}

func TestHomepageLayout(t *testing.T) {
	srv, st := newTestServer(t)
	c := newClient(t, srv)
	me := c.signup("parent@example.com")
	c.setUpFamily()

	// A new member gets the default layout: the two curated cards at half
	// width. The JSON is complete Puck data, with no nulls.
	resp, err := c.http.Get(srv.URL + "/api/me/homepage")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Layout json.RawMessage `json:"layout"`
	}
	err = json.NewDecoder(resp.Body).Decode(&raw)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	wantDefault := `{"root":{"props":{"blocks":[` +
		`{"type":"Articles","props":{"id":"articles","span":2}},` +
		`{"type":"LocalEvents","props":{"id":"events","span":2}}` +
		`]}},"content":[],"zones":{}}`
	if string(raw.Layout) != wantDefault {
		t.Errorf("default layout =\n%s\nwant\n%s", raw.Layout, wantDefault)
	}

	// A widget added before the page was ever arranged shows up through the
	// default layout; nothing is saved yet.
	first := c.addWidget()
	want := []string{"Articles articles 2", "LocalEvents events 2", customCard(first.ID, 2)}
	if got := cards(c.homepage().Layout); !slices.Equal(got, want) {
		t.Errorf("after the first widget: layout = %q, want %q", got, want)
	}
	if _, err := st.GetLayout(context.Background(), me.User.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a layout was saved before the page was arranged (err = %v)", err)
	}

	// Arrange: reorder and resize. The response and the next GET both give
	// back exactly what was saved.
	arranged := blocks(
		custom(first.ID, "custom-"+strconv.FormatInt(first.ID, 10), 4),
		card("LocalEvents", "events", 1),
		card("Articles", "articles", 3),
	)
	var saved layout.SaveResponse
	c.do("PUT", "/api/me/homepage", map[string]any{"layout": arranged}, http.StatusOK, &saved)
	want = []string{customCard(first.ID, 4), "LocalEvents events 1", "Articles articles 3"}
	if got := cards(saved.Layout); !slices.Equal(got, want) {
		t.Errorf("PUT response layout = %q, want %q", got, want)
	}
	page := c.homepage()
	if got := cards(page.Layout); !slices.Equal(got, want) {
		t.Errorf("GET after PUT: layout = %q, want %q", got, want)
	}
	if page.Layout.Content == nil || page.Layout.Zones == nil {
		t.Errorf("layout = %+v, want content and zones present", page.Layout)
	}

	// A widget added now goes on the end of the saved layout, in the
	// database too (not only patched in by GET).
	second := c.addWidget()
	want = append(want, customCard(second.ID, 2))
	if got := cards(c.homepage().Layout); !slices.Equal(got, want) {
		t.Errorf("after the second widget: layout = %q, want %q", got, want)
	}
	stored, err := st.GetLayout(context.Background(), me.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stored), fmt.Sprintf(`"widgetId":%d`, second.ID)) {
		t.Errorf("stored layout = %s, want the second widget in it", stored)
	}

	// A member can save a page without some cards (here, the curated ones).
	c.do("PUT", "/api/me/homepage", map[string]any{"layout": blocks(
		custom(second.ID, "s", 1),
		custom(first.ID, "f", 2),
	)}, http.StatusOK, nil)
	want = []string{fmt.Sprintf("Custom s 1 #%d", second.ID), fmt.Sprintf("Custom f 2 #%d", first.ID)}
	if got := cards(c.homepage().Layout); !slices.Equal(got, want) {
		t.Errorf("without curated cards: layout = %q, want %q", got, want)
	}
}

// GET /api/me/homepage repairs a saved layout that no longer matches the
// member's widgets.
func TestHomepageLayoutHeals(t *testing.T) {
	ctx := context.Background()
	srv, st := newTestServer(t)
	c := newClient(t, srv)
	me := c.signup("parent@example.com")
	c.setUpFamily()

	gone := c.addWidget()
	kept := c.addWidget()
	c.do("PUT", "/api/me/homepage", map[string]any{"layout": blocks(
		custom(kept.ID, "kept", 3),
		custom(gone.ID, "gone", 1),
		card("Articles", "articles", 2),
	)}, http.StatusOK, nil)

	// Behind the API's back: one widget disappears, and another arrives
	// without a card (as if adding it to the layout had failed).
	if _, err := st.DB().Exec(`DELETE FROM custom_widgets WHERE id = ?`, gone.ID); err != nil {
		t.Fatal(err)
	}
	orphan, err := st.CreateCustomWidget(ctx, store.CustomWidget{
		UserID: me.User.ID, Name: "Orphan", Source: "web_search", Query: "orphan",
		RenderSpec: json.RawMessage(`{"title":"Orphan","archetype":"list","items":[]}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	page := c.homepage()
	// The gone widget's card is dropped, the orphan gets one at the end, and
	// Local Events (removed by the member) is not put back.
	want := []string{fmt.Sprintf("Custom kept 3 #%d", kept.ID), "Articles articles 2", customCard(orphan.ID, 2)}
	if got := cards(page.Layout); !slices.Equal(got, want) {
		t.Errorf("healed layout = %q, want %q", got, want)
	}
	if len(page.CustomWidgets) != 2 {
		t.Errorf("customWidgets has %d widgets, want 2", len(page.CustomWidgets))
	}

	// The healed layout can be saved straight back.
	c.do("PUT", "/api/me/homepage", map[string]any{"layout": page.Layout}, http.StatusOK, nil)

	// A stored layout that can't be read falls back to the default.
	if err := st.UpsertLayout(ctx, me.User.ID, json.RawMessage(`{"root": "not a layout"}`)); err != nil {
		t.Fatal(err)
	}
	want = []string{"Articles articles 2", "LocalEvents events 2", customCard(kept.ID, 2), customCard(orphan.ID, 2)}
	if got := cards(c.homepage().Layout); !slices.Equal(got, want) {
		t.Errorf("unreadable layout: got %q, want the default %q", got, want)
	}
	// Adding a widget doesn't fail because of it, either.
	c.addWidget()
}

func TestSaveLayoutRejects(t *testing.T) {
	srv, _ := newTestServer(t)
	c := newClient(t, srv)
	c.signup("parent@example.com")
	c.setUpFamily()
	mine := c.addWidget()

	bob := newClient(t, srv)
	bob.signup("bob@example.com")
	bobs := bob.addWidget()

	// What's saved before the bad requests, and must still be there after.
	good := blocks(card("Articles", "articles", 3), card("LocalEvents", "events", 1), custom(mine.ID, "mine", 4))
	c.do("PUT", "/api/me/homepage", map[string]any{"layout": good}, http.StatusOK, nil)
	before := cards(c.homepage().Layout)

	tooMany := make([]map[string]any, layout.MaxBlocks+1)
	for i := range tooMany {
		tooMany[i] = custom(mine.ID, "c"+strconv.Itoa(i), 1)
	}
	sendJSON := `Send JSON like {"layout": {"root": {"props": {"blocks": [...]}}}}.`

	tests := []struct {
		name    string
		body    any
		wantMsg string
	}{
		{"not JSON", "layout=yes", sendJSON},
		{"no layout", map[string]any{}, sendJSON},
		{"null layout", map[string]any{"layout": nil}, sendJSON},
		{"layout is not an object", map[string]any{"layout": []int{1}}, sendJSON},
		{"no blocks", map[string]any{"layout": map[string]any{"root": map[string]any{"props": map[string]any{}}}}, sendJSON},
		{"blocks is not an array", map[string]any{"layout": map[string]any{"root": map[string]any{"props": map[string]any{"blocks": "x"}}}}, sendJSON},
		{"unknown type", map[string]any{"layout": blocks(card("Weather", "w", 2))}, `Card 1 has a type we don't know: "Weather".`},
		{"span 0", map[string]any{"layout": blocks(card("Articles", "a", 0))}, "Card 1's width must be a whole number from 1 to 4."},
		{"span 5", map[string]any{"layout": blocks(card("Articles", "a", 5))}, "Card 1's width must be a whole number from 1 to 4."},
		{"span 2.5", map[string]any{"layout": blocks(card("Articles", "a", 2.5))}, "Card 1's width must be a whole number from 1 to 4."},
		{"span as a string", map[string]any{"layout": blocks(card("Articles", "a", "2"))}, "Card 1's width must be a whole number from 1 to 4."},
		{"missing id", map[string]any{"layout": blocks(card("Articles", "", 2))}, "Card 1 needs an id."},
		{"duplicate ids", map[string]any{"layout": blocks(card("Articles", "x", 2), card("LocalEvents", "x", 2))}, `Two cards share the id "x".`},
		{"someone else's widget", map[string]any{"layout": blocks(custom(bobs.ID, "b", 2))}, "That layout mentions a widget that isn't yours."},
		{"a widget that doesn't exist", map[string]any{"layout": blocks(custom(9999, "n", 2))}, "That layout mentions a widget that isn't yours."},
		{"Custom with no widgetId", map[string]any{"layout": blocks(card("Custom", "c", 2))}, "Card 1 doesn't say which widget it shows."},
		{"two Articles cards", map[string]any{"layout": blocks(card("Articles", "a1", 2), card("Articles", "a2", 2))}, "The layout has more than one Articles card."},
		{"the same widget twice", map[string]any{"layout": blocks(custom(mine.ID, "m1", 2), custom(mine.ID, "m2", 2))}, "That layout shows the same widget twice."},
		{"too many cards", map[string]any{"layout": blocks(tooMany...)}, "That layout has too many cards."},
		{"body over 256 KB", map[string]any{"layout": good, "padding": strings.Repeat("x", 300<<10)}, "That layout is too big to save."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.wantError("PUT", "/api/me/homepage", tt.body, http.StatusBadRequest, tt.wantMsg)
		})
	}

	// Malformed JSON and trailing data, which c.do can't send.
	for _, body := range []string{`{"layout":`, `{"layout":{"root":{"props":{"blocks":[]}}}} {}`} {
		if status := sendRaw(t, c, "PUT", "/api/me/homepage", body, nil); status != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, status)
		}
	}

	// None of it was saved.
	if after := cards(c.homepage().Layout); !slices.Equal(after, before) {
		t.Errorf("layout after rejected saves = %q, want it unchanged: %q", after, before)
	}
}

// Deleting a widget takes it off the page for good. (Saving a layout without
// its card doesn't: GET puts the card back, so a widget is never lost.)
func TestDeleteWidget(t *testing.T) {
	srv, st := newTestServer(t)
	c := newClient(t, srv)
	me := c.signup("parent@example.com")
	c.setUpFamily()
	first := c.addWidget()
	second := c.addWidget()
	path := func(id int64) string { return "/api/widgets/custom/" + strconv.FormatInt(id, 10) }

	// An arranged page, so there is a stored layout to take the card off.
	c.do("PUT", "/api/me/homepage", map[string]any{"layout": blocks(
		custom(first.ID, "custom-"+strconv.FormatInt(first.ID, 10), 3),
		card("Articles", "articles", 1),
		custom(second.ID, "custom-"+strconv.FormatInt(second.ID, 10), 2),
	)}, http.StatusOK, nil)

	// Nobody else can delete it, and nor can another site.
	bob := newClient(t, srv)
	bob.signup("bob@example.com")
	bob.wantError("DELETE", path(first.ID), nil, http.StatusNotFound, "That widget doesn't exist.")
	if status := sendRaw(t, c, "DELETE", path(first.ID), "", map[string]string{"Origin": "http://localhost:5173"}); status != http.StatusForbidden {
		t.Errorf("cross-site DELETE: status = %d, want 403", status)
	}
	if status := sendRaw(t, c, "DELETE", path(first.ID), "", map[string]string{"Sec-Fetch-Site": "cross-site"}); status != http.StatusForbidden {
		t.Errorf("DELETE a browser says is cross-site: status = %d, want 403", status)
	}
	if got := len(c.homepage().CustomWidgets); got != 2 {
		t.Fatalf("after refused deletes: %d widgets, want 2", got)
	}

	c.do("DELETE", path(first.ID), nil, http.StatusNoContent, nil)

	// The widget and its card are gone, from the stored layout too, and the
	// rest of the page is as it was.
	want := []string{"Articles articles 1", customCard(second.ID, 2)}
	page := c.homepage()
	if got := cards(page.Layout); !slices.Equal(got, want) {
		t.Errorf("after the delete: layout = %q, want %q", got, want)
	}
	if len(page.CustomWidgets) != 1 || page.CustomWidgets[0].ID != second.ID {
		t.Errorf("after the delete: customWidgets = %+v, want only %d", page.CustomWidgets, second.ID)
	}
	stored, err := st.GetLayout(context.Background(), me.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), fmt.Sprintf(`"widgetId":%d`, first.ID)) {
		t.Errorf("stored layout = %s, still has the deleted widget", stored)
	}

	// It stays gone: a widget added next goes on the end, and the deleted
	// one doesn't come back after it.
	third := c.addWidget()
	want = append(want, customCard(third.ID, 2))
	if got := cards(c.homepage().Layout); !slices.Equal(got, want) {
		t.Errorf("after adding another widget: layout = %q, want %q", got, want)
	}

	// It can't be read, deleted again, or put back on the page.
	c.wantError("GET", path(first.ID), nil, http.StatusNotFound, "That widget doesn't exist.")
	c.wantError("DELETE", path(first.ID), nil, http.StatusNotFound, "That widget doesn't exist.")
	c.wantError("PUT", "/api/me/homepage", map[string]any{"layout": blocks(custom(first.ID, "back", 2))},
		http.StatusBadRequest, "That layout mentions a widget that isn't yours.")

	// Someone who never arranged their page can delete a widget too: there's
	// no stored layout to change, and the default one leaves it out.
	bobs := bob.addWidget()
	bob.do("DELETE", path(bobs.ID), nil, http.StatusNoContent, nil)
	if got, want := cards(bob.homepage().Layout), []string{"Articles articles 2", "LocalEvents events 2"}; !slices.Equal(got, want) {
		t.Errorf("bob's layout after his delete = %q, want %q", got, want)
	}
}

// Only the fields the app uses are stored; anything else a browser sends
// is dropped.
func TestSaveLayoutStripsExtraProps(t *testing.T) {
	srv, st := newTestServer(t)
	c := newClient(t, srv)
	me := c.signup("parent@example.com")
	c.setUpFamily()

	body := json.RawMessage(`{"layout": {
		"root": {"props": {"title": "My page", "blocks": [
			{"type": "Articles", "props": {"id": "articles", "span": 3, "widgetId": 12, "html": "<script>alert(1)</script>"}, "readOnly": {}},
			{"type": "LocalEvents", "props": {"id": "events", "span": 1, "nested": {"deep": [1, 2, 3]}}}
		]}},
		"content": [{"type": "Articles", "props": {"id": "sneaky", "span": 1}}],
		"zones": {"root:extra": []},
		"extra": true
	}}`)
	want := `{"root":{"props":{"blocks":[` +
		`{"type":"Articles","props":{"id":"articles","span":3}},` +
		`{"type":"LocalEvents","props":{"id":"events","span":1}}` +
		`]}},"content":[],"zones":{}}`

	var saved struct {
		Layout json.RawMessage `json:"layout"`
	}
	c.do("PUT", "/api/me/homepage", body, http.StatusOK, &saved)
	if string(saved.Layout) != want {
		t.Errorf("PUT response layout =\n%s\nwant\n%s", saved.Layout, want)
	}
	stored, err := st.GetLayout(context.Background(), me.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != want {
		t.Errorf("stored layout =\n%s\nwant\n%s", stored, want)
	}
}

func TestSaveLayoutCrossSite(t *testing.T) {
	srv, _ := newTestServer(t)
	c := newClient(t, srv)
	c.signup("parent@example.com")
	c.setUpFamily()
	before := cards(c.homepage().Layout)

	body := `{"layout":{"root":{"props":{"blocks":[]}}}}`
	tests := []struct {
		name       string
		headers    map[string]string
		wantStatus int
	}{
		{"another localhost port", map[string]string{"Origin": "http://localhost:5173", "Content-Type": "application/json"}, http.StatusForbidden},
		{"a browser says cross-site", map[string]string{"Sec-Fetch-Site": "cross-site", "Content-Type": "application/json"}, http.StatusForbidden},
		{"a text/plain body", map[string]string{"Content-Type": "text/plain"}, http.StatusUnsupportedMediaType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if status := sendRaw(t, c, "PUT", "/api/me/homepage", body, tt.headers); status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
	if after := cards(c.homepage().Layout); !slices.Equal(after, before) {
		t.Errorf("layout = %q after refused requests, want it unchanged: %q", after, before)
	}

	// The app itself gets through.
	headers := map[string]string{"Origin": srv.URL, "Sec-Fetch-Site": "same-origin", "Content-Type": "application/json"}
	if status := sendRaw(t, c, "PUT", "/api/me/homepage", body, headers); status != http.StatusOK {
		t.Errorf("same-origin PUT: status = %d, want 200", status)
	}
}

// sendRaw sends body exactly as given, with the client's cookies and the
// given headers (Content-Type defaults to application/json), and returns
// the status code.
func sendRaw(t *testing.T, c *client, method, path, body string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var errBody respond.ErrorBody
	_ = json.NewDecoder(resp.Body).Decode(&errBody) // only for the log below
	if resp.StatusCode >= 400 {
		t.Logf("%s %s: %d %q", method, path, resp.StatusCode, errBody.Error)
	}
	return resp.StatusCode
}
