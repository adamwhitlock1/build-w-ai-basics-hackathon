package calendar

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeLink(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "  https://calendar.google.com/calendar/ical/abc%40group/private-123/basic.ics ", want: "https://calendar.google.com/calendar/ical/abc%40group/private-123/basic.ics"},
		{in: "webcal://p42-caldav.icloud.com/published/2/XYZ", want: "https://p42-caldav.icloud.com/published/2/XYZ"},
		{in: "WEBCAL://example.com/cal.ics", want: "https://example.com/cal.ics"},
		{in: "http://example.com/cal.ics", wantErr: true},
		{in: "ftp://example.com/cal.ics", wantErr: true},
		{in: "calendar.google.com/basic.ics", wantErr: true},
		{in: "https://user:pass@example.com/cal.ics", wantErr: true},
		{in: "https:///nohost", wantErr: true},
		{in: "", wantErr: true},
		{in: "https://example.com/" + strings.Repeat("a", maxLinkLength), wantErr: true},
	}
	for _, tt := range tests {
		got, err := NormalizeLink(tt.in)
		switch {
		case tt.wantErr && !errors.Is(err, ErrInvalidLink):
			t.Errorf("NormalizeLink(%.40q) error = %v, want ErrInvalidLink", tt.in, err)
		case !tt.wantErr && (err != nil || got != tt.want):
			t.Errorf("NormalizeLink(%.40q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestProvider(t *testing.T) {
	tests := map[string]string{
		"https://calendar.google.com/calendar/ical/x/private-y/basic.ics":  "Google Calendar",
		"https://p42-caldav.icloud.com/published/2/XYZ":                    "iCloud",
		"https://outlook.office365.com/owa/calendar/abc/reachcalendar.ics": "Outlook",
		"https://outlook.live.com/owa/calendar/abc/calendar.ics":           "Outlook",
		"https://school.example.org/events.ics":                            "school.example.org",
	}
	for link, want := range tests {
		if got := Provider(link); got != want {
			t.Errorf("Provider(%q) = %q, want %q", link, got, want)
		}
	}
}
