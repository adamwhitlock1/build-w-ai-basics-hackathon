package main

// These tests run the whole API (real routes, real SQLite store, real scs
// sessions) through an httptest server, with fakes standing in only for the
// LLM and the search API. Each client has its own cookie jar, like a browser.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/sqlite3store"

	"porchlight/internal/auth"
	"porchlight/internal/family"
	"porchlight/internal/layout"
	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// fakePlanner always plans the same widget.
type fakePlanner struct{}

func (fakePlanner) PickWidgetSpec(context.Context, string, llm.Catalog) (llm.WidgetSpec, error) {
	return llm.WidgetSpec{Title: "SpaceX Launches", Source: "events_search", Query: "upcoming SpaceX launches", Archetype: llm.ArchetypeList}, nil
}

func (fakePlanner) ShapeItems(context.Context, string, llm.WidgetSpec, []sources.Result) ([]llm.Item, error) {
	return []llm.Item{{Title: "Starlink 10-12", URL: "https://example.com/launch", Date: "Oct 12"}}, nil
}

// fakeSource returns one canned search result.
type fakeSource struct{}

func (fakeSource) Name() string  { return "events_search" }
func (fakeSource) Shape() string { return "test results" }
func (fakeSource) Search(context.Context, string) ([]sources.Result, error) {
	return []sources.Result{{Title: "Launch schedule", URL: "https://example.com/launch"}}, nil
}

// startServer serves the API backed by the database at dbPath.
func startServer(t *testing.T, dbPath string) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// A cleanup interval of 0 means no background goroutine to stop.
	sessions := auth.NewSessionManager(sqlite3store.NewWithCleanupInterval(st.DB(), 0), logger)
	srv := httptest.NewServer(routes(logger, st, sessions, fakePlanner{}, sources.NewRegistry(fakeSource{})))
	// Cleanups run last-in, first-out: the server stops before the store closes.
	t.Cleanup(func() { st.Close() })
	t.Cleanup(srv.Close)
	return srv, st
}

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	return startServer(t, filepath.Join(t.TempDir(), "test.db"))
}

// client is a browser stand-in: it keeps cookies between requests.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, srv *httptest.Server) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}
}

// do sends a request with body encoded as JSON (if not nil), fails the test
// unless the response has wantStatus, and decodes the response into out (if
// not nil). It returns the response, whose body has been read.
func (c *client) do(method, path string, body any, wantStatus int, out any) *http.Response {
	c.t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, reqBody)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	if resp.StatusCode != wantStatus {
		c.t.Fatalf("%s %s: status = %d, want %d; body: %s", method, path, resp.StatusCode, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode %s: %v", method, path, raw, err)
		}
	}
	return resp
}

// wantError checks an error response body.
func (c *client) wantError(method, path string, body any, wantStatus int, wantMsg string) {
	c.t.Helper()
	var got respond.ErrorBody
	c.do(method, path, body, wantStatus, &got)
	if got.Error != wantMsg {
		c.t.Errorf("%s %s: error = %q, want %q", method, path, got.Error, wantMsg)
	}
}

type creds struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// sessionToken returns the session cookie's value from the client's jar.
func (c *client) sessionToken() string {
	c.t.Helper()
	u, err := url.Parse(c.base)
	if err != nil {
		c.t.Fatal(err)
	}
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == auth.CookieName {
			return ck.Value
		}
	}
	c.t.Fatalf("no %s cookie in the jar", auth.CookieName)
	return ""
}

// meStatusWithToken sends GET /api/me with exactly this session token,
// bypassing any cookie jar (as someone replaying an old cookie would), and
// returns the status code.
func meStatusWithToken(t *testing.T, srv *httptest.Server, token string) int {
	t.Helper()
	req, err := http.NewRequest("GET", srv.URL+"/api/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	resp, err := srv.Client().Do(req) // srv.Client has no cookie jar
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func (c *client) signup(email string) family.Me {
	c.t.Helper()
	var me family.Me
	c.do("POST", "/api/signup", creds{email, "correct horse"}, http.StatusCreated, &me)
	return me
}

func (c *client) setUpFamily() family.Me {
	c.t.Helper()
	var me family.Me
	c.do("POST", "/api/families", map[string]any{"name": "The Robinsons", "interests": "rockets", "location": "Portland"}, http.StatusOK, &me)
	return me
}

func TestSignupMeLogout(t *testing.T) {
	srv, _ := newTestServer(t)
	c := newClient(t, srv)

	me := c.signup("  Parent@Example.com ")
	if me.User.Email != "parent@example.com" || !me.User.IsAdmin || me.User.ID == 0 {
		t.Errorf("user = %+v, want an admin with the normalized email", me.User)
	}
	if !me.NeedsFamilySetup || me.Family.ID == 0 || me.Family.Name != "" {
		t.Errorf("family = %+v, needsFamilySetup = %v; want an unnamed family that needs setup", me.Family, me.NeedsFamilySetup)
	}

	var again family.Me
	c.do("GET", "/api/me", nil, http.StatusOK, &again)
	if again != me {
		t.Errorf("GET /api/me = %+v, want %+v", again, me)
	}

	token := c.sessionToken()
	resp := c.do("POST", "/api/logout", nil, http.StatusNoContent, nil)
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "porchlight_session=;") {
		t.Errorf("logout Set-Cookie = %q, want the session cookie cleared", resp.Header.Get("Set-Cookie"))
	}
	c.wantError("GET", "/api/me", nil, http.StatusUnauthorized, "Please log in.")
	// The jar dropped the cookie, so also replay the old token by hand:
	// logging out must end the session on the server, not just in the browser.
	if got := meStatusWithToken(t, srv, token); got != http.StatusUnauthorized {
		t.Errorf("GET /api/me with the logged-out token: status = %d, want 401", got)
	}

	// Logging out again, already logged out, is fine.
	c.do("POST", "/api/logout", nil, http.StatusNoContent, nil)
}

func TestSessionCookie(t *testing.T) {
	srv, _ := newTestServer(t)
	c := newClient(t, srv)

	resp := c.do("POST", "/api/signup", creds{"parent@example.com", "correct horse"}, http.StatusCreated, nil)
	var cookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == auth.CookieName {
			cookie = ck
		}
	}
	if cookie == nil {
		t.Fatalf("no %s cookie in %v", auth.CookieName, resp.Header["Set-Cookie"])
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Secure || cookie.Path != "/" || cookie.Domain != "" {
		t.Errorf("cookie = %+v, want HttpOnly, SameSite=Lax, not Secure, Path=/, no Domain", cookie)
	}
	if lifetime := time.Duration(cookie.MaxAge) * time.Second; lifetime < 29*24*time.Hour {
		t.Errorf("cookie Max-Age = %v, want about 30 days", lifetime)
	}

	// Logging in issues a new session token rather than reusing the old
	// one, and the old one stops working.
	resp = c.do("POST", "/api/login", creds{"parent@example.com", "correct horse"}, http.StatusOK, nil)
	for _, ck := range resp.Cookies() {
		if ck.Name == auth.CookieName && ck.Value == cookie.Value {
			t.Error("login kept the same session token; want a fresh one")
		}
	}
	if got := meStatusWithToken(t, srv, cookie.Value); got != http.StatusUnauthorized {
		t.Errorf("GET /api/me with the pre-login token: status = %d, want 401", got)
	}
	if got := meStatusWithToken(t, srv, c.sessionToken()); got != http.StatusOK {
		t.Errorf("GET /api/me with the new token: status = %d, want 200", got)
	}
}

func TestLoginWhenSessionCannotBeSaved(t *testing.T) {
	srv, st := newTestServer(t)
	newClient(t, srv).signup("parent@example.com")

	// From here on every session save fails, as it would on a full disk.
	if _, err := st.DB().Exec(`CREATE TRIGGER no_new_sessions BEFORE INSERT ON sessions
		BEGIN SELECT RAISE(ABORT, 'sessions are read-only in this test'); END`); err != nil {
		t.Fatal(err)
	}

	// wantError decodes the body as a single JSON value, so these also check
	// that exactly one well-formed error comes back.
	c := newClient(t, srv)
	c.wantError("POST", "/api/signup", creds{"new@example.com", "correct horse"}, http.StatusInternalServerError,
		"Your account was created, but we couldn't log you in.")
	c.wantError("POST", "/api/login", creds{"parent@example.com", "correct horse"}, http.StatusInternalServerError,
		"Something went wrong on our side.")
	c.wantError("GET", "/api/me", nil, http.StatusUnauthorized, "Please log in.")
}

func TestCrossSiteWritesRejected(t *testing.T) {
	srv, _ := newTestServer(t)
	c := newClient(t, srv) // its cookie jar sends the session cookie, as a browser would
	c.signup("parent@example.com")

	tests := []struct {
		name        string
		origin      string // "" sends no Origin header
		contentType string
		wantStatus  int
	}{
		{"another localhost port", "http://localhost:5173", "application/json", http.StatusForbidden},
		{"another site", "https://example.net", "application/json", http.StatusForbidden},
		{"a text/plain form from another site", "http://localhost:5173", "text/plain;charset=UTF-8", http.StatusForbidden},
		{"a text/plain body, no Origin", "", "text/plain", http.StatusUnsupportedMediaType},
		{"a form-encoded body, no Origin", "", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		// Last, because it succeeds: had any request above got through, the
		// family would already be set up and this would get a 409.
		{"the app itself", srv.URL, "application/json; charset=utf-8", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", srv.URL+"/api/families", strings.NewReader(`{"name":"Planted Family"}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", tt.contentType)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			resp, err := c.http.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var body respond.ErrorBody
			_ = json.NewDecoder(resp.Body).Decode(&body)
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d (%q), want %d", resp.StatusCode, body.Error, tt.wantStatus)
			}
		})
	}

	// A body-less POST (logout) from another site is refused too.
	req, err := http.NewRequest("POST", srv.URL+"/api/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site logout: status = %d, want 403", resp.StatusCode)
	}
	c.do("GET", "/api/me", nil, http.StatusOK, nil) // still logged in
}

func TestSignupRejects(t *testing.T) {
	srv, _ := newTestServer(t)
	newClient(t, srv).signup("taken@example.com")

	tests := []struct {
		name       string
		body       any
		wantStatus int
		wantMsg    string
	}{
		{"not an email", creds{"parent.example.com", "correct horse"}, http.StatusBadRequest, "That doesn't look like an email address."},
		{"short password", creds{"new@example.com", "short"}, http.StatusBadRequest, "Your password needs at least 8 characters."},
		{"email taken", creds{"TAKEN@example.com", "correct horse"}, http.StatusConflict, "That email already has an account."},
		{"not JSON", "email=x", http.StatusBadRequest, `Send JSON like {"email": "you@example.com", "password": "..."}.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(t, srv)
			c.wantError("POST", "/api/signup", tt.body, tt.wantStatus, tt.wantMsg)
			// A failed signup doesn't log anyone in.
			c.wantError("GET", "/api/me", nil, http.StatusUnauthorized, "Please log in.")
		})
	}
}

func TestLogin(t *testing.T) {
	srv, _ := newTestServer(t)
	signedUp := newClient(t, srv).signup("parent@example.com")

	tests := []struct {
		name       string
		creds      creds
		wantStatus int
	}{
		{"right password", creds{"parent@example.com", "correct horse"}, http.StatusOK},
		{"email in another case", creds{" PARENT@example.com", "correct horse"}, http.StatusOK},
		{"wrong password", creds{"parent@example.com", "wrong horse"}, http.StatusUnauthorized},
		{"unknown email", creds{"nobody@example.com", "correct horse"}, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(t, srv)
			if tt.wantStatus == http.StatusUnauthorized {
				// Same message either way, so it doesn't reveal which emails exist.
				c.wantError("POST", "/api/login", tt.creds, tt.wantStatus, "That email and password don't match.")
				c.wantError("GET", "/api/me", nil, http.StatusUnauthorized, "Please log in.")
				return
			}
			var me family.Me
			c.do("POST", "/api/login", tt.creds, tt.wantStatus, &me)
			if me != signedUp {
				t.Errorf("login = %+v, want %+v", me, signedUp)
			}
			c.do("GET", "/api/me", nil, http.StatusOK, nil)
		})
	}
}

func TestCreateFamily(t *testing.T) {
	srv, st := newTestServer(t)
	c := newClient(t, srv)
	c.signup("parent@example.com")

	// Bad input is rejected before anything is saved.
	c.wantError("POST", "/api/families", map[string]any{"name": "  "}, http.StatusBadRequest, "Give your family a name.")
	c.wantError("POST", "/api/families", map[string]any{"name": "The Robinsons", "coAdminEmails": []string{"nope"}},
		http.StatusBadRequest, `"nope" doesn't look like an email address.`)

	var me family.Me
	c.do("POST", "/api/families", map[string]any{
		"name":          " The Robinsons ",
		"coAdminEmails": []string{"co@example.com", " CO@example.com", "parent@example.com", "", "second@example.com"},
		"interests":     "rockets, gardening",
		"location":      "Portland, OR",
	}, http.StatusOK, &me)
	if me.NeedsFamilySetup || me.Family.Name != "The Robinsons" {
		t.Errorf("family = %+v, needsFamilySetup = %v; want it named and set up", me.Family, me.NeedsFamilySetup)
	}
	if me.Profile != (family.MeProfile{Interests: "rockets, gardening", Location: "Portland, OR"}) {
		t.Errorf("profile = %+v", me.Profile)
	}

	// One admin invite per distinct co-admin, skipping the parent's own email.
	invites, err := st.ListPendingInvites(context.Background(), me.Family.ID)
	if err != nil {
		t.Fatal(err)
	}
	var emails []string
	for _, inv := range invites {
		emails = append(emails, inv.Email)
		if !inv.IsAdmin || len(inv.Token) != 43 { // 32 bytes of base64url
			t.Errorf("invite = %+v, want an admin invite with a 43-character token", inv)
		}
		if until := time.Until(inv.ExpiresAt); until < 13*24*time.Hour || until > 15*24*time.Hour {
			t.Errorf("invite expires in %v, want about 14 days", until)
		}
	}
	if strings.Join(emails, ",") != "co@example.com,second@example.com" {
		t.Errorf("invited %v, want [co@example.com second@example.com]", emails)
	}

	var got family.Me
	c.do("GET", "/api/me", nil, http.StatusOK, &got)
	if got != me {
		t.Errorf("GET /api/me = %+v, want %+v", got, me)
	}

	c.wantError("POST", "/api/families", map[string]any{"name": "Again"}, http.StatusConflict, "Your family is already set up.")
}

func TestWidgetsAreSavedPerUser(t *testing.T) {
	srv, _ := newTestServer(t)
	alice := newClient(t, srv)
	alice.signup("alice@example.com")
	alice.setUpFamily()

	var widget widgets.SavedWidget
	alice.do("POST", "/api/widgets/from-prompt", map[string]string{"prompt": "show me upcoming SpaceX launches"}, http.StatusCreated, &widget)
	if widget.ID == 0 || widget.CreatedAt.IsZero() || widget.Title != "SpaceX Launches" || len(widget.Items) != 1 {
		t.Errorf("new widget = %+v", widget)
	}

	var page layout.Homepage
	alice.do("GET", "/api/me/homepage", nil, http.StatusOK, &page)
	if len(page.CustomWidgets) != 1 || page.CustomWidgets[0].ID != widget.ID || page.CustomWidgets[0].Title != widget.Title {
		t.Errorf("homepage widgets = %+v, want [%d]", page.CustomWidgets, widget.ID)
	}
	if page.Profile != (layout.Profile{Interests: "rockets", Location: "Portland"}) {
		t.Errorf("homepage profile = %+v", page.Profile)
	}
	if string(page.Layout) != "null" {
		t.Errorf("layout = %s, want null", page.Layout)
	}

	path := "/api/widgets/custom/" + strconv.FormatInt(widget.ID, 10)
	var got widgets.SavedWidget
	alice.do("GET", path, nil, http.StatusOK, &got)
	if got.ID != widget.ID || got.Title != widget.Title || !got.CreatedAt.Equal(widget.CreatedAt) {
		t.Errorf("GET %s = %+v, want %+v", path, got, widget)
	}

	// Bob, in another family, can't see Alice's widget and has none of his own.
	bob := newClient(t, srv)
	bob.signup("bob@example.com")
	bob.wantError("GET", path, nil, http.StatusNotFound, "That widget doesn't exist.")
	var bobsPage struct {
		CustomWidgets []widgets.SavedWidget `json:"customWidgets"`
	}
	bob.do("GET", "/api/me/homepage", nil, http.StatusOK, &bobsPage)
	if bobsPage.CustomWidgets == nil || len(bobsPage.CustomWidgets) != 0 {
		t.Errorf("bob's widgets = %#v, want an empty array", bobsPage.CustomWidgets)
	}
}

func TestPrivateRoutesNeedLogin(t *testing.T) {
	srv, _ := newTestServer(t)
	private := []struct{ method, path string }{
		{"GET", "/api/me"},
		{"GET", "/api/me/homepage"},
		{"POST", "/api/families"},
		{"POST", "/api/widgets/from-prompt"},
		{"GET", "/api/widgets/custom/1"},
	}
	for _, r := range private {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			newClient(t, srv).wantError(r.method, r.path, map[string]string{"prompt": "hi", "name": "x"}, http.StatusUnauthorized, "Please log in.")
		})
	}

	newClient(t, srv).do("GET", "/api/health", nil, http.StatusOK, nil)
}

func TestDataSurvivesRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	first, _ := startServer(t, dbPath)
	c := newClient(t, first)
	c.signup("parent@example.com")
	c.setUpFamily()
	c.do("POST", "/api/widgets/from-prompt", map[string]string{"prompt": "show me upcoming SpaceX launches"}, http.StatusCreated, nil)
	first.Close()

	second, _ := startServer(t, dbPath)
	c.base = second.URL // same cookie jar: 127.0.0.1 on a new port shares cookies

	// The old session lives in the database too, so it still works...
	c.do("GET", "/api/me", nil, http.StatusOK, nil)
	// ...and logging in again finds the account, the family, and the widget.
	var me family.Me
	c.do("POST", "/api/login", creds{"parent@example.com", "correct horse"}, http.StatusOK, &me)
	if me.Family.Name != "The Robinsons" || me.NeedsFamilySetup {
		t.Errorf("after restart: family = %+v", me.Family)
	}
	var page layout.Homepage
	c.do("GET", "/api/me/homepage", nil, http.StatusOK, &page)
	if len(page.CustomWidgets) != 1 {
		t.Errorf("after restart: %d widgets, want 1", len(page.CustomWidgets))
	}
}
