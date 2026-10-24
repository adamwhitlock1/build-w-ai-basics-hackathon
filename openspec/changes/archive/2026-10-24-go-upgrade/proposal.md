## Why

The backend is built with go1.22.0. That release lost security support in February 2025 and never received a single 1.22.x patch, so the Go vulnerability database lists 82 standard-library advisories that apply to it, among them HTTP/2 crashes and a Range-header memory exhaustion in `net/http`. v2 also needs a newer toolchain: Genkit for Go (v1.14.0), the planned agent framework, requires Go 1.25 or newer. Upgrading first, on its own, keeps the security fix and the framework change apart, so if anything breaks we know which change did it.

## What Changes

- **Install go1.27.3**, the newest stable release (2026-10-19), using the official macOS installer (`.pkg`). It replaces the existing official install in `/usr/local/go`. Homebrew Go is deliberately not used: `/opt/homebrew/bin` comes earlier in `PATH` and would silently shadow `/usr/local/go`. Installing is a step the user runs, since it needs an administrator password.
- **`backend/go.mod` moves to `go 1.27.0`**, with no `toolchain` line. The Makefile keeps `GOTOOLCHAIN=local`, so a machine still on an old Go fails loudly instead of downloading a toolchain through go1.22.0's vulnerable `cmd/go` (GO-2026-6602).
- **`CGO_ENABLED=0` stays, but now by choice.** Go 1.24+ fixed the macOS `LC_UUID` crash that first forced it. The build checks that a cgo binary now runs, then documents the real reason to keep it off: Porchlight and its SQLite driver are pure Go, so the result is a single static binary that cross-compiles to Linux for the homelab.
- **Dependencies are refreshed one at a time**, each with the test suite green before the next:
  - `modernc.org/sqlite` v1.36.1 → v1.61.0, with `modernc.org/libc` pinned to exactly v1.79.0, as the driver requires;
  - `github.com/alexedwards/scs/v2` v2.8.0 → v2.9.0 (it needs Go 1.23+);
  - `golang.org/x/crypto` v0.33.0 → v0.58.0;
  - `github.com/arran4/golang-ical` v0.3.7 → v0.3.8.

  `go-openai`, `godotenv`, `rrule-go` and the `scs/sqlite3store` pseudo-version are already the latest and stay as they are.
- **Code is modernized with `go fix`** (Go's built-in modernizers), reviewed by hand, with no behavior changes.
- **`govulncheck` is added** through the `go.mod` `tool` directive and runs in `make check`. A known vulnerability that the code can actually reach then fails the check.
- **The docs are updated:** the Makefile comment, and the README's Go prerequisite and its "Why `make run`" section.
- **No product behavior changes.** No API, schema or frontend changes, and every existing test stays green unchanged.

## Capabilities

### New Capabilities

None. This change is build tooling and dependency maintenance only.

### Modified Capabilities

None. No product requirement changes, so this change sets `skip_specs: true`.

## Impact

- **Developer machine:** Go at `/usr/local/go` moves from 1.22.0 to 1.27.3. Any other project on this Mac that relied on go1.22.0 gets 1.27.3 too; Go 1 compatibility makes that almost always fine.
- **Backend:**
  - `backend/go.mod`, `backend/go.sum` and `backend/Makefile` change.
  - The `store` DSN is checked against the new driver, which validates its parameters since v1.55.0.
  - `go fix` touches source files mechanically.
- **Tooling:** `golang.org/x/vuln/cmd/govulncheck` becomes a module tool (`go tool govulncheck`), and `make check` gains a vulnerability step.
- **Docs:** `README.md` and the Makefile comment are updated. On archive, one line goes in "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
- **Unblocks** `genkit-integration` (needs Go 1.25+), and gives `v2-foundation` newer standard-library tools to use: `http.CrossOriginProtection`, `testing/synctest` and `os.Root`.
