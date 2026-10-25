## Context

See proposal.md for motivation and specs/chore-checklist/spec.md for the behavior contract. The current implementation has:

- `backend/internal/auth/subject.go`: `UserFrom` identifies the viewer; `SubjectFrom` identifies the viewed member. `RequireMember` enforces same-family membership. `backend/routes.go` combines these with `private`, `asParent`, and `parentsOnly` wrappers.
- `backend/internal/store/accounts.go`: family membership and admin status, but no saved household time zone. Calendar/briefing endpoints accept a browser time zone; that must not drive chores.
- `backend/internal/store/migrate.go` and `backend/migrations/`: ordered, transactional SQL migrations; 009_theme.sql is currently the latest.
- `frontend/src/routes/_authed/index.tsx`: own homepage and parent member selection through `/?as=<id>`. `frontend/src/family/MemberPage.tsx` renders the viewed member. Puck owns saved widget arrangement separately.
- `frontend/src/api/client.ts`, `hooks.ts`, and `pageOwner.ts`: typed API access, TanStack Query, and member-scoped cache keys. Frontend tests use Node's test runner; there is no existing component-test framework to assume.

## Goals / Non-Goals

**Goals:** Use existing family authorization, preserve saved page layouts, make day boundaries deterministic and testable, and keep writes small and idempotent.

**Non-Goals:** A generic scheduling engine, an event bus, recurring background reset jobs, a new UI testing framework, AI integration, or new external services. Chore planning does not authorize feature implementation in this PR.

## Decisions

### 1. A dedicated chore service with explicit actor and subject

Add `backend/internal/chores` for service and HTTP concerns and `backend/internal/store/chores.go` for persistence. Pass the authenticated viewer and the authorized subject to the service; never accept family identity or completion actor from JSON. Service-level checks supplement route middleware so later callers cannot accidentally bypass ownership rules.

Proposed routes:

| Route | Access / purpose |
| --- | --- |
| `GET /api/me/chores` | Authenticated member's checklist |
| `PUT /api/me/chores/{choreId}/completion` | Set own completed state |
| `GET /api/members/{id}/chores` | Parent viewing a same-family member |
| `POST /api/members/{id}/chores` | Parent creates a title for that member; own id is valid |
| `PUT /api/members/{id}/chores/{choreId}` | Parent renames the chore |
| `DELETE /api/members/{id}/chores/{choreId}` | Parent removes the chore |
| `PUT /api/members/{id}/chores/{choreId}/completion` | Parent sets that member's state |
| `GET /api/family/chores` | Parent-only family progress summary |

Completion JSON is `{ "date": "YYYY-MM-DD", "completed": true }`; create/rename accept only a title. List responses include `date`, `timeZone`, `serverNow`, `nextResetAt`, `items`, `completedCount`, and `totalCount`. Summary responses use the same date metadata and include every current family member, including members with zero chores. Mutation responses return the subject's updated checklist (create uses 201; other successes use 200). Use existing error envelopes: 400 invalid input, 401 no session, 403 forbidden role, 404 inaccessible/missing member or chore, 409 stale date or active-chore limit conflict. Responses are not cacheable by shared HTTP caches.

Alternative: add chore operations to generic widgets. Rejected because assignments and completion have different write permissions and must not disappear when a member edits their layout.

### 2. Assignments and daily completion are separate rows

Add the next available migration (expected `010_chores.sql`; verify ordering at implementation). `chores` contains `id`, `family_id`, `assignee_user_id`, `title`, `created_by_user_id`, `created_at`, `updated_at`, and nullable `archived_at`. `chore_completions` contains `chore_id`, household `local_date`, `completed_by_user_id`, and `completed_at`, with a unique primary key on `(chore_id, local_date)` and foreign keys consistent with existing account relations.

A checked state is an upsert; unchecked deletes only that chore/date's completion row. Soft removal sets `archived_at` so older completion records remain available without a history UI. All visible lists and totals exclude archived chores. Rename preserves identity and completion; moving a chore to another member is intentionally unavailable (remove and create instead). Order by creation time and id. Keep titles as trimmed plain text, measured in Unicode code points, with a 120-character maximum and 30 active chores per member.

Scope SQL reads and writes by family and assignee as well as chore id. Validate active-chore existence and perform completion/removal operations transactionally; removal must not race a completion into reviving a chore. Enforce the active-count cap within the create transaction so concurrent creates cannot exceed it. Handle SQLite busy failures through the repository's existing error policy without partial writes. Summary counts use one bounded family query rather than a per-member HTTP request.

Alternative: store checkmarks in Puck JSON or overwrite a daily boolean. Rejected because that mixes permissions and loses day boundaries/history. No new retention cleanup job in this version; row growth is small for one household.

### 3. One household clock, with no reset job

Session-settled: daily repeat, immediate completion with parent undo, and America/Denver for everyone are user-selected. Use a named chore time-zone constant loaded through `time.LoadLocation` and an injected clock in the service. Ensure time-zone data is available in local tests and the deployed image (the image already includes `tzdata`); fail clearly rather than silently falling back to UTC if loading fails. This is intentionally a one-household setting, with no settings UI or new environment variable in this slice.

Compute the current local date and the next local calendar midnight from one sampled server instant. Use calendar arithmetic (`time.Date`/`AddDate`), not `now + 24h`; the interval can be 23 or 25 hours. Query completions by current date. A restart after midnight naturally reads the new date.

A completion request must carry the date the viewer saw. Validate it against the sampled server day at mutation execution; a mismatch returns 409 and changes no rows. The client reloads and asks the person to check again. Check/uncheck sends an explicit desired state, never “toggle,” so retries are idempotent. Conflicting same-day writes are last-commit-wins; no websockets or revision-editing protocol is necessary for this household checklist.

Alternative: browser time zones or cron-driven clearing. Rejected because travel could give parent and child different days, and a stopped server could miss a reset.

### 4. Fixed cards and predictable cache behavior

Add a chore API module under `frontend/src/api/` and components under `frontend/src/chores/`. Render the member's “Today's chores” card before the arranged widgets in both YourPage and MemberPage. Parents get add/rename/remove controls on either surface. Parent-owned homepages also show a compact family progress card linking through the existing `/?as=` navigation. The viewed member's identity comes from the same authorized page context as existing cards.

Keep all chore cards outside Puck; do not rewrite saved layouts or introduce generic shared-widget semantics. Display America/Denver alongside the daily heading. Use native labeled checkboxes, existing form/error styles, and the current theme/reduced-motion conventions. Parent add forms and removal confirmations must restore focus sensibly.

Cache keys include viewer/session and subject identity, following existing logout cache handling. Invalidate the subject checklist and family summary after every successful local mutation. Disable the affected control during submission and keep the last confirmed value until success. Do not automatically replay a write after a stale-date error or an ambiguous failure. On failure expose retry/refetch without showing a false saved state.

Poll visible cards every 60 seconds, refresh on focus/visibility return, and schedule an additional refetch at the server-provided next midnight using the serverNow-to-nextResetAt duration. If rollover reload fails, label the data as stale and disable completion until the current date is confirmed. Cancel/recalculate timers after refetch and on unmount. A device clock must not choose the stored date. Parent summary counts and day metadata come from one server read.

Alternative: optimistic toggle with global cache state. Rejected because an incorrect completed count is confusing, and the list is small enough for confirmed writes.

### 5. Planning PR and deployment documentation

The current PR includes the OpenSpec artifacts, the roadmap's next-item status, and `docs/deploy.md` cleanup. The restore guide now describes a temporary-file restore and optional download, rather than requiring production Secure cookies to work over an HTTP SSH tunnel. Record that the owner reported live access, successful restore, local database inspection, and cleanup on 2026-10-25. Do not imply an independently verified recovery point, freshness check, automated integrity result, or replacement-server exercise.

## Risks / Trade-offs

- [Concurrent edits or a midnight click] → explicit completed state, transactional writes, server-day validation, and stale-date reload tests.
- [A kid reaches a parent operation directly] → middleware plus service/store scoping, tested with different-family fixtures and guessed ids.
- [New cards disturb customized pages] → render outside Puck and test saved-layout preservation and the parent member-view flow.
- [Polling is not instantaneous] → document the 60-second visible refresh bound; local saves invalidate immediately.
- [Long-term accumulation and edits to titles] → retain dated completion rows without promising an immutable audit trail or historical title snapshots. Revisit retention only with a history feature.
- [Hard-coded household zone limits reuse] → one named constant for this household; a future zone-setting feature needs an explicit date-history policy.

## Migration Plan

1. In the implementation PR, add an additive migration and test both an existing populated database and a fresh database. Keep existing accounts, layouts, and calendars intact.
2. Run store/service/route and frontend checks, then browser verification with local fake accounts and a controllable service clock. No live keys are needed for chores.
3. After separate merge/deployment authorization, deploy and verify parent assignment, kid completion, parent undo, and progress on the homelab. Do not set the production clock to test midnight.
4. Roll back by deploying the previous app image; it ignores the new tables. Preserve the database and additive schema rather than dropping chore data.
5. Archive the chore change and update the roadmap only once its implementation and required checks are complete.
