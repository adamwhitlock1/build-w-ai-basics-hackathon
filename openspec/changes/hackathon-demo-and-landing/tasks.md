## 1. Hosted demo foundations

- [x] 1.1 Add explicit hosted-demo configuration, marked-directory startup checks and separate deployment manifest; verify configuration tests reject development settings, unmarked existing databases, symlinks and household paths, and manifest checks show no household volume, backup credentials or port 1337 mapping.
- [x] 1.2 Extract reusable fictional seed helpers and implement hosted provisioning with operator-supplied passwords, sample labels, chores and a sample briefing; verify seed tests cover all four Rivera accounts, prepared layouts, password hashing and unchanged local-demo safeguards.
- [x] 1.3 Add hosted synthetic calendars without enabling development fixtures or fetching arbitrary calendar URLs; verify current-day/DST calendar tests and route tests reject external calendar updates while household behavior is unchanged.
- [x] 1.4 Add shared-demo notice and server restrictions on signup, invites and credential changes; verify parent/kid route authorization tests and browser login checks for both roles, including direct API restriction attempts.

## 2. Live usage limits and resets

- [x] 2.1 Add a persistent quota ledger outside resettable content with atomic reservations, configurable daily model/search ceilings, Denver reset times and bounded concurrency; verify concurrent-last-slot, ledger-failure, restart and 23/25-hour-day tests.
- [x] 2.2 Connect pre-dispatch enforcement to all model/search paths, retries and tool rounds and bound output size; verify fake-provider tests count actual dispatches across briefing, widget, parent chat and kid safety/generation paths and prove no skipped safety call or bypass on retry.
- [x] 2.3 Add limit and partial-stream UI states that keep cached content visible and stop automatic retries; verify frontend tests distinguish quota exhaustion from provider failures and browser checks retain browsable seeded content.
- [x] 2.4 Implement operator-only content reset with stopped writes, marker/target checks, validated replacement and session invalidation; verify reset tests retain quota, preserve prior content on failure, reject unsafe targets and leave a separate household database hash unchanged.

## 3. Public landing page

- [x] 3.1 Extract/reuse homepage rendering and make `/` session-aware without moving member URLs; verify route/browser tests for anonymous visits, signed-in parent/kid, `?as`, incomplete setup, expired sessions and protected deep links, with no private anonymous data fetch.
- [x] 3.2 Add the responsive branded landing page and fictional screenshot with demo/login actions and optional video; verify keyboard, 390px/desktop, both themes, reduced-motion, contrast and missing-link states.
- [x] 3.3 Add allowlisted public URL configuration and demo-login guidance; verify configuration tests reject non-HTTPS/unsafe URLs and ensure the public response contains no keys, passwords or private settings.

## 4. Verification and operator handoff

- [x] 4.1 Document separate Coolify/tunnel deployment, credentials delivery, allowance tuning, reset, rollback and no automatic demo restore; verify the runbook identifies concrete commands, expected results and reserved family resources without embedding secrets.
- [x] 4.2 Run the existing backend/frontend checks and strict OpenSpec validation; record exact commands/results and verify normal household mode still supports its existing workflows.
- [x] 4.3 Rehearse the complete demo locally with fake providers and separate parent/kid browser sessions; verify widget creation/arrangement, chore completion/undo, chat, parent review, exhaustion and reset outcomes, recording evidence separately from live deployment claims.
- [x] 4.4 Deliver the video preparation and submission handoff linked from operator documentation; verify every item in video-prep.md has a clear owner and that actual recording/upload/submission items remain unchecked until performed.

## 5. Owner release and submission steps

These steps require the later release/submission phase. They are not evidence of implementation completion and must not be silently marked complete after local tests.

- [ ] 5.1 Provision the isolated demo and configure landing links when deployment is authorized; verify independent storage/port/keys, HTTPS session behavior, ordinary redeploy persistence, and parent/kid access in a fresh browser outside the owner's authenticated environment.
- [ ] 5.2 Rehearse real AI with demo credentials, confirm quota and child safety behavior, confirm the 2,048-token output cap leaves structured answers complete for the configured model, and set adequate judging allowance; record live results and verify household resources are unchanged.
- [ ] 5.3 Owner records and uploads the under-three-minute demo following video-prep.md; verify playback without authentication, legible UI/audio and no personal data or secrets.
- [ ] 5.4 Owner completes submission text, required form/survey fields and judge testing instructions, then submits; verify public repository/video links and retain the submission confirmation. Keep judge access available through the judging period.

## Verification record

See [verification.md](verification.md) for commands, local browser results and the limits of the evidence. Checked implementation items do not represent deployed-provider or owner release verification.
