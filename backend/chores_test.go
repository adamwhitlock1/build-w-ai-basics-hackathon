package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kinlight/internal/chores"
)

func TestChoreFamilyJourneyAndAuthorization(t *testing.T) {
	var clock atomic.Int64
	clock.Store(time.Date(2026, 10, 26, 5, 59, 59, 0, time.UTC).Unix())
	srv, _ := startServerOpts(t, filepath.Join(t.TempDir(), "db"), func(d *deps) { d.Chores = chores.New(d.Store, func() time.Time { return time.Unix(clock.Load(), 0) }) })
	parent, kid, other, anon := newClient(t, srv), newClient(t, srv), newClient(t, srv), newClient(t, srv)
	p := parent.signup("parent@example.com")
	parent.setUpFamily()
	k := kid.accept(tokenOf(parent.invite(p.Family.ID, "kid@example.com", "kid", 201)))
	o := other.signup("other@example.com")
	kid.do("PUT", "/api/me/homepage", map[string]any{"layout": blocks(card("Articles", "articles", 4))}, 200, nil)
	beforeLayout := kid.homepage().Layout
	base := memberPath(k.User.ID, "/chores")
	var list chores.List
	parent.do("POST", base, map[string]string{"title": "  Feed dog  "}, 201, &list)
	if len(list.Items) != 1 || list.Items[0].Title != "Feed dog" || list.CompletedCount != 0 {
		t.Fatal(list)
	}
	id := list.Items[0].ID
	completion := fmt.Sprintf("/api/me/chores/%d/completion", id)
	for range 2 {
		kid.do("PUT", completion, map[string]any{"date": list.Date, "completed": true}, 200, &list)
	}
	parent.do("PUT", fmt.Sprintf("%s/%d", base, id), map[string]string{"title": "Feed pets"}, 200, &list)
	if !list.Items[0].Completed || list.CompletedCount != 1 {
		t.Fatal(list)
	}
	var summary chores.Summary
	parent.do("GET", "/api/family/chores", nil, 200, &summary)
	if len(summary.Members) != 2 || summary.Members[0].TotalCount != 0 || summary.Members[1].CompletedCount != 1 {
		t.Fatal(summary)
	}
	for _, route := range []struct {
		method, path string
		body         any
	}{
		{"GET", base, nil}, {"POST", base, map[string]string{"title": "No"}}, {"PUT", fmt.Sprintf("%s/%d", base, id), map[string]string{"title": "No"}}, {"DELETE", fmt.Sprintf("%s/%d", base, id), nil}, {"GET", "/api/family/chores", nil},
	} {
		kid.do(route.method, route.path, route.body, 403, nil)
		anon.do(route.method, route.path, route.body, 401, nil)
	}
	for _, member := range []int64{o.User.ID, 999999} {
		parent.do("GET", memberPath(member, "/chores"), nil, 404, nil)
	}
	other.do("PUT", completion, map[string]any{"date": list.Date, "completed": true}, 404, nil)
	parent.do("POST", memberPath(p.User.ID, "/chores"), map[string]string{"title": "Parent chore"}, 201, nil)
	for _, title := range []string{" ", strings.Repeat("a", 121)} {
		parent.do("POST", base, map[string]string{"title": title}, 400, nil)
	}
	kid.do("PUT", completion, map[string]any{"date": list.Date}, 400, nil)
	kid.do("PUT", completion, map[string]any{"date": "bad", "completed": true}, 400, nil)
	parent.do("PUT", fmt.Sprintf("%s/%d/completion", base, id), map[string]any{"date": list.Date, "completed": false}, 200, &list)
	if list.CompletedCount != 0 {
		t.Fatal(list)
	}
	kid.do("PUT", completion, map[string]any{"date": list.Date, "completed": true}, 200, &list)
	clock.Add(1)
	kid.do("PUT", completion, map[string]any{"date": list.Date, "completed": false}, 409, nil)
	kid.do("GET", "/api/me/chores", nil, 200, &list)
	if list.CompletedCount != 0 || list.Date != "2026-10-26" {
		t.Fatal(list)
	}
	parent.do("DELETE", fmt.Sprintf("%s/%d", base, id), nil, 200, &list)
	if list.TotalCount != 0 {
		t.Fatal(list)
	}
	kid.do("PUT", completion, map[string]any{"date": list.Date, "completed": true}, 404, nil)
	for range 30 {
		parent.do("POST", base, map[string]string{"title": "Daily"}, 201, nil)
	}
	parent.do("POST", base, map[string]string{"title": "One too many"}, 409, nil)
	if !reflect.DeepEqual(beforeLayout, kid.homepage().Layout) {
		t.Fatal("chores changed the saved Arrange layout")
	}
	res, err := parent.http.Get(srv.URL + "/api/me/chores")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal(res.Header)
	}
}
