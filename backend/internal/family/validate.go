package family

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"kinlight/internal/auth"
)

// minPasswordRunes is the shortest password we accept.
const minPasswordRunes = 8

// maxEmailBytes is the longest email address the standards allow.
const maxEmailBytes = 254

// normalizeEmail trims and lowercases s, and reports whether the result
// looks like an email address: one "@", something before it, and a dot
// after it with something on both sides. That catches typos; whether the
// address really works is only known when someone uses it.
func normalizeEmail(s string) (email string, ok bool) {
	email = strings.ToLower(strings.TrimSpace(s))
	if len(email) > maxEmailBytes || strings.ContainsAny(email, " \t\r\n") {
		return email, false
	}
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || strings.Contains(domain, "@") {
		return email, false
	}
	dot := strings.LastIndex(domain, ".")
	return email, dot > 0 && dot < len(domain)-1
}

// cleanEmails normalizes a list of emails, skipping blanks, duplicates, and
// own (the caller's own address). If an entry isn't a valid email, it
// returns that entry as bad.
func cleanEmails(raw []string, own string) (emails []string, bad string) {
	seen := map[string]bool{own: true}
	for _, s := range raw {
		if strings.TrimSpace(s) == "" {
			continue
		}
		email, ok := normalizeEmail(s)
		if !ok {
			return nil, strings.TrimSpace(s)
		}
		if seen[email] {
			continue
		}
		seen[email] = true
		emails = append(emails, email)
	}
	return emails, ""
}

// cleanProfile trims interests and location and checks their lengths. If
// one is too long, it returns a message and hint for the member instead.
func cleanProfile(interests, location string) (p MeProfile, msg, hint string) {
	p = MeProfile{Interests: strings.TrimSpace(interests), Location: strings.TrimSpace(location)}
	switch {
	case utf8.RuneCountInString(p.Interests) > maxProfileRunes:
		return p, "That's a lot of interests!", fmt.Sprintf("Keep them under %d characters.", maxProfileRunes)
	case utf8.RuneCountInString(p.Location) > maxProfileRunes:
		return p, "That location is too long.", "A city is plenty."
	}
	return p, "", ""
}

// passwordProblem returns a message and hint explaining why p can't be a
// password, or empty strings if it's fine.
func passwordProblem(p string) (msg, hint string) {
	switch {
	case utf8.RuneCountInString(p) < minPasswordRunes:
		return fmt.Sprintf("Your password needs at least %d characters.", minPasswordRunes), "A short phrase is easy to remember."
	case len(p) > auth.MaxPasswordBytes:
		return "That password is too long.", fmt.Sprintf("Use %d characters or fewer.", auth.MaxPasswordBytes)
	}
	return "", ""
}

// newInviteToken returns 32 random bytes from crypto/rand, encoded as
// unpadded base64url so it can go straight into a link.
func newInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
