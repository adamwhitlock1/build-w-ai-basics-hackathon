## 1. Persistence and household day

- [ ] 1.1 Add the next additive migration for assignments and dated completions, plus scoped store queries. Verify fresh and populated database migration tests preserve accounts, layouts, and calendar data.
- [ ] 1.2 Implement create, rename, archive, explicit completion state, and summary queries. Verify store tests cover Unicode title limits, the 30-active cap including concurrent creates, stable ordering, rename preservation, duplicate writes, conflicting writes, removal/completion races, and zero-chore members.
- [ ] 1.3 Add the chore service with an injected clock and America/Denver date metadata. Verify service tests cover midnight, travel-independent dates, 23/25-hour daylight-saving days, restart after missed days, retained prior completions, and stale-date rejection without writes.

## 2. Authorized API

- [ ] 2.1 Wire the own-member, parent member-management, and family-summary routes through existing authentication wrappers and service/store scope checks. Verify route tests cover unauthenticated requests, child self-completion, denied child management/summary/other-member access, parent self-management, and cross-family identifiers returning the same 404 as missing records.
- [ ] 2.2 Implement validated request/response contracts and error handling. Verify route tests cover list/count consistency, all mutation responses, invalid titles and dates, missing completion state, count-limit conflicts, archived chores, stale-date 409 responses, and no shared-cache responses.

## 3. Frontend data and cards

- [ ] 3.1 Add typed chore API functions and member/session-scoped queries. Verify frontend helper tests cover desired-state payloads, cache-key isolation, midnight delay calculations from server metadata, and stale-date error handling; run the frontend type/build check.
- [ ] 3.2 Add the fixed checklist to own and parent-viewed homepages with parent add/rename/remove actions, confirmed save states, and accessible controls. Verify in a local browser that kids complete their own chores, parents assign and undo, empty/error states are clear, removal asks confirmation, titles render as text, and saved Arrange layouts stay unchanged.
- [ ] 3.3 Add the parent family summary and member links. Verify in a local browser that all family members appear with accurate counts, zero chores is labeled, links open the correct member, and kids cannot see the summary.
- [ ] 3.4 Add mutation invalidation, visible 60-second polling, focus/visibility refresh, and midnight reload with timer cleanup. Verify helper tests and a local browser with a controlled clock cover rollover, background resume, another session's update, failed rollover disabling stale completion, and no automatic replay of stale writes.

## 4. Integrated verification and completion

- [ ] 4.1 Run `make check` in `backend` and the frontend lint, test, check:contrast, check:copy, and build scripts. Verify all pass and record results with any environment limitations.
- [ ] 4.2 Exercise parent assignment → child completion → parent undo → summary refresh with local test accounts. Verify keyboard focus/announcements, narrow screens, both themes, and failure/retry behavior; record the browser results without changing the production clock or family data.
- [ ] 4.3 Reconcile the delivered behavior with every requirement, run strict OpenSpec validation, and update the roadmap to implemented only when required checks pass. Archive through the OpenSpec workflow after implementation is complete; verify the resulting main spec and historical archive agree. Production deployment remains separately authorized.
