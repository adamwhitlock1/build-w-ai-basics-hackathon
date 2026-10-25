package main

// End-to-end tests of widget freshness: remembered curated widgets, and
// refreshing described ones, through the real routes and database.

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"kinlight/internal/widgets"
)

func TestDescribedWidgetFreshness(t *testing.T) {
	srv, st := newTestServer(t)
	c := newClient(t, srv)
	c.signup("parent@example.com")
	c.setUpFamily()
	w := c.addWidget()

	// Just built: fresh, dated now.
	page := c.homepage()
	if len(page.CustomWidgets) != 1 || page.CustomWidgets[0].Stale || page.CustomWidgets[0].UpdatedAt.IsZero() {
		t.Fatalf("new widget on the homepage = %+v, want fresh with an update time", page.CustomWidgets)
	}

	// Seven hours later it's stale.
	if _, err := st.DB().Exec(`UPDATE custom_widgets SET created_at = datetime(created_at, '-7 hours') WHERE id = ?`, w.ID); err != nil {
		t.Fatal(err)
	}
	stale := c.homepage().CustomWidgets[0]
	if !stale.Stale {
		t.Errorf("7-hour-old widget: stale = false, want true")
	}

	// Refreshing it makes it fresh, on the homepage too.
	var refreshed widgets.SavedWidget
	c.do("POST", "/api/widgets/custom/"+strconv.FormatInt(w.ID, 10)+"/refresh", nil, http.StatusOK, &refreshed)
	if refreshed.Stale || !refreshed.UpdatedAt.After(stale.UpdatedAt) || len(refreshed.Items) != 1 {
		t.Errorf("refreshed = %+v, want fresh, newer than %v, with items", refreshed, stale.UpdatedAt)
	}
	if again := c.homepage().CustomWidgets[0]; again.Stale || !again.UpdatedAt.Equal(refreshed.UpdatedAt) {
		t.Errorf("homepage after the refresh = %+v, want it fresh at %v", again, refreshed.UpdatedAt)
	}

	// Nobody else can refresh it.
	other := newClient(t, srv)
	other.signup("stranger@example.com")
	other.do("POST", "/api/widgets/custom/"+strconv.FormatInt(w.ID, 10)+"/refresh", nil, http.StatusNotFound, nil)
}

func TestCuratedWidgetsAreRemembered(t *testing.T) {
	srv, st := newTestServer(t)
	c := newClient(t, srv)
	c.signup("parent@example.com")
	c.setUpFamily()

	var first widgets.CuratedWidget
	c.do("GET", "/api/widgets/articles", nil, http.StatusOK, &first)
	if first.Stale || len(first.Items) == 0 {
		t.Fatalf("first articles = %+v, want fresh items", first)
	}

	// Pretend they were picked an hour ago: the next visit sends them as
	// they are, an hour old and still fresh.
	if _, err := st.DB().Exec(`UPDATE curated_widget_results SET updated_at = datetime(updated_at, '-1 hours') WHERE kind = 'articles'`); err != nil {
		t.Fatal(err)
	}
	var second widgets.CuratedWidget
	c.do("GET", "/api/widgets/articles", nil, http.StatusOK, &second)
	if want := first.UpdatedAt.Add(-time.Hour); !second.UpdatedAt.Equal(want) || second.Stale {
		t.Errorf("second visit: updatedAt %v, stale %v; want the remembered %v, fresh", second.UpdatedAt, second.Stale, want)
	}

	// A refresh picks again.
	var third widgets.CuratedWidget
	c.do("GET", "/api/widgets/articles?refresh=1", nil, http.StatusOK, &third)
	if !third.UpdatedAt.After(second.UpdatedAt) {
		t.Errorf("after a refresh: updatedAt %v, want newer than %v", third.UpdatedAt, second.UpdatedAt)
	}

	// New interests pick again, without being asked.
	c.do("PUT", "/api/me/profile", map[string]string{"interests": "chess", "location": "Portland"}, http.StatusOK, nil)
	var fourth widgets.CuratedWidget
	c.do("GET", "/api/widgets/articles", nil, http.StatusOK, &fourth)
	if len(fourth.Items) == 0 || fourth.Items[0].Title != "News about chess" {
		t.Errorf("after new interests: items %+v, want articles about chess", fourth.Items)
	}
}
