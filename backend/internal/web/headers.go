// Package web is what the server adds to serve Kinlight's web app in
// production: the built frontend itself, and the security headers that go
// with every response.
package web

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"
)

// inlineScript matches a <script> element with no src attribute, capturing
// its exact text: the browser hashes that text, byte for byte.
var inlineScript = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)

// srcAttr finds a src attribute among a script tag's attributes.
var srcAttr = regexp.MustCompile(`(?i)\bsrc\s*=`)

// InlineScriptHashes returns the CSP hash source ('sha256-…') of every
// inline script in indexHTML, so the Content-Security-Policy allows exactly
// those scripts and no others.
func InlineScriptHashes(indexHTML []byte) []string {
	var hashes []string
	for _, m := range inlineScript.FindAllSubmatch(indexHTML, -1) {
		if srcAttr.Match(m[1]) || len(strings.TrimSpace(string(m[2]))) == 0 {
			continue
		}
		sum := sha256.Sum256(m[2])
		hashes = append(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	return hashes
}

// ContentSecurityPolicy allows the app's own files and the given inline
// script hashes. Styles allow inline: React writes style attributes (a
// card's light level, say). Images allow any https site and data: URIs,
// for card thumbnails and the CSS's textures.
func ContentSecurityPolicy(scriptHashes []string) string {
	script := strings.Join(append([]string{"'self'"}, scriptHashes...), " ")
	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + script,
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: https:",
		"font-src 'self'",
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}

// SecurityHeaders sets production's security headers on every response.
func SecurityHeaders(scriptHashes []string, next http.Handler) http.Handler {
	csp := ContentSecurityPolicy(scriptHashes)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("Strict-Transport-Security", "max-age=31536000")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
