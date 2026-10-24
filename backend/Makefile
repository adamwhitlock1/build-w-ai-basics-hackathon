# Every go command here runs with these two settings:
#
# CGO_ENABLED=0 -- build pure Go. Porchlight and its SQLite driver
#   (modernc.org/sqlite) need no C code, so the result is one static binary
#   that cross-compiles to Linux for the homelab (GOOS=linux make build) with
#   no C toolchain. (It was first turned off because go1.22.0 binaries built
#   with cgo crashed on this Mac; Go 1.24+ fixed that, so it's a choice now.)
# GOTOOLCHAIN=local -- always use the installed Go and never download a
#   toolchain. If go.mod ever asks for a newer Go than is installed, the
#   build stops and says so instead.
export CGO_ENABLED := 0
export GOTOOLCHAIN := local

.PHONY: run build vet test vuln check seed demo demo-serve

run:
	go run .

build:
	go build -o bin/porchlight .

vet:
	go vet ./...

test:
	go test ./...

# Known vulnerabilities that Porchlight's code can actually reach, in Go
# itself or any dependency. govulncheck is pinned in go.mod (the `tool` line)
# and checks the call graph, so a vulnerable function we never call doesn't
# fail the build. It downloads the vulnerability list from vuln.go.dev, so it
# needs network; `make test` works offline.
vuln:
	go tool govulncheck ./...

check: vet test vuln build

# A test family to log in with, for local testing only (see cmd/seed).
seed:
	go run ./cmd/seed

# The demo family, rebuilt from scratch in porchlight.db.demo: two parents,
# two kids, a pending invite, calendars, widgets and remembered results, plus
# a second family (see cmd/seed/demo.go). Local testing only.
demo:
	go run ./cmd/seed -demo

# The API on the demo database, answering the demo family's calendar links
# itself (PORCHLIGHT_DEV_FIXTURES). Never turn fixtures on in a deployment.
demo-serve:
	PORCHLIGHT_DEV_FIXTURES=1 DB_PATH=porchlight.db.demo go run .
