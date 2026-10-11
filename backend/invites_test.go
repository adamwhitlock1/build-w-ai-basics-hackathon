package main

// End-to-end tests of the family endpoints: GET /api/me/family, creating
// and revoking invites, and accepting an invite link. They reuse the server
// and client helpers in routes_test.go and homepage_test.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"porchlight/internal/family"
	"porchlight/internal/store"
)

func invitesPath(familyID int64) string {
	return fmt.Sprintf("/api/families/%d/invites", familyID)
}

// tokenOf returns the token from an invite's "/invite/<token>" link.
func tokenOf(inv family.PendingInvite) string {
	return strings.TrimPrefix(inv.URL, "/invite/")
}

// invite has c invite email into familyID as role ("parent" or "kid"),
// expecting wantStatus, and returns the invite.
func (c *client) invite(familyID int64, email, role string, wantStatus int) family.PendingInvite {
	c.t.Helper()
	var resp family.InviteResponse
	c.do("POST", invitesPath(familyID), map[string]string{"email": email, "role": role}, wantStatus, &resp)
	return resp.Invite
}

// accept has c accept the invite with token, choosing the password
// "kid password", and returns the new member's Me.
func (c *client) accept(token string) family.Me {
	c.t.Helper()
	var me family.Me
	c.do("POST", "/api/invites/"+token+"/accept", map[string]string{"password": "kid password"}, http.StatusCreated, &me)
	return me
}

// myFamily fetches c's GET /api/me/family.
func (c *client) myFamily() family.FamilyResponse {
	c.t.Helper()
	var resp family.FamilyResponse
	c.do("GET", "/api/me/family", nil, http.StatusOK, &resp)
	return resp
}

// memberList describes members as short strings like "kid@example.com
// kid you", so whole lists compare at a glance.
func memberList(members []family.Member) []string {
	out := []string{}
	for _, m := range members {
		s := m.Email + " kid"
		if m.IsAdmin {
			s = m.Email + " parent"
		}
		if m.IsYou {
			s += " you"
		}
		out = append(out, s)
	}
	return out
}

// setUpFamilyWith runs the create-family intro, inviting coAdmins.
func (c *client) setUpFamilyWith(coAdmins ...string) family.Me {
	c.t.Helper()
	var me family.Me
	c.do("POST", "/api/families", map[string]any{"name": "The Robinsons", "coAdminEmails": coAdmins}, http.StatusOK, &me)
	return me
}

func TestFamilyListing(t *testing.T) {
	srv, _ := newTestServer(t)
	alice := newClient(t, srv)
	alice.signup("alice@example.com")
	me := alice.setUpFamilyWith("co@example.com")

	// The co-parent from the intro is a pending parent invite with a link.
	fam := alice.myFamily()
	if fam.Family != me.Family {
		t.Errorf("family = %+v, want %+v", fam.Family, me.Family)
	}
	if got, want := memberList(fam.Members), []string{"alice@example.com parent you"}; !slices.Equal(got, want) {
		t.Errorf("members = %q, want %q", got, want)
	}
	if len(fam.PendingInvites) != 1 {
		t.Fatalf("pending invites = %+v, want the co-parent's", fam.PendingInvites)
	}
	co := fam.PendingInvites[0]
	if co.Email != "co@example.com" || !co.IsAdmin || len(tokenOf(co)) != 43 || !strings.HasPrefix(co.URL, "/invite/") {
		t.Errorf("co-parent invite = %+v, want a parent invite with an /invite/<token> link", co)
	}
	if until := time.Until(co.ExpiresAt); until < 13*24*time.Hour || until > 15*24*time.Hour {
		t.Errorf("co-parent invite expires in %v, want about 14 days", until)
	}

	// The kid joins first, then the co-parent: parents are listed first.
	kidInvite := alice.invite(me.Family.ID, "kid@example.com", "kid", http.StatusCreated)
	kid := newClient(t, srv)
	kid.accept(tokenOf(kidInvite))
	coClient := newClient(t, srv)
	coMe := coClient.accept(tokenOf(co))
	if !coMe.User.IsAdmin || coMe.Family != me.Family {
		t.Errorf("co-parent's Me = %+v, want an admin in alice's family", coMe)
	}

	fam = alice.myFamily()
	want := []string{"alice@example.com parent you", "co@example.com parent", "kid@example.com kid"}
	if got := memberList(fam.Members); !slices.Equal(got, want) {
		t.Errorf("alice sees members %q, want %q", got, want)
	}
	if len(fam.PendingInvites) != 0 {
		t.Errorf("pending after both joined = %+v, want none", fam.PendingInvites)
	}
	// The co-parent is an admin too, so sees the whole family.
	want = []string{"alice@example.com parent", "co@example.com parent you", "kid@example.com kid"}
	if got := memberList(coClient.myFamily().Members); !slices.Equal(got, want) {
		t.Errorf("co-parent sees members %q, want %q", got, want)
	}

	// A kid sees only themselves, and no invites: [] in the JSON, not null.
	resp, err := kid.http.Get(srv.URL + "/api/me/family")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&raw)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw["pendingInvites"]) != "[]" {
		t.Errorf("kid's pendingInvites = %s, want []", raw["pendingInvites"])
	}
	if got, want := memberList(kid.myFamily().Members), []string{"kid@example.com kid you"}; !slices.Equal(got, want) {
		t.Errorf("kid sees members %q, want %q", got, want)
	}

	// Someone in another family sees only their own.
	bob := newClient(t, srv)
	bob.signup("bob@example.com")
	bobs := bob.myFamily()
	if bobs.Family.ID == me.Family.ID || len(bobs.Members) != 1 || len(bobs.PendingInvites) != 0 {
		t.Errorf("bob's family = %+v, want only his own", bobs)
	}
}

func TestCreateInviteRules(t *testing.T) {
	srv, _ := newTestServer(t)
	alice := newClient(t, srv)
	alice.signup("alice@example.com")
	me := alice.setUpFamily()
	kid := newClient(t, srv)
	kid.accept(tokenOf(alice.invite(me.Family.ID, "kid@example.com", "kid", http.StatusCreated)))
	bob := newClient(t, srv)
	bobsMe := bob.signup("bob@example.com")
	bob.setUpFamily()
	carol := newClient(t, srv) // hasn't done the create-family intro yet
	carolsMe := carol.signup("carol@example.com")

	tests := []struct {
		name       string
		who        *client
		familyID   int64
		body       any
		wantStatus int
		wantMsg    string
	}{
		{"already a member", alice, me.Family.ID, map[string]string{"email": " KID@example.com ", "role": "kid"}, http.StatusConflict, "Already a member."},
		{"inviting yourself", alice, me.Family.ID, map[string]string{"email": "alice@example.com", "role": "parent"}, http.StatusConflict, "Already a member."},
		{"account in another family", alice, me.Family.ID, map[string]string{"email": "bob@example.com", "role": "parent"}, http.StatusConflict, "That email already has a Porchlight account."},
		{"a kid inviting", kid, me.Family.ID, map[string]string{"email": "friend@example.com", "role": "kid"}, http.StatusForbidden, "Only parents can invite family members."},
		{"another family's id", alice, bobsMe.Family.ID, map[string]string{"email": "new@example.com", "role": "kid"}, http.StatusNotFound, "That family doesn't exist."},
		{"into alice's family as bob", bob, me.Family.ID, map[string]string{"email": "new@example.com", "role": "kid"}, http.StatusNotFound, "That family doesn't exist."},
		{"family id 0", alice, 0, map[string]string{"email": "new@example.com", "role": "kid"}, http.StatusNotFound, "That family doesn't exist."},
		{"family not set up yet", carol, carolsMe.Family.ID, map[string]string{"email": "new@example.com", "role": "kid"}, http.StatusConflict, "Finish setting up your family first."},
		{"not an email", alice, me.Family.ID, map[string]string{"email": "new.example.com", "role": "kid"}, http.StatusBadRequest, "That doesn't look like an email address."},
		{"no role", alice, me.Family.ID, map[string]string{"email": "new@example.com"}, http.StatusBadRequest, "Choose whether they're a parent or a kid."},
		{"unknown role", alice, me.Family.ID, map[string]string{"email": "new@example.com", "role": "admin"}, http.StatusBadRequest, "Choose whether they're a parent or a kid."},
		{"not JSON", alice, me.Family.ID, "email=new@example.com", http.StatusBadRequest, `Send JSON like {"email": "kid@example.com", "role": "kid"}.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.who.wantError("POST", invitesPath(tt.familyID), tt.body, tt.wantStatus, tt.wantMsg)
		})
	}
	// None of those made an invite.
	if pending := alice.myFamily().PendingInvites; len(pending) != 0 {
		t.Errorf("pending after refused invites = %+v, want none", pending)
	}

	// Inviting the same email again gives back the same invite (200, not
	// 201), and a different role updates it rather than adding another.
	first := alice.invite(me.Family.ID, "new@example.com", "kid", http.StatusCreated)
	if first.Email != "new@example.com" || first.IsAdmin || first.ID == 0 {
		t.Errorf("new invite = %+v, want a kid invite", first)
	}
	again := alice.invite(me.Family.ID, " NEW@Example.com", "kid", http.StatusOK)
	if again != first {
		t.Errorf("inviting again = %+v, want the same invite %+v", again, first)
	}
	asParent := alice.invite(me.Family.ID, "new@example.com", "Parent", http.StatusOK)
	if asParent.ID != first.ID || asParent.URL != first.URL || !asParent.IsAdmin {
		t.Errorf("inviting as a parent = %+v, want invite %d with the same link, now a parent", asParent, first.ID)
	}
	pending := alice.myFamily().PendingInvites
	if len(pending) != 1 || pending[0].ID != first.ID || !pending[0].IsAdmin {
		t.Errorf("pending = %+v, want only invite %d, as a parent", pending, first.ID)
	}
}

// The create-family intro applies the sidebar's rule to its co-admins: an
// email that already has an account can't be invited, since that invite
// could never be accepted.
func TestFamilySetupRefusesCoAdminWithAccount(t *testing.T) {
	srv, _ := newTestServer(t)
	newClient(t, srv).signup("bob@example.com") // bob has a family of his own
	alice := newClient(t, srv)
	alice.signup("alice@example.com")

	body := map[string]any{"name": "The Robinsons", "coAdminEmails": []string{"co@example.com", "Bob@example.com"}}
	alice.wantError("POST", "/api/families", body, http.StatusBadRequest, "bob@example.com already has a Porchlight account.")

	// Nothing was saved, so the intro is still to do...
	var me family.Me
	alice.do("GET", "/api/me", nil, http.StatusOK, &me)
	if !me.NeedsFamilySetup {
		t.Errorf("after the refused setup, Me = %+v, want the intro still to do", me)
	}
	// ...and without bob it goes through, with only co@'s invite pending.
	alice.setUpFamilyWith("co@example.com")
	pending := alice.myFamily().PendingInvites
	if len(pending) != 1 || pending[0].Email != "co@example.com" {
		t.Errorf("pending = %+v, want only co@example.com's invite", pending)
	}
}

func TestRevokeInvite(t *testing.T) {
	srv, _ := newTestServer(t)
	alice := newClient(t, srv)
	alice.signup("alice@example.com")
	me := alice.setUpFamilyWith("co@example.com")
	co := alice.myFamily().PendingInvites[0]
	kidInvite := alice.invite(me.Family.ID, "kid@example.com", "kid", http.StatusCreated)
	kid := newClient(t, srv)
	kid.accept(tokenOf(kidInvite))
	bob := newClient(t, srv)
	bobsMe := bob.signup("bob@example.com")

	coPath := fmt.Sprintf("%s/%d", invitesPath(me.Family.ID), co.ID)
	tests := []struct {
		name       string
		who        *client
		path       string
		wantStatus int
		wantMsg    string
	}{
		{"a kid", kid, coPath, http.StatusForbidden, "Only parents can manage invites."},
		{"another family's admin", bob, coPath, http.StatusNotFound, "That family doesn't exist."},
		{"through another family's id", bob, fmt.Sprintf("%s/%d", invitesPath(bobsMe.Family.ID), co.ID), http.StatusNotFound, "That invite doesn't exist."},
		{"an accepted invite", alice, fmt.Sprintf("%s/%d", invitesPath(me.Family.ID), kidInvite.ID), http.StatusNotFound, "That invite doesn't exist."},
		{"no such invite", alice, invitesPath(me.Family.ID) + "/999", http.StatusNotFound, "That invite doesn't exist."},
		{"not an invite id", alice, invitesPath(me.Family.ID) + "/abc", http.StatusNotFound, "That invite doesn't exist."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.who.wantError("DELETE", tt.path, nil, tt.wantStatus, tt.wantMsg)
		})
	}

	// The co-parent's link still works after all that...
	anon := newClient(t, srv)
	anon.do("GET", "/api/invites/"+tokenOf(co), nil, http.StatusOK, nil)

	// ...until a parent revokes it. Then it's as if it never existed.
	alice.do("DELETE", coPath, nil, http.StatusNoContent, nil)
	anon.wantError("GET", "/api/invites/"+tokenOf(co), nil, http.StatusNotFound, "That invite link doesn't work.")
	anon.wantError("POST", "/api/invites/"+tokenOf(co)+"/accept", map[string]string{"password": "kid password"}, http.StatusNotFound, "That invite link doesn't work.")
	alice.wantError("DELETE", coPath, nil, http.StatusNotFound, "That invite doesn't exist.")
	if pending := alice.myFamily().PendingInvites; len(pending) != 0 {
		t.Errorf("pending after revoking = %+v, want none", pending)
	}
}

func TestShowInvite(t *testing.T) {
	srv, st := newTestServer(t)
	alice := newClient(t, srv)
	alice.signup("alice@example.com")
	me := alice.setUpFamily()
	kidInvite := alice.invite(me.Family.ID, "kid@example.com", "kid", http.StatusCreated)
	coInvite := alice.invite(me.Family.ID, "co@example.com", "parent", http.StatusCreated)
	used := alice.invite(me.Family.ID, "used@example.com", "kid", http.StatusCreated)
	newClient(t, srv).accept(tokenOf(used))
	// An invite that ran out yesterday, put straight into the database.
	if _, err := st.CreateInvite(context.Background(), store.Invite{
		FamilyID: me.Family.ID, Email: "late@example.com", Token: "expired-token", ExpiresAt: time.Now().Add(-24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// The invite page is public: no session needed.
	anon := newClient(t, srv)
	for _, tt := range []struct {
		inv  family.PendingInvite
		want family.InviteDetails
	}{
		{kidInvite, family.InviteDetails{Email: "kid@example.com", FamilyName: "The Robinsons", IsAdmin: false}},
		{coInvite, family.InviteDetails{Email: "co@example.com", FamilyName: "The Robinsons", IsAdmin: true}},
	} {
		var got family.InviteDetails
		anon.do("GET", "/api/invites/"+tokenOf(tt.inv), nil, http.StatusOK, &got)
		if got != tt.want {
			t.Errorf("GET invite for %s = %+v, want %+v", tt.inv.Email, got, tt.want)
		}
	}

	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantMsg    string
	}{
		{"unknown token", "not-a-real-token", http.StatusNotFound, "That invite link doesn't work."},
		{"expired", "expired-token", http.StatusGone, "That invite has expired or was already used."},
		{"already accepted", tokenOf(used), http.StatusGone, "That invite has expired or was already used."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anon.wantError("GET", "/api/invites/"+tt.token, nil, tt.wantStatus, tt.wantMsg)
			anon.wantError("POST", "/api/invites/"+tt.token+"/accept", map[string]string{"password": "kid password"}, tt.wantStatus, tt.wantMsg)
		})
	}
	// The expired invite isn't listed as pending, either.
	for _, inv := range alice.myFamily().PendingInvites {
		if inv.Email == "late@example.com" || inv.Email == "used@example.com" {
			t.Errorf("pending invites include %s, which can't be accepted", inv.Email)
		}
	}
}

func TestAcceptInvite(t *testing.T) {
	srv, st := newTestServer(t)
	alice := newClient(t, srv)
	aliceMe := alice.signup("alice@example.com")
	alice.setUpFamily()
	inv := alice.invite(aliceMe.Family.ID, "kid@example.com", "kid", http.StatusCreated)
	token := tokenOf(inv)

	kid := newClient(t, srv) // a fresh browser, logged out
	kid.wantError("GET", "/api/me", nil, http.StatusUnauthorized, "Please log in.")
	kid.wantError("POST", "/api/invites/"+token+"/accept", map[string]string{"password": "short"}, http.StatusBadRequest, "Your password needs at least 8 characters.")
	kid.wantError("POST", "/api/invites/"+token+"/accept", "password=kid", http.StatusBadRequest, `Send JSON like {"password": "..."}.`)

	me := kid.accept(token)
	want := family.Me{
		User:    family.MeUser{ID: me.User.ID, Email: "kid@example.com", IsAdmin: false},
		Family:  family.MeFamily{ID: aliceMe.Family.ID, Name: "The Robinsons"},
		Profile: family.MeProfile{},
	}
	if me.User.ID == 0 || me != want {
		t.Errorf("accept = %+v, want %+v", me, want)
	}

	// The kid is now logged in, as themselves.
	var again family.Me
	kid.do("GET", "/api/me", nil, http.StatusOK, &again)
	if again != me {
		t.Errorf("GET /api/me = %+v, want %+v", again, me)
	}
	if _, err := st.GetMemberProfile(context.Background(), me.User.ID); err != nil {
		t.Errorf("no member profile for the new kid: %v", err)
	}

	// Their homepage is the default one, with nothing of Alice's on it.
	alice.addWidget()
	page := kid.homepage()
	if got, want := cards(page.Layout), []string{"Articles articles 2", "LocalEvents events 2"}; !slices.Equal(got, want) {
		t.Errorf("kid's layout = %q, want the default %q", got, want)
	}
	if len(page.CustomWidgets) != 0 || page.Profile.Interests != "" {
		t.Errorf("kid's homepage = %+v, want an empty profile and no widgets", page)
	}

	// The link is used up, even with the right password.
	newClient(t, srv).wantError("POST", "/api/invites/"+token+"/accept", map[string]string{"password": "kid password"},
		http.StatusGone, "That invite has expired or was already used.")
	// The password they chose works for logging in.
	newClient(t, srv).do("POST", "/api/login", creds{"kid@example.com", "kid password"}, http.StatusOK, nil)
}

func TestAcceptInviteSwitchesSession(t *testing.T) {
	srv, _ := newTestServer(t)
	alice := newClient(t, srv)
	aliceMe := alice.signup("alice@example.com")
	alice.setUpFamily()
	inv := alice.invite(aliceMe.Family.ID, "co@example.com", "parent", http.StatusCreated)

	// Bob is logged in when he opens the co-parent's link on his computer.
	bob := newClient(t, srv)
	bob.signup("bob@example.com")
	bobsToken := bob.sessionToken()

	me := bob.accept(tokenOf(inv))
	if me.User.Email != "co@example.com" || !me.User.IsAdmin {
		t.Errorf("accept = %+v, want the co-parent, an admin", me.User)
	}
	var now family.Me
	bob.do("GET", "/api/me", nil, http.StatusOK, &now)
	if now.User.Email != "co@example.com" {
		t.Errorf("after accepting, logged in as %q, want co@example.com", now.User.Email)
	}
	// Accepting renewed the session token, so the old one is dead.
	if got := meStatusWithToken(t, srv, bobsToken); got != http.StatusUnauthorized {
		t.Errorf("GET /api/me with the token from before: status = %d, want 401", got)
	}
}

func TestAcceptInviteEmailTakenMeanwhile(t *testing.T) {
	srv, _ := newTestServer(t)
	alice := newClient(t, srv)
	aliceMe := alice.signup("alice@example.com")
	alice.setUpFamily()
	inv := alice.invite(aliceMe.Family.ID, "kid@example.com", "kid", http.StatusCreated)

	// Before opening the link, the kid signs up on their own.
	kid := newClient(t, srv)
	kid.signup("kid@example.com")

	kid.wantError("POST", "/api/invites/"+tokenOf(inv)+"/accept", map[string]string{"password": "kid password"},
		http.StatusConflict, "That email already has a Porchlight account.")
	// Nothing changed: still logged in as their own account, and the
	// invite is still pending for Alice to revoke.
	var me family.Me
	kid.do("GET", "/api/me", nil, http.StatusOK, &me)
	if me.Family.ID == aliceMe.Family.ID || !me.User.IsAdmin {
		t.Errorf("kid's account = %+v, want the one they signed up with", me)
	}
	if pending := alice.myFamily().PendingInvites; len(pending) != 1 || pending[0].ID != inv.ID {
		t.Errorf("pending = %+v, want invite %d still there", pending, inv.ID)
	}
}
