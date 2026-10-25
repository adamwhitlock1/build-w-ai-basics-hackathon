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

- [ ] 3.1 Close sign-up after the first family in production, with `GET /api/signup` and the sign-up page's closed state (design decision 4). Tests:
  - the first sign-up works and the second is refused in production;
  - both work in development;
  - `GET /api/signup` reports the state.

  Verify with `make check`, lint and build, and the closed page in the browser.
- [ ] 3.2 Add login throttling, with the client-address rule (design decision 5). Tests:
  - the sixth failure in 15 minutes is refused, even with the right password, and with the same message whether or not the email exists;
  - success clears the email's count;
  - the address limit works across emails;
  - `CF-Connecting-IP` counts only when trusted;
  - time is injected so the 15 minutes is tested.

  Verify with `make check`.

## 4. The image and Compose

- [ ] 4.1 Add the `Dockerfile`, `.dockerignore`, `deploy/start.sh` and `deploy/litestream.yml` (design decision 6), with Litestream v0.5.17 pinned by checksum for amd64 and arm64. Verify locally:
  - `docker build` succeeds for the local architecture, and with `--platform linux/amd64`;
  - the container runs as uid 10001;
  - `/api/health` passes its health check;
  - without a bucket it warns and serves.
- [ ] 4.2 Add `compose.yaml` and `compose.test.yaml` with MinIO, and `deploy/check.sh` (design decisions 7 and 8). Run the check and record it here:
  - the app and its assets are served with the headers, and no CSP violations show in the browser console on the main pages;
  - sign-up closes after the first family;
  - the database replicates to MinIO, and after the volume is removed the next start restores the account.

## 5. Docs and wrap-up

- [ ] 5.1 Write `docs/deploy.md`:
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
- [ ] 5.2 Run `openspec validate homelab-deployment --strict`, `make check`, and the frontend lint, `npm test`, contrast check, copy check and build. Push, open the PR, and confirm CI passes on it. Add the "Homelab deployment" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
