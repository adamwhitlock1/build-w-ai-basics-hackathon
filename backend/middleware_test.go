package main

import "testing"

// Invite tokens are secrets, so request logs show "<token>" instead.
func TestLoggedPath(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/api/invites/abc123", "/api/invites/<token>"},
		{"/api/invites/abc123/accept", "/api/invites/<token>/accept"},
		{"/api/invites/", "/api/invites/"},
		{"/api/me/family", "/api/me/family"},
		{"/api/families/1/invites/2", "/api/families/1/invites/2"}, // an id, not a secret
	}
	for _, tt := range tests {
		if got := loggedPath(tt.path); got != tt.want {
			t.Errorf("loggedPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
