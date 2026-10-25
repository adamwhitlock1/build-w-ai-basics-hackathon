# Chore checklist verification — 2026-10-25

## Automated checks

- Backend `make check`: vet, all Go tests, reachable-vulnerability scan, and build passed. The scanner found no reachable vulnerabilities.
- New store tests cover a populated pre-chore database migration, existing account/layout/calendar preservation, Unicode titles, concurrent assignment limits, ordering, rename preservation, idempotent and conflicting completion writes, removal races, and retained prior-day completions.
- Service tests cover Denver dates independent of travel, 23/25-hour DST days, midnight rejection without changing either day, restart after missed days, and service authorization.
- Route tests cover the parent/child journey, authentication, role and family isolation, invalid requests, 30-chore cap, archived IDs, no-store headers, and saved Arrange layout preservation.
- Frontend lint, all 14 Node tests, build/type check, 44 contrast pairings, and copy check passed. Chore helper tests cover server-based rollover timing, cache isolation, explicit desired state, and stale-day errors.
- Strict OpenSpec validation passed for all 16 main specs.

## Local browser checks

Used the built frontend with disposable SQLite data, fake external services, separate parent/child browser sessions, and an injected chore clock. No production keys, data, or system-clock changes were used.

- Parent self-assignment and child assignment; child keyboard completion; parent undo; rename preserves identity; parent summary/member navigation.
- Child has no assignment controls or family summary. Empty lists and zero-assignment summary entries have their specified labels.
- Chore titles containing HTML render literally. Rename restores button focus; confirmed removal focuses the card heading and removes the chore.
- Offline write leaves the confirmed checkmark unchanged and offers reload/retry. Successful write followed by a failed GET keeps the newly confirmed checkmark and reports the refresh error.
- Simulated Denver midnight clears yesterday's checks. Offline rollover disables stale checkboxes; reconnect reloads today without replaying a write. Visibility return and window-focus refresh were exercised.
- A separate-session change reached the open parent summary on its scheduled fetch at approximately 60 seconds (resource timing recorded at 60,086 ms after page load).
- Checked both themes and a 390px viewport without horizontal overflow. Parent controls move below the title on narrow screens.
- Fresh-load Arrange/Done works with fixed chore cards outside the editor. The API regression test independently verifies the saved layout remains unchanged through chore mutations.

## Limits

These are local checks; production deployment and owner acceptance remain separate. Frontend `npm audit` reports four existing high-severity entries involving Puck, tiptap/html, happy-dom, and zod. Package manifests and lockfile are unchanged by this feature; dependency remediation is outside this change.
