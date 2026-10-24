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

.PHONY: run build vet test vuln check seed demo demo-serve ai-smoke dev-ui

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

# Calls the real AI providers (OpenRouter, and MiniMax when MINIMAX_API_KEY
# is set) through Genkit: structured output, one tool call, and usage. It
# spends a fraction of a cent, so it's opt-in and never runs in CI.
ai-smoke:
	PORCHLIGHT_LIVE_AI=1 go test -count=1 -v -run Live ./internal/aikit/

# The API under the Genkit Dev UI (http://localhost:4000), which shows every
# model call's prompt, reply, tokens and timing. GENKIT_ENV=dev starts
# Genkit's reflection server, which runs AI flows without a login: this is a
# laptop-only tool and must never run on the homelab. The CLI is pinned and
# fetched by npx; extra settings pass through, e.g.
# `DB_PATH=porchlight.db.demo PORCHLIGHT_DEV_FIXTURES=1 make dev-ui`.
dev-ui:
	GENKIT_ENV=dev npx --yes genkit-cli@1.44.1 start -- go run .

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
