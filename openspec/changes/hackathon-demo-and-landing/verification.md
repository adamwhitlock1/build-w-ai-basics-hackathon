# Implementation verification — October 25, 2026

## Automated checks

- `cd backend && make check`: passed Go vet, the complete test suite, vulnerability analysis and production compilation. No reachable vulnerabilities; one finding exists in a required module whose affected code is not called.
- `cd backend && CGO_ENABLED=0 go test ./internal/aikit ./cmd/seed ./internal/hosteddemo`: passed after adding retry/chat-dispatch coverage and strengthening reset assertions.
- `cd frontend && npm run lint && npm test && npm run build && npm run check:contrast && npm run check:copy`: passed; 16 tests, 44 contrast pairings in both themes. Lint/build repeated after removing effect-driven partial-response cleanup.
- Strict OpenSpec validation and `git diff --check`: passed before commit.

Storage tests cover marked targets, running-server locks, unsafe targets/symlinks, retained content after failed reset, retained quota after reset/restart, failed initial provisioning retry, and staging inside the mounted demo volume. Seed tests verify role passwords, all four accounts, chores, ignored private calendar overrides and cleared sessions. Existing household tests remain enabled.

Quota tests cover 20 concurrent attempts competing for three slots, closed-ledger failure, Denver 23/25-hour days, output limits, shared legacy/Genkit enforcement, exhaustion before kid safety, parent/kid generation, model retry prevention, and a failed search stopping follow-up model dispatch. These tests use fake providers; they do not establish actual provider billing or quality.

## Local browser rehearsal

The opt-in `TestDemoBrowser` served the production frontend against a disposable marked database under `/private/tmp/kinlight-hackathon-check/kinlight-demo`. Providers were fake. Separate browser sessions represented Maya, Jordan and an anonymous visitor.

Observed:

- Maya and Jordan logged in with distinct role passwords and opened populated pages. Maya viewed Jordan through the existing `/?as=3` URL.
- Jordan completed a chore; Maya saw the completion and undid it.
- Maya submitted a card prompt; the fake provider's SpaceX card appeared. Arrange opened and Done returned to the normal page.
- Jordan opened a sample homework chat and sent a new message. With one model call remaining, safety consumed the last slot and the fixture's partial reply was retained with an explicit incomplete-answer label and allowance error.
- Maya marked the seeded Lily review as reviewed. Saved pages and reviews remained readable with zero model calls available.
- Anonymous `/` showed the landing page; `/reviews` redirected to login. Captured anonymous API traffic contained `/api/me` and `/api/public-config`, without homepage/member data requests.
- The landing page was inspected at 390×844 and 1440×1000, in dark and light themes. Neither viewport overflowed horizontally. Keyboard Tab exposed a visible login focus ring. Reduced-motion preference was enabled for the light desktop inspection. The fictional screenshot loaded. The unconfigured video action was absent.
- After a clean stop, the seed reset command completed. The sessions table was empty and the quota ledger still recorded 100 model attempts. The old parent session redirected to login. Signing in again restored sample cards, chores and the unreviewed sample flag while the exhausted allowance remained visible.

The browser fixture deliberately uses scripted replies; its intermediate chunk for exhaustion testing is a test fixture behavior. The production transport is separately tested against fake HTTP providers. Live inference, Docker/Coolify provisioning, HTTPS cookies, Cloudflare access and ordinary deployed redeploy persistence remain owner release checks. Incomplete-setup routing and empty-demo-link handling were inspected in source; they were not separately exercised in this browser rehearsal. Full drag-and-drop permutations and every model/tool combination were not exhaustively replayed.

## Delivery boundary

No running household service, production database, tunnel, bucket or provider account was changed. No paid AI calls were made. The separate deployment manifest and operator runbook are ready for review. Section 5 and the actual recording/upload/submission checklist remain unchecked.

## Review follow-up (2026-10-25)

Concurrency slots are now taken on an operation's first provider dispatch rather than by URL, with a 20-second wait that ends as "busy" (code `demo_busy`) without spending allowance. Spent allowance turns new chat messages and card builds away with a 429 before anything is saved, and a safety check stopped by the demo no longer files a "couldn't check" review. Demo pauses carry a `code` (`demo_limit` / `demo_busy`) that the frontend reads instead of matching wording. Public config keeps `demo: true` when the ledger can't be read. The legacy model client keeps its attribution headers under the allowance transport. Birth-year changes and conversation deletes are fixed in the demo. Stale staging directories from a killed build no longer block startup or reset. Tests cover each of these; backend `go vet`/`go test ./...` and frontend lint/test/build were re-run.
