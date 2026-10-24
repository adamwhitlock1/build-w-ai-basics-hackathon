## Context

See proposal.md (Why) for the motivation. The current state:

- **Toolchain:** Go is the official macOS `.pkg` install at `/usr/local/go` (`VERSION` go1.22.0), added to `PATH` by `/etc/paths.d/go`. Homebrew isn't involved, and `/opt/homebrew/bin` comes earlier in `PATH`. `/usr/local/go/go.env` sets `GOTOOLCHAIN=auto`; `backend/Makefile` overrides it with `GOTOOLCHAIN=local` and also exports `CGO_ENABLED=0`.
- **Why `CGO_ENABLED=0` exists:** on macOS 27, go1.22.0's internal linker leaves out the Mach-O `LC_UUID` load command, so any cgo binary aborts at launch. The backend hits this because, with cgo on, `net` pulls in `runtime/cgo` on darwin. I reproduced it during research. Go 1.24 made the linker emit `LC_UUID` by default (golang/go#68678).
- **Dependencies:** `backend/go.mod` pins `modernc.org/sqlite` v1.36.1 (libc v1.61.13), `scs/v2` v2.8.0, `x/crypto` v0.33.0, `golang-ical` v0.3.7, `go-openai` v1.43.0, `godotenv` v1.5.1, `rrule-go` v1.8.2, and a `scs/sqlite3store` pseudo-version.
- **Store DSN:** `internal/store/store.go` uses `_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate`.
- **Tests:** 37 test files. `routes_test.go` checks the session cookie's flags (HttpOnly, SameSite=Lax, not Secure, Path=/, no Domain).
- **No CI exists yet;** that arrives in `v2-foundation`. `make check` (vet + test + build) is the gate.

## Goals / Non-Goals

**Goals:**
- One Go on this Mac, go1.27.3, that the project can never accidentally swap for another.
- Each dependency bump lands as its own green step, so a regression points at exactly one cause.
- `make check` fails when a known vulnerability is reachable from Porchlight's code.
- Docs explain why each build setting exists today, not why it existed under go1.22.0.

**Non-Goals:**
- Adopting new standard-library features such as `http.CrossOriginProtection`, `testing/synctest` or `os.Root`. Those belong to `v2-foundation`, where they come with behavior and tests.
- Adding Genkit or any new runtime dependency.
- CI, Docker images or homelab deployment.
- Changing `go-openai` or replacing any library.

## Decisions

### 1. Install with the official `.pkg`, run by the user
The go1.27.3 darwin-arm64 `.pkg` (sha256 `fbf88e3b97b66168c34ed859c4a52b795da07dfd806504bdb0b95dd58f8976da`) replaces `/usr/local/go` in place and keeps `/etc/paths.d/go`, so `which -a go` still shows exactly one Go. The user runs the install, because `installer` needs an administrator password; the task gives the exact commands, including the checksum check.
- *Alternatives:*
  - **Homebrew `go`:** it would sit in `/opt/homebrew/bin` and silently shadow the `.pkg`, leaving two Gos.
  - **`golang.org/dl/go1.27.3`, or a `toolchain` line with `GOTOOLCHAIN=auto`:** both download through go1.22.0's own `cmd/go`, which has a checksum-database bypass (GO-2026-6602).

### 2. `go 1.27.0`, no `toolchain` line, `GOTOOLCHAIN=local` stays
The `go` line declares the minimum language version. Leaving out `toolchain` and keeping `GOTOOLCHAIN=local` means an old Go prints "requires go >= 1.27.0" and stops; it never downloads anything.
- *Alternatives:* `go 1.25.0` (the lowest version Genkit needs) would let 1.25/1.26 build the project. That doesn't help a single-machine project, and it rules out 1.27-only features later.

### 3. Keep `CGO_ENABLED=0` deliberately, after checking cgo works
The reason changes from "cgo crashes" to "pure Go gives one static binary that cross-compiles to `GOOS=linux` for the homelab, with no C toolchain or macOS linker and signing issues". A task builds a small cgo program with go1.27.3 and records whether it runs. That way the docs state a checked fact, not a hope. If it still fails, nothing changes for Porchlight, and the docs say so.
- *Alternative:* turning cgo back on gains nothing, because no dependency needs it. modernc's SQLite is pure Go.

### 4. Upgrade the toolchain first, then dependencies one at a time
Order:
1. Bump `go 1.27.0` and run `gofmt`, `go mod tidy`, `go vet` and `go test -race` on unchanged code. This step separates language and stdlib fallout from dependency fallout.
2. `modernc.org/sqlite` v1.61.0, with an exact `modernc.org/libc v1.79.0`. The driver documents that downstream modules must pin the same libc it does, so `go get -u ./...` is off limits. Since v1.55.0 the driver validates DSN parameters, so the existing DSN is checked by the store tests (`TestOpenOddFileNames` and friends) and by a `PRAGMA` read-back of `foreign_keys`, `journal_mode` and `busy_timeout`.
3. `scs/v2` v2.9.0. It adds `Cookie.Partitioned`, which defaults to false, so the cookie test must still pass unchanged.
4. `x/crypto` v0.58.0. Only `bcrypt` is used.
5. `golang-ical` v0.3.8. The calendar fixture and recurrence tests cover it.

Each step is one commit, so `git bisect` stays useful.
- *Alternative:* one big `go get -u`. That's faster, but it would break the libc pin, and a failure wouldn't point to its cause.

### 5. `go fix` modernizers as a separate, reviewed commit
Go 1.26 rebuilt `go fix` on the analysis framework, with "modernizer" rewrites such as `min`/`max`, `slices` helpers and `for range n`. They preserve behavior. They run after the dependency steps, with a full test run before and after, and their diff is read by hand before committing. If a rewrite makes the code harder for a learner to read, it's dropped from the commit.
- *Alternative:* skip it. That's possible, but the rewrites are a cheap way to see what idiomatic modern Go looks like in code you already know.

### 6. `govulncheck` as a module tool in `make check`
`go get -tool golang.org/x/vuln/cmd/govulncheck@v1.9.0` records it in `go.mod` (a `tool` directive, Go 1.24+), so its version is pinned and reviewed like any dependency, and it runs with `go tool govulncheck ./...`. It reports only vulnerabilities the code can reach (call-graph analysis) and exits non-zero for them. That's the useful signal; module-level matches such as x/crypto's ssh advisories don't fail it. A new `vuln` Makefile target joins `check`. `govulncheck` needs network access to vuln.go.dev, so `make test` stays available offline.
- *Alternative:* install `govulncheck` globally with `go install`. That's unpinned, invisible in the repo, and differs per machine.

## Risks / Trade-offs

- [Other Go projects on this Mac relied on go1.22.0] → Go 1 compatibility makes upgrades safe in almost every case, and a project that needs an old Go can say so in its own `go.mod`. The install note in the task mentions it.
- [The new SQLite driver's DSN validation rejects or changes an existing parameter] → the PRAGMA read-back test catches it. `_txlock=immediate` and `_pragma` are both documented for modernc, and a failure blocks only this step.
- [`go vet` in 1.27 adds checks that flag existing code] → fix the findings in the toolchain step as a separate, explained edit; never silence them.
- [`go fix` rewrites hurt readability for a learner] → review by hand and drop any rewrite you wouldn't write yourself.
- [`govulncheck` reports a reachable vulnerability in a dependency with no fix] → document it in the task and pass `-show verbose` for evidence. Don't weaken the check for everyone; if it blocks, decide case by case.
- [`make check` now needs network] → `make test` and `make vet` still work offline. The README says so.
- [`-race` needs cgo on Linux] → on macOS, `go test -race` works with `CGO_ENABLED=0`. Linux CI in `v2-foundation` will set `CGO_ENABLED=1` for its race job only.

## Migration Plan

1. **Install:** the user installs the go1.27.3 `.pkg` (task 1) and confirms `go version`.
2. **Build:** the toolchain bump, each dependency step, the modernizers and govulncheck land as separate commits on the `aw-/v2-go-upgrade` branch. `make check` passes after every one.
3. **Run:** `make demo && make demo-serve` with the frontend, and log in as a demo member, to confirm the app runs end to end on the new toolchain.
4. **Merge:** open a PR to `main`, then archive the change.

**Rollback:** revert the branch (or individual commits). To go back to Go 1.22, reinstall that `.pkg`, which isn't recommended given its advisories. Because the commits are separate, one bad dependency bump can be reverted without undoing the toolchain upgrade.
