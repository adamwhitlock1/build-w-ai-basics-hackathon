Run Go with `CGO_ENABLED=0 GOTOOLCHAIN=local` (or `make`). Never read `backend/.env`, never handle the owner's real keys, and never touch the live server: the owner does Coolify, Cloudflare and the bucket. Commit after each task.

## 1. Production mode

- [x] 1.1 Add `KINLIGHT_PUBLIC_URL` to `loadConfig`, with its validation and the production refusals (design decision 1). Pass it on: Secure cookies through `auth`, the site URL through `aikit` and `llm`. Tests:
  - an `http://` URL or a path is refused;
  - each development setting is refused in production, with its name;
  - without the URL nothing changes;
  - the cookie is Secure only in production (route test);
  - the OpenRouter site and referer follow the URL.

  Verify with `make check`.
  - Done. Production behavior hangs off `deps` (`PublicURL`, `WebDir`, `TrustCloudflare`), so route tests can set it, through a new `startServerOpts` helper. `routes` sets the cookie's Secure flag.
  - **Tests:** `parsePublicURL` (good and bad addresses), `devOnlySettings` (each dev setting named, off values ignored), the Secure cookie in production only (and always HttpOnly and SameSite=Lax), and `llm.NewClientForSite` sending the public URL as `HTTP-Referer`.
  - The Genkit side passes the same URL through `aikit.Config.SiteURL` to the OpenRouter plugin. Its tests use a scripted model with no HTTP, so the header is tested on the `llm` client. `make check` passes.
- [x] 1.2 Add the security-headers middleware, with the CSP's script hash computed from the served `index.html` (design decision 3). Tests: the headers appear only in production, and the hash matches a fixture `index.html`'s inline script. Verify with `make check`.
  - Done. A new `internal/web` package holds `InlineScriptHashes`, `ContentSecurityPolicy` and `SecurityHeaders`, and `routes` wraps everything in it in production, hashing the served `index.html` at startup. Unit tests cover the hash of a fixture page's inline script, and that external and empty scripts are ignored. A route test shows the headers only in production. `make check` passes. The real built page is checked under the policy in 4.2.

## 2. One server

- [x] 2.1 Serve `KINLIGHT_WEB_DIR` with the SPA fallback and caching, plus the `/api/` JSON 404 (design decision 2). Route tests, against a temporary directory:
  - a file;
  - a hashed asset's cache header;
  - an app route answered with `index.html`;
  - `/api/nothing` answered with JSON 404;
  - no web dir, no static routes.

  Verify with `make check`.
  - Done. `web.App` serves files from `os.DirFS`: `/assets/*` immutable for a year, other files and the page `no-cache`, and anything else, a missing asset included, answered with `index.html`. `routes` registers it as `GET /`, with a `GET /api/` JSON 404 in front. It's GET-only, so other methods on API paths keep the mux's usual 405. Route tests cover each case and the no-web-dir case. `make check` passes.

## 3. Sign-up and login

- [x] 3.1 Close sign-up after the first family in production, with `GET /api/signup` and the sign-up page's closed state (design decision 4). Tests:
  - the first sign-up works and the second is refused in production;
  - both work in development;
  - `GET /api/signup` reports the state.

  Verify with `make check`, lint and build, and the closed page in the browser.
  - Done.
    - **Backend:** `store.CountFamilies`, plus `family.Handler.CloseSignupAfterFirstFamily`, which `routes` calls in production. `GET /api/signup` answers `{"open": …}`.
    - **Tests:** in production the first sign-up works and the next gets 403 "Sign-up is closed. Ask a parent in your family for an invite."; in development two families sign up and it stays open.
    - **The sign-up page** reads the state in its loader, and when it's closed shows "Joining Kinlight" with that sentence and a link to log in.
    - **Checked in the browser:** the built binary in production mode, serving `frontend/dist` on a copy of the demo database. The closed page, fonts and theme all worked, with no CSP violations in the console, so the hashed theme script ran.
    - `make check`, lint, build and `check:copy` pass.
- [x] 3.2 Add login throttling, with the client-address rule (design decision 5). Tests:
  - the sixth failure in 15 minutes is refused, even with the right password, and with the same message whether or not the email exists;
  - success clears the email's count;
  - the address limit works across emails;
  - `CF-Connecting-IP` counts only when trusted;
  - time is injected so the 15 minutes is tested.

  Verify with `make check`.
  - Done. `auth.Throttle` is in memory with an injectable clock, and `auth.ClientIP` handles the Cloudflare rule. `routes` gives every server its own throttle, in development too (no test fails five logins). Login checks the throttle before the password, records a failure against the email and the address, and clears the email on success.
  - **Tests:**
    - unit: the limit, the window ending at 15 minutes, Clear, and the trusted, untrusted and malformed header;
    - route: the sixth try is refused even with the right password; a blocked address gives an unknown email the same message; five emails from one Cloudflare address block it, while Maya from another address still logs in.
  - `make check` passes.
  - **Detail:** the block lasts until the oldest of the five failures is 15 minutes old, so at most 15 minutes. Attempts while blocked don't extend it.

## 4. The image and Compose

- [x] 4.1 Add the `Dockerfile`, `.dockerignore`, `deploy/start.sh` and `deploy/litestream.yml` (design decision 6), with Litestream v0.5.17 pinned by checksum for amd64 and arm64. Verify locally:
  - `docker build` succeeds for the local architecture, and with `--platform linux/amd64`;
  - the container runs as uid 10001;
  - `/api/health` passes its health check;
  - without a bucket it warns and serves.
  - Done (2026-10-25):
    - **Builds:** the image builds natively (arm64) and with `--platform linux/amd64`. Each architecture's Litestream archive is checked against the hash from the release's `checksums.txt`, which I also checked against the downloaded files; the amd64 build reported "OK", and `litestream version` printed 0.5.17.
    - **Running it:** the container runs as uid 10001 and Docker marks it healthy. Without `LITESTREAM_BUCKET` it logs "the database is NOT being backed up" and serves health, the app's page and a static file.
    - **Found and fixed:** the server listened only on 127.0.0.1, which inside a container nothing outside can reach. A new `KINLIGHT_HOST` (default 127.0.0.1) lets the image set 0.0.0.0.
    - **For local runs:** the server checks the OpenRouter key at startup, and a real key is never used here. The local run points `OPENROUTER_BASE_URL` at a closed port, which the preflight treats as unreachable (a warning), not as a bad key.
- [x] 4.2 Add `compose.yaml` and `compose.test.yaml` with MinIO, and `deploy/check.sh` (design decisions 7 and 8). Run the check and record it here:
  - the app and its assets are served with the headers, and no CSP violations show in the browser console on the main pages;
  - sign-up closes after the first family;
  - the database replicates to MinIO, and after the volume is removed the next start restores the account.
  - Done (2026-10-25). `deploy/check.sh` passed all 9 checks:
    - the page's CSP with the theme script's hash;
    - HSTS;
    - immutable assets;
    - the API 404;
    - the first sign-up, and a second refused;
    - a restore on the next start after the volume was deleted;
    - the restored account logging in;
    - sign-up still closed.
  - **In the browser:** on the same stack, signing up, setting up the family, then home, the Assistant, Reviews and not-found showed no CSP violations. The one console error is the expected 401 from asking who's logged in first.
  - **Changes from the plan:**
    - The MinIO images are no longer published on Docker Hub, so SeaweedFS 4.50 stands in as the S3 store (`compose.test.yaml`, test identity in `deploy/s3-test.json`).
    - The bucket creator tolerates an existing bucket, and the restart in the check skips dependencies; re-running the creator had blocked the restart.
    - SeaweedFS's health check is a port check: busybox `wget` exits 1 on an HTTP error.

## 5. Docs and wrap-up

- [x] 5.1 Write `docs/deploy.md`:
  - Coolify (a new resource from the repo using `compose.yaml`, the environment variables and what each is);
  - the backup bucket and its keys;
  - the Cloudflare Tunnel to the service, with `KINLIGHT_TRUST_CLOUDFLARE` and why the port mustn't be published;
  - Cloudflare Access for allowed emails, and how to lift it later;
  - the first run: sign up as a parent, set up the family, invite everyone;
  - the kids' zero-retention key and `make ai-smoke` before kids use chat;
  - updating;
  - a restore drill;
  - what never runs on the homelab.

  Link it from the README, and update the roadmap's series 2 as done. Verify by reading it.
  - Done. `docs/deploy.md` covers the bucket (Cloudflare R2 suggested, since the owner is on Cloudflare), Coolify with a table of every variable, the tunnel (the cloudflared service on the same Docker network, and the trust flag only once the tunnel is the only way in), Access with a one-time-PIN email allowlist and how to lift it, the first run, the kids' key with `make ai-smoke` and `make ai-eval-kids` first, updating, restores (a lost volume and a point in time), the drill, and what never runs on the homelab. The README's Run section links it, and the roadmap marks series 2 done.
  - **Changed while writing:** the drill restores into a throwaway container that serves the copy and never replicates, rather than duplicating the Coolify resource. Two copies writing to one backup path could corrupt it.
  - **For the owner to confirm:** Coolify's exact menu names vary between versions, so the guide names what to look for.
- [x] 5.2 Run `openspec validate homelab-deployment --strict`, `make check`, and the frontend lint, `npm test`, contrast check, copy check and build. Push, open the PR, and confirm CI passes on it. Add the "Homelab deployment" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
  - Progress (2026-10-25): the change and all 14 specs validate strictly, and `make check` passes. The frontend lint is clean, `npm test` passes 11 tests, `check:contrast` passes 44 pairings, `check:copy` is clean, and the build passes. `deploy/check.sh` passed earlier (4.2). The Post-PoC line is in both files. Pushing, the PR and CI are next.
  - CI passed on PR #20, and the owner merged it (2026-10-25).
