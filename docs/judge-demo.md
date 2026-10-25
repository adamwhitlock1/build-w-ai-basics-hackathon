# Hosted judge demo

The judge demo is a separate, disposable Rivera household. It must never mount the real family's volume or receive its backup configuration. The family's existing hostname, port 1337, database and Litestream setup stay as they are.

## Deploy a separate application

1. Build the reviewed revision in a **second Coolify application**, selecting `compose.demo.yaml`. Choose a separate HTTPS hostname and an unused loopback port other than 1337. Point host-based cloudflared at that port, preserving Host. Judges must reach this hostname without the owner's Cloudflare Access login.
2. Configure `KINLIGHT_PUBLIC_URL` as that demo origin, `KINLIGHT_DEMO_PORT` as the unused port, and `KINLIGHT_TRUST_CLOUDFLARE=1` only when the tunnel is the sole external entry point. Supply **separate** `DEMO_OPENROUTER_API_KEY`, `DEMO_OPENROUTER_KIDS_API_KEY` (zero-retention enabled), and `DEMO_BRAVE_API_KEY`. Do not copy household provider keys. Do not add Litestream settings or enable development fixtures/reflection/retention overrides. The server refuses an unmarked database until provisioning finishes; this initial refusal is expected.
3. Before starting the service, provision its volume with the image's `/app/seed` command below. This is an explicit operator step, never startup behavior. Use the same Compose project name, manifest and environment file as the demo application, so the one-off container uses its actual volume. Inspect the resolved volume mapping first. A separate shell invocation against a different project would create an unrelated empty volume.
4. Start the demo and check `/api/health`, then log in as Maya and Jordan separately. Confirm session cookies are Secure/HttpOnly/SameSite=Lax, the page says shared judge demo, and new AI actions work. Re-deploy once to verify existing content and allowance persist.
5. On the **family** deployment set `KINLIGHT_DEMO_URL` to the verified demo origin. After the video is public, set `KINLIGHT_VIDEO_URL` to its HTTPS link. Neither is a secret. Empty video settings omit the action. No application code or database edits are needed to change these links.

The following commands are a template for an operator shell already in the deployed demo checkout with its correct Compose project selected. On Coolify, use the actual generated project/compose location or its corresponding terminal operation; do not guess its project name. Never run against `compose.yaml` or the family application.

```sh
# Enter distinct passwords of at least 12 characters without storing them in history.
read -r -s KINLIGHT_DEMO_PARENT_PASSWORD
read -r -s KINLIGHT_DEMO_KID_PASSWORD
export KINLIGHT_DEMO_PARENT_PASSWORD KINLIGHT_DEMO_KID_PASSWORD

docker compose -f compose.demo.yaml run --rm --no-deps \
  -e KINLIGHT_DEMO_PARENT_PASSWORD -e KINLIGHT_DEMO_KID_PASSWORD \
  --entrypoint /app/seed judge-demo -hosted-dir /kinlight-demo

unset KINLIGHT_DEMO_PARENT_PASSWORD KINLIGHT_DEMO_KID_PASSWORD
docker compose -f compose.demo.yaml up -d judge-demo
```

Provisioning succeeds silently, creates marked content, and prints no passwords or invite links. It refuses an already populated directory. The dedicated volume contains `judge-content.sqlite`, `.judge-demo`, `.lock` and `quota.sqlite` (plus transient SQLite sidecars). The image runs as its unprivileged app user.

## Judge testing instructions

Supply the verified demo URL and preset passwords in Devpost's testing instructions. All judges share the accounts; do not promise private sessions or private chat content. These passwords are for fictional demo accounts only and are not committed to the repository.

| Role | Account | What to try |
| --- | --- | --- |
| Parent | `maya@rivera.test` | Arrange the page, describe a new card, assign/undo a chore, view family progress and Reviews |
| Parent | `sam@rivera.test` | A second personalized parent page |
| Kid | `jordan@rivera.test` | Complete a chore, open sample chats, ask a new homework question |
| Kid | `lily@rivera.test` | A younger child's page and sample conversation |

Parents share the parent demo password; kids share the kid demo password. Account/invite/calendar changes are disabled in this demo. Ordinary family authorization still applies. Opening Jordan's page while logged in as Maya does not impersonate Jordan; use a separate browser profile to test as the kid.

The prepared briefing and conversations titled “Sample” are examples. New replies use real models. All initial cards/calendar events are fictional examples too; do not portray them as verified current news or real appointments.

## Usage allowance

Defaults are 100 outbound model attempts and 50 paid-search attempts per Denver day. Change `KINLIGHT_DEMO_MODEL_CALLS` / `KINLIGHT_DEMO_SEARCH_CALLS` to positive values and restart the demo to adjust limits. At most two AI operations run concurrently. Output is limited to 2,048 tokens per model call. These are request limits, not a guaranteed dollar budget: model input lengths and prices also affect cost.

Every attempted provider call is counted before dispatch, including safety checks, retries and tool follow-ups. Failed calls are not refunded. Midnight America/Denver opens a new day's allowance. The ledger persists across logout, restart, redeployment and content reset. When exhausted, existing pages/sample chats remain readable and the banner states the next reset. Keep adequate credit and allowance for judges through October 30; increase it deliberately if needed. Provider outages remain distinct errors. Never disable the child's safety checks to save allowance.

## Reset only the demo

Reset before recording/submitting, or deliberately between evaluations. It logs out every demo session and removes visitor changes/chats. Never reset automatically during judging or delete a running database.

```sh
docker compose -f compose.demo.yaml stop judge-demo
# Read/export the same two preset password variables as in provisioning above.
docker compose -f compose.demo.yaml run --rm --no-deps \
  -e KINLIGHT_DEMO_PARENT_PASSWORD -e KINLIGHT_DEMO_KID_PASSWORD \
  --entrypoint /app/seed judge-demo -hosted-dir /kinlight-demo \
  -hosted-reset -confirm-dir /kinlight-demo
unset KINLIGHT_DEMO_PARENT_PASSWORD KINLIGHT_DEMO_KID_PASSWORD
docker compose -f compose.demo.yaml up -d judge-demo
```

The exclusive lock refuses a reset while the server is running. An invalid marker, unexpected file, symlink or uncheckpointed SQLite sidecar also stops it. If sidecars remain after an abrupt crash, restart and cleanly stop the **demo** server to let SQLite recover/checkpoint; do not delete the sidecars. Construction failures preserve old content. Verify both logins, sample review/chores, expired old sessions and unchanged allowance after reset. The demo has no backup restore step; do not point Litestream at the household bucket to repair it.

## Verification and rollback

Local checks use temporary directories and fake providers; they do not prove deployment or paid provider behavior. Run backend `make check`, frontend lint/test/build/contrast/copy checks and the OpenSpec validator. Storage tests cover marker/lock/symlink safety, unchanged household files, failure recovery, session reset and persistent quotas. Provider tests check that the legacy and Genkit paths share the allowance.

To rehearse in a local browser, create a real absolute directory ending in `kinlight-demo`, provision it with disposable test passwords, build the frontend, and run `KINLIGHT_BROWSER_DEMO_DIR=<absolute-directory> go test -run '^TestDemoBrowser$' -v -timeout 30m` from backend. This opt-in test serves fake agents/search for 25 minutes, prints its local URL and writes `/tmp/kinlight-judge-browser-url`. It is not a deployment mode or a real-provider check.

Rollback the demo independently to the last reviewed image. Its storage persists; reset only if intentionally required. Clear the public demo link if unavailable and keep honest local run instructions/video available. Never remove the family volume. Do not run Compose with `down -v` on the household application.

Use the [video and submission checklist](../openspec/changes/hackathon-demo-and-landing/video-prep.md) for rehearsal, recording, upload, judge access and owner submission. Those tasks remain separate from merging this code.
