package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"porchlight/internal/calendar"
	"porchlight/internal/layout"
	"porchlight/internal/store"
)

func TestSeedDemo(t *testing.T) {
	const secret = "https://calendar.google.com/calendar/ical/maya/private-SECRET123/basic.ics"
	for _, name := range []string{"MAYA", "SAM", "JORDAN", "ALEX"} {
		t.Setenv("SEED_CALENDAR_"+name, "")
	}
	t.Setenv("SEED_CALENDAR_MAYA", secret)

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.demo")
	var out bytes.Buffer
	if err := seedDemo(ctx, dbPath, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "SECRET123") {
		t.Errorf("the output contains the calendar link:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "from SEED_CALENDAR_MAYA") {
		t.Errorf("the output doesn't say Maya's calendar came from the environment:\n%s", out.String())
	}

	// Running it again starts over rather than failing on existing accounts.
	if err := seedDemo(ctx, dbPath, &bytes.Buffer{}); err != nil {
		t.Fatalf("second run: %v", err)
	}

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	user := func(email string) store.User {
		t.Helper()
		u, err := st.GetUserByEmail(ctx, email)
		if err != nil {
			t.Fatalf("%s: %v", email, err)
		}
		return u
	}
	maya := user("maya@rivera.test")
	alex := user("alex@chen.test")
	if maya.FamilyID == alex.FamilyID {
		t.Error("the Riveras and the Chens are one family")
	}

	// Members, roles and calendars.
	wantCalendars := map[string]string{
		"maya@rivera.test":   secret,
		"sam@rivera.test":    calendar.FixtureLink("sam"),
		"jordan@rivera.test": calendar.FixtureLink("jordan"),
		"lily@rivera.test":   "",
		"alex@chen.test":     calendar.FixtureLink("alex"),
		"riley@chen.test":    "",
	}
	parents := map[string]bool{"maya@rivera.test": true, "sam@rivera.test": true, "alex@chen.test": true}
	var count int
	for _, fam := range []int64{maya.FamilyID, alex.FamilyID} {
		members, err := st.ListFamilyMembers(ctx, fam)
		if err != nil {
			t.Fatal(err)
		}
		count += len(members)
		for _, m := range members {
			if m.IsAdmin != parents[m.Email] {
				t.Errorf("%s: parent = %v, want %v", m.Email, m.IsAdmin, parents[m.Email])
			}
			link, err := st.GetCalendarLink(ctx, m.ID)
			if err != nil {
				t.Fatal(err)
			}
			if want, ok := wantCalendars[m.Email]; !ok || link != want {
				t.Errorf("%s: calendar = %q, want %q", m.Email, link, want)
			}
		}
	}
	if count != 6 {
		t.Errorf("%d users, want 6", count)
	}

	// Grandma's invite is pending, as a parent.
	invites, err := st.ListPendingInvites(ctx, maya.FamilyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(invites) != 1 || invites[0].Email != "grandma@rivera.test" || !invites[0].IsAdmin {
		t.Errorf("pending invites = %+v, want Grandma's, as a parent", invites)
	}

	// Maya's page is a custom arrangement of all three widgets.
	widgetList, err := st.ListCustomWidgets(ctx, maya.ID)
	if err != nil {
		t.Fatal(err)
	}
	var widgetIDs []int64
	for _, w := range widgetList {
		widgetIDs = append(widgetIDs, w.ID)
	}
	raw, err := st.GetLayout(ctx, maya.ID)
	if err != nil {
		t.Fatalf("Maya's layout: %v", err)
	}
	var data layout.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	var onPage []int64
	for _, b := range data.Root.Props.Blocks {
		if b.Type == layout.TypeCustom {
			onPage = append(onPage, b.Props.WidgetID)
		}
	}
	slices.Sort(onPage)
	if len(widgetIDs) != 3 || !slices.Equal(onPage, widgetIDs) {
		t.Errorf("Maya's page shows widgets %v, want all of %v", onPage, widgetIDs)
	}
	if _, err := st.GetLayout(ctx, user("sam@rivera.test").ID); err == nil {
		t.Error("Sam has a saved layout; want the default page")
	}

	// Kids' ages and sample chats: Jordan's two, Lily's one with a flag to
	// review, no crisis, and none for the Chen kid.
	thisYear := time.Now().Year()
	for email, wantAge := range map[string]int{"jordan@rivera.test": 15, "lily@rivera.test": 9} {
		year, ok, err := st.GetBirthYear(ctx, user(email).ID)
		if err != nil || !ok || thisYear-year != wantAge {
			t.Errorf("%s: birth year %d (set %v, %v), want age %d", email, year, ok, err, wantAge)
		}
	}
	for email, want := range map[string]int{"jordan@rivera.test": 2, "lily@rivera.test": 1, "riley@chen.test": 0} {
		u := user(email)
		chats, err := st.ListConversations(ctx, u.FamilyID, u.ID)
		if err != nil || len(chats) != want {
			t.Errorf("%s: %d chats (%v), want %d", email, len(chats), err, want)
		}
	}
	queue, err := st.ReviewQueue(ctx, maya.FamilyID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].MemberID != user("lily@rivera.test").ID || queue[0].Category != "strangers" ||
		queue[0].Severity != "medium" || queue[0].Crisis || !queue[0].ReviewedAt.IsZero() {
		t.Errorf("review queue = %+v, want Lily's one unreviewed medium strangers flag", queue)
	}
	if crises, _ := st.UnreviewedCrises(ctx, maya.FamilyID); len(crises) != 0 {
		t.Errorf("crises = %+v, want none", crises)
	}

	// Everyone has remembered Articles and events.
	for email := range wantCalendars {
		u := user(email)
		for _, kind := range []string{"articles", "events"} {
			if _, err := st.GetCuratedResult(ctx, u.ID, kind); err != nil {
				t.Errorf("%s has no remembered %s: %v", email, kind, err)
			}
		}
	}
}

func TestSeedDemoRefusesARealDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "porchlight.db")
	if err := seed(ctx, dbPath); err != nil {
		t.Fatal(err)
	}
	err := seedDemo(ctx, dbPath, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already has 2 users") {
		t.Fatalf("seedDemo on a database with users = %v, want a refusal", err)
	}
}
