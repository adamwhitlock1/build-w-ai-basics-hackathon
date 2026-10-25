package web_test

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"kinlight/internal/web"
)

const page = `<!doctype html>
<html><head>
<script type="module" src="/assets/index-abc.js"></script>
<script>
  ;(function () { document.documentElement.dataset.theme = 'light' })()
</script>
</head><body><div id="root"></div></body></html>`

func TestInlineScriptHashes(t *testing.T) {
	body := "\n  ;(function () { document.documentElement.dataset.theme = 'light' })()\n"
	sum := sha256.Sum256([]byte(body))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	got := web.InlineScriptHashes([]byte(page))
	if len(got) != 1 || got[0] != want {
		t.Fatalf("hashes = %v, want only the inline script's, %s", got, want)
	}
	if !strings.Contains(web.ContentSecurityPolicy(got), "script-src 'self' "+want) {
		t.Errorf("policy doesn't allow the hash: %s", web.ContentSecurityPolicy(got))
	}
}

func TestInlineScriptHashesIgnoresExternalAndEmpty(t *testing.T) {
	got := web.InlineScriptHashes([]byte(`<script src="/a.js"></script><script> </script>`))
	if len(got) != 0 {
		t.Errorf("hashes = %v, want none", got)
	}
}
