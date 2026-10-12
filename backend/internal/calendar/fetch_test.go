package calendar

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPublicAddress(t *testing.T) {
	tests := map[string]bool{
		"8.8.8.8":              true,
		"142.250.72.14":        true, // a Google address
		"2607:f8b0:4005::200e": true,
		"127.0.0.1":            false,
		"::1":                  false,
		"10.0.0.5":             false,
		"172.16.3.4":           false,
		"192.168.1.10":         false,
		"169.254.169.254":      false, // cloud metadata
		"fe80::1":              false,
		"fc00::1":              false,
		"100.64.0.1":           false, // carrier-grade NAT
		"0.0.0.0":              false,
		"0.1.2.3":              false,
		"::":                   false,
		"224.0.0.1":            false,
		"255.255.255.255":      false,
		"::ffff:127.0.0.1":     false, // IPv4-mapped loopback
		"::ffff:10.1.2.3":      false,
		"64:ff9b::a00:5":       false, // NAT64 for 10.0.0.5
	}
	for addr, want := range tests {
		if got := publicAddress(netip.MustParseAddr(addr)); got != want {
			t.Errorf("publicAddress(%s) = %v, want %v", addr, got, want)
		}
	}
}

// secretPath stands in for the secret part of a real calendar link: no
// error may contain it.
const secretPath = "/calendar/ical/me/private-SECRET123/basic.ics"

const tinyFeed = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"

// testServer serves h over TLS on loopback, and returns a fetcher that
// trusts it and may connect to loopback, plus one that may not.
func testServer(t *testing.T, h http.HandlerFunc) (srv *httptest.Server, loopbackOK, strict *Fetcher) {
	t.Helper()
	srv = httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	tlsConfig := &tls.Config{RootCAs: pool}
	allowAll := func(netip.Addr) bool { return true }
	return srv, newFetcher(allowAll, tlsConfig), newFetcher(publicAddress, tlsConfig)
}

func TestFetch(t *testing.T) {
	srv, loopbackOK, strict := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		w.Write([]byte("\xef\xbb\xbf" + tinyFeed)) // with a byte-order mark, as some services send
	})
	link := srv.URL + secretPath

	feed, err := loopbackOK.Fetch(context.Background(), link)
	if err != nil || !strings.Contains(string(feed), "BEGIN:VCALENDAR") {
		t.Fatalf("Fetch = %q, %v; want the feed", feed, err)
	}

	// The real check refuses loopback, both by IP and by host name.
	for _, l := range []string{link, strings.Replace(link, "127.0.0.1", "localhost", 1)} {
		_, err := strict.Fetch(context.Background(), l)
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("strict Fetch(%s) error = %v, want ErrBlockedAddress", l, err)
		}
		if err != nil && strings.Contains(err.Error(), "SECRET123") {
			t.Errorf("error leaks the link: %v", err)
		}
	}
}

func TestFetchWebcal(t *testing.T) {
	srv, loopbackOK, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(tinyFeed)) })
	if _, err := loopbackOK.Fetch(context.Background(), strings.Replace(srv.URL, "https://", "webcal://", 1)+secretPath); err != nil {
		t.Errorf("webcal Fetch error = %v", err)
	}
}

func TestFetchErrors(t *testing.T) {
	big := tinyFeed + strings.Repeat("X", maxFeedBytes)
	var hops atomic.Int32
	var plainHits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plainHits.Add(1)
		w.Write([]byte(tinyFeed))
	}))
	t.Cleanup(plain.Close)

	srv, fetcher, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("case") {
		case "404":
			http.NotFound(w, r)
		case "410":
			w.WriteHeader(http.StatusGone)
		case "403":
			w.WriteHeader(http.StatusForbidden)
		case "500":
			w.WriteHeader(http.StatusInternalServerError)
		case "html":
			w.Write([]byte("<!doctype html><html><body>Sign in</body></html>"))
		case "big":
			w.Write([]byte(big))
		case "to-http":
			http.Redirect(w, r, plain.URL+"/cal.ics", http.StatusFound)
		case "loop":
			hops.Add(1)
			http.Redirect(w, r, r.URL.String(), http.StatusFound) // forever
		case "three":
			// Three redirects, then the feed: allowed.
			if n := hops.Add(1); n <= 3 {
				http.Redirect(w, r, r.URL.String(), http.StatusFound)
				return
			}
			w.Write([]byte(tinyFeed))
		}
	})

	tests := []struct {
		name string
		want error // nil = success
	}{
		{"404", ErrLinkGone},
		{"410", ErrLinkGone},
		{"403", ErrLinkGone},
		{"500", ErrUnavailable},
		{"html", ErrNotCalendar},
		{"big", ErrUnavailable},
		{"to-http", ErrUnavailable},
		{"loop", ErrUnavailable},
		{"three", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hops.Store(0)
			_, err := fetcher.Fetch(context.Background(), srv.URL+secretPath+"?case="+tt.name)
			switch {
			case tt.want == nil && err != nil:
				t.Fatalf("error = %v, want success", err)
			case tt.want != nil && !errors.Is(err, tt.want):
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), "SECRET123") {
				t.Errorf("error leaks the link: %v", err)
			}
		})
	}
	if plainHits.Load() != 0 {
		t.Error("followed a redirect to http://")
	}
	if n := hops.Load(); n > maxRedirects+1 {
		t.Errorf("followed %d redirects, want at most %d", n-1, maxRedirects)
	}
}

func TestFetchRejectsBadLinks(t *testing.T) {
	for _, link := range []string{"http://example.com/cal.ics", "file:///etc/passwd", "nonsense"} {
		if _, err := NewFetcher().Fetch(context.Background(), link); !errors.Is(err, ErrInvalidLink) {
			t.Errorf("Fetch(%q) error = %v, want ErrInvalidLink", link, err)
		}
	}
}
