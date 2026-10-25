package calendar

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

const (
	// fetchTimeout bounds one whole fetch, redirects and body included.
	fetchTimeout = 10 * time.Second
	// maxFeedBytes caps a feed. Google feeds hold years of past events, but
	// a family calendar is well under this.
	maxFeedBytes = 5 << 20
	// maxRedirects is how many redirects a fetch follows.
	maxRedirects = 3
)

// Fetcher gets calendar feeds from links members paste. Fetching a URL
// someone typed is how server-side request forgery happens (making the
// server call addresses on its own private network), so every connection
// is checked against the IP actually being dialed, after DNS, and private
// or local addresses are refused.
type Fetcher struct {
	client *http.Client
	// allow says whether the fetcher may connect to an address. It's
	// publicAddress except in tests, which serve feeds from loopback.
	allow func(netip.Addr) bool
}

// NewFetcher returns a Fetcher that only connects to public addresses.
func NewFetcher() *Fetcher {
	return newFetcher(publicAddress, nil)
}

// newFetcher returns a Fetcher that connects only where allow says it may.
// tlsConfig is nil except in tests, which trust their own test server.
func newFetcher(allow func(netip.Addr) bool, tlsConfig *tls.Config) *Fetcher {
	f := &Fetcher{allow: allow}
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: f.checkDial}
	transport := &http.Transport{
		Proxy:               nil, // ignore HTTP_PROXY: a proxy would dial for us, unchecked
		DialContext:         dialer.DialContext,
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: 5 * time.Second,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
	}
	f.client = &http.Client{
		Timeout:       fetchTimeout,
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
	return f
}

// checkDial is the dialer's Control hook: Go calls it with the IP and port
// it is about to connect to, after resolving the host name. Checking here
// rather than the URL's text means a host name that resolves (or
// re-resolves) to a private address is still refused, and so is every
// redirect target.
func (f *Fetcher) checkDial(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !f.allow(ap.Addr()) {
		return ErrBlockedAddress
	}
	return nil
}

// blockedPrefixes are ranges that aren't covered by netip's Is* methods
// but still aren't the public internet.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network"
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved, and broadcast
	netip.MustParsePrefix("64:ff9b::/96"),  // NAT64: can map to any IPv4 address
}

// publicAddress reports whether a is an address on the public internet.
func publicAddress(a netip.Addr) bool {
	a = a.Unmap() // ::ffff:127.0.0.1 is 127.0.0.1
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsUnspecified() ||
		a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() || a.IsMulticast() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// checkRedirect allows at most maxRedirects redirects, all to https.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > maxRedirects {
		return fmt.Errorf("%w: more than %d redirects", ErrUnavailable, maxRedirects)
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("%w: redirected to a non-https address", ErrUnavailable)
	}
	return nil
}

// Fetch gets the feed at link and checks that it is a calendar. The errors
// it returns never contain the link: they wrap ErrInvalidLink,
// ErrBlockedAddress, ErrLinkGone, ErrNotCalendar or ErrUnavailable.
func (f *Fetcher) Fetch(ctx context.Context, link string) ([]byte, error) {
	link, err := NormalizeLink(link)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, ErrInvalidLink
	}
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	req.Header.Set("User-Agent", "Kinlight/1.0 (family homepage calendar reader)")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, withoutLink(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("%w: HTTP %d", ErrLinkGone, resp.StatusCode)
	default:
		return nil, fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	}

	// Read one byte past the cap, to tell "exactly 5 MB" from "more".
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, withoutLink(err)
	}
	if len(body) > maxFeedBytes {
		return nil, fmt.Errorf("%w: feed is over %d MB", ErrUnavailable, maxFeedBytes>>20)
	}
	if !looksLikeCalendar(body) {
		return nil, ErrNotCalendar
	}
	return body, nil
}

// withoutLink turns an error from the HTTP client into one of ours. The
// client's errors are *url.Error values whose message starts with the URL,
// so the wrapper is dropped and only what went wrong is kept.
func withoutLink(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		err = ue.Err
	}
	switch {
	case errors.Is(err, ErrBlockedAddress):
		return ErrBlockedAddress
	case errors.Is(err, ErrUnavailable):
		return err // from checkRedirect, already ours and link-free
	}
	return fmt.Errorf("%w: %w", ErrUnavailable, err)
}

// looksLikeCalendar reports whether body starts like an iCalendar file,
// after any byte-order mark and blank space.
func looksLikeCalendar(body []byte) bool {
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
	body = bytes.TrimLeft(body, " \t\r\n")
	const begin = "BEGIN:VCALENDAR"
	return len(body) >= len(begin) && bytes.EqualFold(body[:len(begin)], []byte(begin))
}
