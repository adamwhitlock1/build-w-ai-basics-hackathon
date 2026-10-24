## 1. Install go1.27.3 (run by the user)

- [x] 1.1 Download and verify the official installer, then install it. These steps need an administrator password, so the user runs them:
  ```
  curl -fLo /tmp/go1.27.3.darwin-arm64.pkg https://go.dev/dl/go1.27.3.darwin-arm64.pkg
  shasum -a 256 /tmp/go1.27.3.darwin-arm64.pkg
  # expect fbf88e3b97b66168c34ed859c4a52b795da07dfd806504bdb0b95dd58f8976da
  sudo installer -pkg /tmp/go1.27.3.darwin-arm64.pkg -target /
  ```
  Verify in a **new** terminal: `go version` prints `go1.27.3 darwin/arm64`, `which -a go` prints only `/usr/local/go/bin/go`, and `go env GOROOT` prints `/usr/local/go`.
- [x] 1.2 Check whether cgo binaries now run on macOS 27. In a scratch directory outside the repo, build a tiny program with `CGO_ENABLED=1` that imports `net` (and one that uses `import "C"`), then run both. Verify with `otool -l <binary> | grep LC_UUID`, which shows the load command, and with both programs exiting 0. Record the result (works / still fails, plus the exit code) in this task's notes for the docs in 6.1.
  - **Result (2026-10-24, go1.27.3, macOS 27.0.1):** works.
    - A `CGO_ENABLED=1` program importing `net` has `LC_UUID`, runs and exits 0.
    - An `import "C"` program has `LC_UUID`, runs and exits 0.
    - The same program linked externally (`-ldflags=-linkmode=external`) passes `codesign -v`, runs and exits 0.
    - Both go1.22.0 failures (the `LC_UUID` abort, and the SIGKILL from an invalid signature) are gone.

## 2. Move the module to Go 1.27 (no dependency changes yet)

- [x] 2.1 Set `go 1.27.0` in `backend/go.mod`, with no `toolchain` line, and run `go mod tidy`. Verify that `git diff backend/go.mod` shows only the `go` line (and any tidy-only changes), and that `grep toolchain backend/go.mod` finds nothing.
- [x] 2.2 Run `gofmt -l .`, `make vet` and `make test` on the unchanged code, then `CGO_ENABLED=0 GOTOOLCHAIN=local go test -race ./...`. Verify that all pass. If vet in 1.27 reports anything new, fix it as a separate explained edit; don't suppress it. Commit as "Build the backend with Go 1.27".
- [x] 2.3 Confirm that the old toolchain is refused. Run `GOTOOLCHAIN=local /path/to/an/old/go version` if an old Go is still available, or reason from `go help toolchain`, and record it. Verify that `make check` runs with no network toolchain download: `GOTOOLCHAIN=local` is still exported in the Makefile, and `go env GOTOOLCHAIN` inside `make` prints `local`.
  - **Result:**
    - With go1.22.0 uninstalled, the refusal was shown with a scratch module that needs `go 1.99.0`: `go: go.mod requires go >= 1.99.0 (running go 1.27.3; GOTOOLCHAIN=local)`, exit 1, nothing downloaded.
    - Inside the Makefile, `go env` prints `GOTOOLCHAIN=local CGO_ENABLED=0`.
    - Task 2.2: vet was clean. One whitespace-only gofmt alignment change was applied in `internal/widgets/refresh_test.go`. `go test -race ./...` passed for every package.

## 3. Refresh dependencies one at a time

- [x] 3.1 Upgrade the SQLite driver: `go get modernc.org/sqlite@v1.61.0 modernc.org/libc@v1.79.0`, then `go mod tidy`. Verify that `go list -m modernc.org/libc` prints exactly `v1.79.0`, and that `make test` passes (including `TestOpenOddFileNames` and `TestOpenIsRepeatable`). Commit separately.
- [x] 3.2 Add a store test (e.g. `TestOpenConnectionSettings`) that opens a store and reads back `PRAGMA foreign_keys` (1), `PRAGMA journal_mode` (`wal`) and `PRAGMA busy_timeout` (5000). The test guards the DSN against the driver's newer parameter validation. It also checks that a transaction started through the store can't run alongside another writer. Verify with `go test ./internal/store/` passing on v1.61.0. Commit together with 3.1, or right after it.
- [x] 3.3 Upgrade sessions: `go get github.com/alexedwards/scs/v2@v2.9.0`. Verify with `make test`: the existing cookie assertions in `routes_test.go` (HttpOnly, SameSite=Lax, not Secure, Path=/, no Domain) pass unchanged, and no `Partitioned` attribute appears on the cookie. Commit separately.
- [x] 3.4 Upgrade crypto: `go get golang.org/x/crypto@v0.58.0`. Verify with `make test`; the auth tests cover bcrypt hashing and checking. Commit separately.
- [x] 3.5 Upgrade the calendar parser: `go get github.com/arran4/golang-ical@v0.3.8`. Verify with `go test ./internal/calendar/ ./cmd/seed/` (fixture feeds, recurrences, moved and deleted occurrences) and `make test`. Commit separately.
- [x] 3.6 Confirm that the rest is current: `go list -m -u all` lists no newer version for `go-openai`, `godotenv`, `rrule-go` or `scs/sqlite3store`. Record any that do have one, and leave them for a later change unless the upgrade is trivial and tested. Verify that `go mod verify` prints "all modules verified".
  - **Result:**
    - `go-openai` v1.43.0, `godotenv` v1.5.1, `rrule-go` v1.8.2 and `sqlite3store` show no newer versions.
    - `go list -m -u all` lists newer versions only for modules outside Porchlight's build:
      - `go-sqlite3` is used only by `sqlite3store`'s own tests;
      - `golang.org/x/net` isn't needed by the main module;
      - `testify`, `go-cmp`, `check.v1`, and modernc's `cc`/`ccgo` code generators.

      Nothing to do.
    - `go mod verify`: all modules verified.
    - Task 3.2: the new `TestOpenConnectionSettings` and `TestTransactionsTakeTheWriteLockAtBegin` tests pass. Removing `_txlock=immediate` makes the second test fail, as intended.
    - Task 3.3: the cookie test now also asserts `Partitioned` is off.

## 4. Modernize with go fix

- [x] 4.1 Run `go fix ./...` and review the diff by hand. Keep the rewrites that read naturally (`min`/`max`, `slices`/`maps` helpers, `for range n`, `any`). Revert any that obscure the code for a learner. Verify that `make test` passes before and after, and that `gofmt -l .` is empty. Commit as "Modernize Go code with go fix", and explain two or three of the rewrites in the commit body.
  - **Result:**
    - Kept: `wg.Go` (Go 1.25), `errors.AsType` (Go 1.26), `strings.SplitSeq`/`FieldsSeq` (Go 1.24), `max`, `for range n`. The `searchAll` WaitGroup comment was updated to explain `wg.Go`.
    - Reverted: go fix's rewrite of embedded-struct literals to promoted fields (Go 1.27) in `handler.go`, `refresh_test.go` and `curated_test.go`. The explicit `Widget: Widget{...}` documents the embedding the JSON shape depends on, and the test rewrite was 364 lines of churn.
    - vet is clean, and `go test -race ./...` passes.

## 5. Vulnerability checking

- [x] 5.1 Add govulncheck as a module tool: `go get -tool golang.org/x/vuln/cmd/govulncheck@v1.9.0`. Verify that `go.mod` has a `tool golang.org/x/vuln/cmd/govulncheck` line and that `go tool govulncheck -version` runs.
- [x] 5.2 Add a `vuln` target to `backend/Makefile` (`go tool govulncheck ./...`) and include it in `check`. Verify that `make check` passes with "No vulnerabilities found", or with only unreachable module-level notes. If govulncheck reports a reachable vulnerability, record its ID and the affected call path here and fix it (usually by bumping the module) before continuing.
  - **Result:** `make check` passes. govulncheck v1.9.0 reports 0 vulnerabilities reachable from the code. The one module-level match is GO-2026-5932 (`x/crypto/openpgp`, unmaintained, no fix); Porchlight never imports it.

## 6. Docs

- [x] 6.1 Rewrite the comment at the top of `backend/Makefile`. `CGO_ENABLED=0` stays as a choice: pure Go gives a single static binary that cross-compiles to Linux for the homelab. Mention the 1.2 finding only as history if useful. `GOTOOLCHAIN=local` means never downloading a toolchain. Document the new `vuln` target. Verify by reading the file and with `make check` still passing.
  - **Result:** `GOOS=linux make build` produces a statically linked ELF binary (checked for arm64 and amd64), so the Makefile's claim holds.
- [x] 6.2 Update `README.md`:
  - the prerequisite becomes "Go 1.27+", with the `.pkg` install note;
  - rewrite the "Why `make run` and not `go run .`?" section to match 6.1;
  - in "Tests", mention that `make check` now runs govulncheck and needs network, and that `make test` works offline.

  Verify by reading the README and following its run steps on the new toolchain.

## 7. End-to-end check

- [x] 7.1 Run the app on the new toolchain: `make demo && make demo-serve` with `npm run dev` in `frontend/`. Log in as `maya@rivera.test` and `jordan@rivera.test`. Verify these four things:
  - the homepage loads with the briefing, Today, Articles and Happening near you cards;
  - Arrange saves a width change that survives a reload;
  - logging out and back in works;
  - the API log shows no errors and no Brave or curating AI calls on first page loads.
  - **Result (2026-10-24):** run with the API on :8086 (demo database, fixtures on) and the frontend on :5186.
    - Maya (parent) and Jordan (kid) log in, and their layouts load.
    - Maya's Today card shows 3 events today (one all-day) and 2 tomorrow, from the fixture calendar. That means the upgraded golang-ical parser works end to end.
    - Articles load for both; Jordan's are marked kid-safe. Happening near you loads for both.
    - Maya's briefing was written with one AI call ("drizzly day… dentist appointment at quarter past noon…").
    - A width change (`custom-1` to 4/4) saved, and survived logout (`/api/me` returned 401 afterwards) and login.
    - The browser shows the sidebar, the briefing and the Today card.
    - The API log has no warnings beyond the expected fixtures notice, no errors, and no Brave or curating AI calls.
- [x] 7.2 Run `openspec validate go-upgrade` and confirm it passes, then add one "Go upgrade" line to "Post-PoC Additions" at the end of `devpost/prd.md` and `devpost/spec.md` when archiving. Verify both files gain exactly one line.
  - **Result:** `openspec validate go-upgrade` passes. The "Go upgrade" line was added to both files during apply, so archiving only needs to move the change.
