package main

// End-to-end tests of the /api/members/{id}/... routes, where a parent
// looks at a family member's page and sets their interests. They reuse the
// server and client helpers in routes_test.go, homepage_test.go and
// invites_test.go.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"porchlight/internal/family"
	"porchlight/internal/layout"
	"porchlight/internal/widgets"
)

func memberPath(id int64, rest string) string {
	return fmt.Sprintf("/api/members/%d%s", id, rest)
}

// testFamily is a parent, alice, whose family is set up (her interests are
// "rockets", her location "Portland"), and her kid, who has no interests or
// location yet, each logged in on their own client.
type testFamily struct {
	srv            *httptest.Server
	alice, kid     *client
	aliceMe, kidMe family.Me
}

func newTestFamily(t *testing.T) testFamily {
	t.Helper()
	srv, _ := newTestServer(t)
	f := testFamily{srv: srv, alice: newClient(t, srv), kid: newClient(t, srv)}
	f.aliceMe = f.alice.signup("alice@example.com")
	f.alice.setUpFamily()
	f.kidMe = f.kid.accept(tokenOf(f.alice.invite(f.aliceMe.Family.ID, "kid@example.com", "kid", http.StatusCreated)))
	return f
}

func TestParentSeesAndSetsMemberPage(t *testing.T) {
	f := newTestFamily(t)
	srv, alice, kid, aliceMe, kidMe := f.srv, f.alice, f.kid, f.aliceMe, f.kidMe

	// The kid makes a widget and arranges their page; alice has one too.
	alice.addWidget()
	kidWidget := kid.addWidget()
	kidLayout := blocks(
		custom(kidWidget.ID, fmt.Sprintf("custom-%d", kidWidget.ID), 4),
		card("Articles", "articles", 1),
	)
	kid.do("PUT", "/api/me/homepage", map[string]any{"layout": kidLayout}, http.StatusOK, nil)

	// Alice sees the kid's page exactly as the kid does: their layout,
	// their widget, their (still empty) profile. Nothing of hers.
	var seen layout.Homepage
	alice.do("GET", memberPath(kidMe.User.ID, "/homepage"), nil, http.StatusOK, &seen)
	own := kid.homepage()
	if got, want := cards(seen.Layout), cards(own.Layout); !slices.Equal(got, want) {
		t.Errorf("alice sees the kid's layout as %q, want %q", got, want)
	}
	if len(seen.CustomWidgets) != 1 || seen.CustomWidgets[0].ID != kidWidget.ID {
		t.Errorf("alice sees the kid's widgets as %+v, want only widget %d", seen.CustomWidgets, kidWidget.ID)
	}
	if seen.Profile != (layout.Profile{}) {
		t.Errorf("kid's profile = %+v, want empty", seen.Profile)
	}

	// With no interests, the kid's Articles card asks for some, and their
	// events card for a location, without searching anything.
	var articles, events widgets.CuratedWidget
	alice.do("GET", memberPath(kidMe.User.ID, "/widgets/articles"), nil, http.StatusOK, &articles)
	alice.do("GET", memberPath(kidMe.User.ID, "/widgets/events"), nil, http.StatusOK, &events)
	if articles.EmptyReason != widgets.EmptyNoInterests || events.EmptyReason != widgets.EmptyNoLocation {
		t.Errorf("kid's empty reasons = %q, %q; want no_interests, no_location", articles.EmptyReason, events.EmptyReason)
	}

	// Alice sets the kid's interests and location. Validation is the same
	// as for her own.
	profilePath := memberPath(kidMe.User.ID, "/profile")
	alice.wantError("PUT", profilePath, map[string]string{"interests": "only interests"}, http.StatusBadRequest,
		`Send JSON like {"interests": "space exploration, soccer", "location": "Denver, CO"}.`)
	var saved family.ProfileResponse
	alice.do("PUT", profilePath, map[string]string{"interests": " minecraft, skateboarding ", "location": "Denver, CO"}, http.StatusOK, &saved)
	want := family.MeProfile{Interests: "minecraft, skateboarding", Location: "Denver, CO"}
	if saved.Profile != want {
		t.Errorf("saved = %+v, want %+v", saved.Profile, want)
	}

	// The kid's cards now follow the kid's new profile (the fake curator
	// plans one query per interest), not alice's rockets in Portland.
	alice.do("GET", memberPath(kidMe.User.ID, "/widgets/articles"), nil, http.StatusOK, &articles)
	if articles.Query != "minecraft · skateboarding" || len(articles.Items) != 2 || articles.EmptyReason != "" {
		t.Errorf("kid's articles = %+v, want two items for minecraft and skateboarding", articles)
	}
	alice.do("GET", memberPath(kidMe.User.ID, "/widgets/events"), nil, http.StatusOK, &events)
	if !strings.Contains(events.Query, "Denver") || strings.Contains(events.Query, "Portland") || len(events.Items) != 1 {
		t.Errorf("kid's events = %+v, want one event in Denver", events)
	}

	// The kid sees the change everywhere their profile shows...
	var kidNow family.Me
	kid.do("GET", "/api/me", nil, http.StatusOK, &kidNow)
	if kidNow.Profile != want || kid.homepage().Profile != layout.Profile(want) {
		t.Errorf("kid's own profile = %+v, want %+v", kidNow.Profile, want)
	}
	var kidsArticles widgets.CuratedWidget
	kid.do("GET", "/api/widgets/articles", nil, http.StatusOK, &kidsArticles)
	if kidsArticles.Query != articles.Query {
		t.Errorf("kid's own articles query = %q, want %q", kidsArticles.Query, articles.Query)
	}

	// ...while alice's own page, profile and cards haven't changed.
	var aliceNow family.Me
	alice.do("GET", "/api/me", nil, http.StatusOK, &aliceNow)
	if aliceNow.Profile != (family.MeProfile{Interests: "rockets", Location: "Portland"}) {
		t.Errorf("alice's profile = %+v, want rockets in Portland still", aliceNow.Profile)
	}
	var alicesArticles widgets.CuratedWidget
	alice.do("GET", "/api/widgets/articles", nil, http.StatusOK, &alicesArticles)
	if alicesArticles.Query != "rockets" {
		t.Errorf("alice's articles query = %q, want rockets", alicesArticles.Query)
	}

	// A parent may name themselves: the same page as /api/me/homepage.
	var self layout.Homepage
	alice.do("GET", memberPath(aliceMe.User.ID, "/homepage"), nil, http.StatusOK, &self)
	mine := alice.homepage()
	if !slices.Equal(cards(self.Layout), cards(mine.Layout)) || len(self.CustomWidgets) != len(mine.CustomWidgets) || self.Profile != mine.Profile {
		t.Errorf("alice via her member id = %+v, want her own homepage %+v", self, mine)
	}

	// A co-parent who joins later can do the same.
	co := newClient(t, srv)
	co.accept(tokenOf(alice.invite(aliceMe.Family.ID, "co@example.com", "parent", http.StatusCreated)))
	co.do("GET", memberPath(kidMe.User.ID, "/homepage"), nil, http.StatusOK, &seen)
	if seen.Profile != layout.Profile(want) {
		t.Errorf("co-parent sees the kid's profile as %+v, want %+v", seen.Profile, want)
	}
}

// Parents can look at a member's page and set their interests, but never
// rearrange it or change its widgets: there are no routes for that.
func TestParentCannotChangeMemberLayoutOrWidgets(t *testing.T) {
	f := newTestFamily(t)
	alice, kid, kidMe := f.alice, f.kid, f.kidMe
	before := cards(kid.homepage().Layout)

	// The path exists for GET only, so a PUT is 405 Method Not Allowed.
	alice.do("PUT", memberPath(kidMe.User.ID, "/homepage"), map[string]any{"layout": blocks(card("Articles", "articles", 4))},
		http.StatusMethodNotAllowed, nil)
	alice.do("POST", memberPath(kidMe.User.ID, "/widgets/from-prompt"), map[string]string{"prompt": "show me upcoming SpaceX launches"},
		http.StatusNotFound, nil)

	page := kid.homepage()
	if got := cards(page.Layout); !slices.Equal(got, before) || len(page.CustomWidgets) != 0 {
		t.Errorf("kid's page after alice's attempts = %q with %d widgets, want %q and none", got, len(page.CustomWidgets), before)
	}
}

func TestMemberRoutesRefuse(t *testing.T) {
	f := newTestFamily(t)
	srv, alice, kid, aliceMe, kidMe := f.srv, f.alice, f.kid, f.aliceMe, f.kidMe
	// Bob is a parent too, in a family of his own.
	bob := newClient(t, srv)
	bobMe := bob.signup("bob@example.com")
	bob.setUpFamily()

	routes := []struct{ method, rest string }{
		{"GET", "/homepage"},
		{"PUT", "/profile"},
		{"GET", "/widgets/articles"},
		{"GET", "/widgets/events"},
	}
	profile := map[string]string{"interests": "planted", "location": "Nowhere"}
	const kidsOnly = "Only parents can see other family members' pages."
	const missing = "That family member doesn't exist."

	tests := []struct {
		name       string
		who        *client
		id         string
		wantStatus int
		wantMsg    string
	}{
		{"a kid, about a parent", kid, fmt.Sprint(aliceMe.User.ID), http.StatusForbidden, kidsOnly},
		// Kids are turned away before the id is even looked at, so a kid's
		// own id is refused too (their own page is under /api/me), and a
		// kid can't probe which ids exist.
		{"a kid, about themselves", kid, fmt.Sprint(kidMe.User.ID), http.StatusForbidden, kidsOnly},
		{"a kid, about nobody", kid, "999", http.StatusForbidden, kidsOnly},
		{"another family's parent", bob, fmt.Sprint(kidMe.User.ID), http.StatusNotFound, missing},
		{"a parent, about another family", alice, fmt.Sprint(bobMe.User.ID), http.StatusNotFound, missing},
		{"a parent, about nobody", alice, "999", http.StatusNotFound, missing},
		{"a parent, about id 0", alice, "0", http.StatusNotFound, missing},
		{"a parent, with a word for an id", alice, "kid", http.StatusNotFound, missing},
	}
	for _, tt := range tests {
		for _, route := range routes {
			t.Run(tt.name+" "+route.method+" "+route.rest, func(t *testing.T) {
				var body any
				if route.method == "PUT" {
					body = profile
				}
				tt.who.wantError(route.method, "/api/members/"+tt.id+route.rest, body, tt.wantStatus, tt.wantMsg)
			})
		}
	}

	// None of the refused PUTs changed anyone's profile.
	for _, c := range []*client{alice, kid, bob} {
		var me family.Me
		c.do("GET", "/api/me", nil, http.StatusOK, &me)
		if me.Profile.Interests == "planted" {
			t.Errorf("%s's profile = %+v, changed by a refused request", me.User.Email, me.Profile)
		}
	}
}

func TestMemberRoutesRejectCrossSite(t *testing.T) {
	f := newTestFamily(t)
	srv, alice, kid, kidMe := f.srv, f.alice, f.kid, f.kidMe

	send := func(method, path, body string, header map[string]string) int {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range header {
			req.Header.Set(k, v)
		}
		resp, err := alice.http.Do(req) // alice's jar sends her session cookie
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// Another site can't set a member's interests with alice's cookie...
	put := send("PUT", memberPath(kidMe.User.ID, "/profile"), `{"interests":"planted","location":""}`,
		map[string]string{"Content-Type": "application/json", "Origin": "http://localhost:5173"})
	if put != http.StatusForbidden {
		t.Errorf("cross-site PUT: status = %d, want 403", put)
	}
	// ...or spend her credit on a member's curated cards.
	get := send("GET", memberPath(kidMe.User.ID, "/widgets/articles"), "", map[string]string{"Sec-Fetch-Site": "cross-site"})
	if get != http.StatusForbidden {
		t.Errorf("cross-site GET: status = %d, want 403", get)
	}

	var me family.Me
	kid.do("GET", "/api/me", nil, http.StatusOK, &me)
	if me.Profile.Interests != "" {
		t.Errorf("kid's interests = %q after a cross-site PUT, want unchanged", me.Profile.Interests)
	}
}
