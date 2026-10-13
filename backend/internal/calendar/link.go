// Package calendar reads a member's calendar from its private iCal (ICS)
// link: it fetches the feed safely, parses it, and lists the events that
// fall on a given day, repeating ones included.
//
// A calendar link is a secret (anyone holding it can read the calendar), so
// nothing this package returns, including its errors, contains the link.
package calendar

import (
	"errors"
	"net/url"
	"strings"
)

var (
	// ErrInvalidLink is returned for a link that isn't an https:// or
	// webcal:// address.
	ErrInvalidLink = errors.New("not an https or webcal link")
	// ErrNotCalendar is returned when a link answers with something other
	// than a calendar.
	ErrNotCalendar = errors.New("not a calendar")
	// ErrLinkGone is returned when the calendar service says the link no
	// longer exists or is no longer allowed (HTTP 401, 403, 404 or 410),
	// e.g. after its secret address was reset.
	ErrLinkGone = errors.New("calendar link no longer works")
	// ErrUnavailable wraps every other failure to get the feed: an HTTP
	// error, a timeout, a feed that's too big, too many redirects.
	ErrUnavailable = errors.New("calendar unavailable")
	// ErrBlockedAddress is returned when the link leads to a private or
	// local network address, which Porchlight never connects to.
	ErrBlockedAddress = errors.New("calendar link points to a blocked address")
)

// maxLinkLength caps a pasted link. Real ones are a few hundred characters.
const maxLinkLength = 2000

// NormalizeLink checks a pasted calendar link and returns it as an https://
// URL. webcal:// is how calendar apps label subscription links; it's the
// same address over https.
func NormalizeLink(raw string) (string, error) {
	link := strings.TrimSpace(raw)
	if link == "" || len(link) > maxLinkLength {
		return "", ErrInvalidLink
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", ErrInvalidLink
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "webcal":
		u.Scheme = "https"
	default:
		return "", ErrInvalidLink
	}
	if u.Hostname() == "" || u.User != nil {
		return "", ErrInvalidLink
	}
	return u.String(), nil
}

// Provider names the service a calendar link comes from, for people:
// "Google Calendar", "iCloud", "Outlook", or else the link's host name.
// It's safe to show; the rest of the link is not.
func Provider(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "calendar.google.com" || strings.HasSuffix(host, ".google.com"):
		return "Google Calendar"
	case host == "icloud.com" || strings.HasSuffix(host, ".icloud.com"):
		return "iCloud"
	case strings.HasPrefix(host, "outlook.") || strings.HasSuffix(host, ".outlook.com"):
		return "Outlook"
	}
	return host
}

// Summary is everything a browser may know about a member's calendar link:
// whether there is one, and which service it's from. Never the link.
type Summary struct {
	Connected bool   `json:"connected"`
	Provider  string `json:"provider"` // "" when not connected
}

// Summarize describes link ("" for none) for a browser.
func Summarize(link string) Summary {
	if link == "" {
		return Summary{}
	}
	return Summary{Connected: true, Provider: Provider(link)}
}
