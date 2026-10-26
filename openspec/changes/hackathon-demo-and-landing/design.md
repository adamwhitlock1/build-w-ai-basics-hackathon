## Context

See proposal.md for motivation. The current root homepage is `frontend/src/routes/_authed/index.tsx`; `_authed.tsx` redirects anonymous visits to login. Existing `/?as=<id>` links are used for parent member views. The seed in `backend/cmd/seed/` already creates Rivera parents Maya/Sam and kids Jordan/Lily, populated widgets and sample chats, but no chores. Its public development credentials and destructive local reset are not a hosted deployment interface.

`backend/main.go` rejects development fixtures in production. `internal/calendar/fixtures.go` generates current-day synthetic feeds. `compose.yaml` maps the family service to loopback 1337 and mounts its persistent data. `deploy/start.sh` restores missing databases from Litestream. Usage recording is asynchronous and after the call, so it cannot enforce an allowance. Calls span the legacy LLM client, `internal/aikit` and search sources.

## Goals / Non-Goals

Goals: let judges experience real parent/kid workflows with fictional data, protect the household deployment, preserve ordinary production authentication and child safety, and produce a practical recording/submission checklist.

Non-goals: per-visitor cloned families, one-click impersonation, automatic public resets, marketing analytics, signup funnels, a general billing system, new family features, or rewriting `devpost/`. No experimental Genkit agents or framework migration; repository restrictions take precedence over generic Genkit skill examples.

## Decisions

### Separate deployment with explicit hosted-demo identity

Use a second Coolify application, hostname, volume and unused loopback port. Suggested hostname: `demo.kinlight.codestuff.io`; verify availability during deployment. Keep the family service on 1337. Introduce `KINLIGHT_HOSTED_DEMO=1` alongside a required HTTPS public URL, an explicit demo data directory, and a provisioning marker stored with the demo data. Never infer demo status from an account email or hostname alone.

A separate provisioning command initializes only an empty, dedicated demo directory; later resets require its marker and an explicit target confirmation. Reject symlinks, ordinary household database names/locations and unmarked nonempty targets. The demo application must refuse an unmarked database. Do not pass production storage or backup secrets into the demo. Disposable demo content has no Litestream restore path; the existing family Compose and backup behavior remain unchanged. Share seed-building helpers rather than exposing the local development command to production.

### Fixed judge logins and a shared sandbox

Use ordinary session/password login, with separate operator-supplied preset parent and child passwords (never the checked-in development password). Supply them in Devpost testing instructions; do not require email delivery or a request-access flow. The landing page links to demo login, which explains where judges find credentials. No public credential display or auth bypass is needed.

All judges share the Rivera family in this first version. Show a persistent notice that it is fictional, shared, resettable and unsuitable for personal information. Keep user-created chat content visibly distinct from seeded samples. Disable demo account/password changes, new invitations, signup, and custom calendar URL writes server-side, including through any alternate tool route. Permit dashboard arrangement, widget creation, interests changes, chores, chat and parent review. These restrictions prevent shared credentials and demo availability from being destroyed while retaining the main judging journey. Existing parent/kid authorization applies; viewing a kid's page as a parent is not a kid login. Document separate browser profiles for recording both roles.

### Curated seed and explicit reset

Reuse the fictional family, refresh cached widget dates, add daily chores with both complete/incomplete examples, and retain sample homework/explore conversations plus the medium-severity review example. Include a clearly labeled sample briefing so the first view is populated. Hosted-demo calendar handling accepts only built-in fixture identities and never fetches arbitrary URLs; it is a new demo-specific adapter, not permission to enable `KINLIGHT_DEV_FIXTURES` in production. Real-family calendar behavior is unchanged.

Reset is an operator maintenance action: stop demo requests/process, construct and validate a replacement content database, invalidate sessions, then atomically replace content and restart. Failures retain the last usable content. Do not delete a live SQLite database or forget WAL/SHM files. No web reset endpoint, periodic reset during judging, or reset-on-redeploy. Reset before recording and before submitting; subsequent resets are deliberate maintenance and log users out. Keep the quota ledger separate from resettable content and preserve it across redeploys and content resets.

### Live calls with an enforceable allowance

User confirmed live AI on 2026-10-25. Proposed defaults: 100 model requests and 50 paid search requests per demo per Denver calendar day, with at most two active AI operations. An operation takes its slot on its first provider dispatch and holds it until its request ends, so reads never queue behind live AI; a request that waits 20 seconds without a slot ends as "busy" without spending allowance. These are configurable positive server settings, not a dollar guarantee. Apply provider output-token limits (initial maximum 2,048 per generation) and retain existing history/input/tool-round bounds. Demo provider keys are separate from household keys; kid calls retain zero-retention routing and the safety preflight.

Reserve each outbound attempt synchronously and atomically in a persistent ledger before contacting a provider, covering automatic refresh, briefing, widget retries, safety classification, generation, tool follow-ups, and searches. Each retry costs another slot; do not refund attempted calls on error. Enforce at outbound boundaries, not only HTTP handlers; concurrency tests must show the cap cannot be exceeded. Fail closed on ledger errors. No allowed generation may skip child safety to fit within the quota. If quota runs out mid-turn, terminate with an explicit limit outcome, preserve already streamed content as incomplete, and do not claim success. Keep existing content browsable, show the next Denver reset time, and suppress automatic retry loops until then. A content reset, logout, account switch or process restart never replenishes quota.

An operator can increase the configured allowance if judges need more; no self-service purchase or account registration. Before submission, rehearse and set enough allowance for testing through the judging period. Provider failures show their own retryable failure state, not fabricated answers. A separate provider credit ceiling, where supported, is additional operational protection rather than the application quota's enforcement mechanism.

### Public root without breaking member links

Move root route ownership outside the pathless authenticated layout and reuse an extracted homepage component. At `/`, resolve session once: anonymous visitors get the landing page, configured signed-in users get their existing dashboard, and incomplete accounts retain create-family handling. Keep `?as` validation and all private API authorization. Other protected deep links still redirect to login. Never fetch homepage data for the anonymous landing page or flash private content during loading/errors.

Use current typography, lantern assets, both themes and responsive styles. Sections: short owner-reviewed introduction, fictional dashboard screenshot, personalized pages/daily routines/AI oversight overview, and actions for demo, video and family sign-in. Configure public HTTPS demo/video links through an allowlisted public configuration response; never expose environment variables or credentials wholesale. Omit the video action until a real URL is configured. Missing demo URL yields an honest unavailable message rather than a broken link. No signup CTA. Prepare screenshots only from fictional data.

### Submission preparation stays separate from product code

Use video-prep.md in this change as the preparation checklist; implementation can link it from the operator guide. Target approximately two minutes. The owner supplies narration and personal reflections and completes the actual Devpost form. Do not rewrite the historical 60-second PoC plan. Public rules were checked on 2026-10-25; verify actual form fields before submission. Landing copy is product copy; it is not the owner's submission answers.

## Risks / Trade-offs

- Shared accounts expose judges' entered text to other judges and allow layout changes → clear shared-demo notice, no personal data, deliberate resets and a video that remains a reliable tour.
- Global quota exhaustion can interrupt evaluation → cached examples remain readable, clear reset time, realistic rehearsal and operator-adjustable limits; verify judge access remains usable through October 30.
- Deadline pressure → deliver demo and recording first, then minimal landing page; defer decorative motion and extra sections. Never trade away isolation or safety checks.
- Routing changes can affect member links and auth → explicit anonymous, parent, kid, expired-session and incomplete-account browser tests.

## Migration Plan

Implement/test locally using temporary databases and fake providers, then review the PR. Prepare a separate demo deployment manifest and runbook. Live provisioning and provider calls require a later deployment step; proposal creation does not perform them. Deploy the demo, confirm volume/port/credential separation and both logins, then configure the landing links and capture the video. Record deployed revision and tested URLs. Rollback the demo independently; remove its public link or revert the landing change without moving or resetting family data. Keep the submitted version available through judging.

## Open Questions

- Actual demo hostname and free host port: operational allocation during deployment; port 1337 is reserved for the family service.
- Final video URL and exact Devpost form fields: filled by the owner before submission; absent links remain hidden.
- Daily quota values can be adjusted after rehearsal without changing the enforcement contract.
