> Archive note (2026-10-25): This records the original deployment plan and its implementation. The guide was subsequently changed to Backblaze B2 for backups, and merged PR #21 replaced the Docker-network tunnel approach with host-based cloudflared and `127.0.0.1:8080:8080`. See `docs/deploy.md` and `openspec/specs/production-hosting/spec.md` for the current setup. Local checks and PR #20 CI are recorded below; successful live tunnel access and production backup/restore have not been verified by this housekeeping review.

## Context

See proposal.md for why. What exists today:

- **Server.** It reads `.env` and the environment in `loadConfig`, and listens on `PORT` (8080). It serves only `/api/*`; the frontend is a Vite dev server that proxies `/api`. `GET /api/health` exists.
- **Sessions.** `scs`, with `Cookie.Secure = false` hard-coded (`internal/auth/session.go`), SameSite=Lax, HttpOnly.
- **Cross-site writes.** They're refused by `http.CrossOriginProtection` plus a JSON-body rule. Through a Cloudflare Tunnel, the browser's `Origin` is `https://kinlight.codestuff.io` and the request's `Host` is the same, as long as the tunnel keeps the Host header (cloudflared does by default).
- **AI site URL.** OpenRouter's site URL is hard-coded to `http://localhost:5174` in `aikit/kit.go` and `llm/client.go`.
- **Sign-up and login.** `POST /api/signup` creates an account and a family for anyone. Logins aren't throttled.
- **SQLite.** modernc, in WAL mode (`journal_mode(WAL)` in the DSN), which Litestream requires.
- **Dev-only settings.** `KINLIGHT_DEV_FIXTURES`, `KINLIGHT_KIDS_ALLOW_RETENTION`, and Genkit's reflection server (`GENKIT_ENV=dev` or `GENKIT_REFLECTION_ENABLED`). Today they only log warnings.
- **The frontend's own requests.** `index.html` has one inline pre-paint script. Card thumbnails load from other https sites. The CSS uses `data:` SVG masks.
- **Tools.** Docker 29 and Compose are available locally (arm64). The homelab is x86-64, and Coolify builds the Dockerfile on it. Litestream's latest release is v0.5.17 (2026-08-31).

## Goals / Non-Goals

**Goals:**
- One image, one process tree, one port.
- Production can't run with a development setting by accident.
- Everything the owner does by hand is written down.
- The restore is tested before the family relies on it.

**Non-Goals:**
- Kubernetes, multiple replicas, or a separate web server; SQLite and one process are the architecture.
- Cloudflare configuration as code.
- Verifying Cloudflare Access's JWT in the app.
- Email.
- Monitoring dashboards.

## Decisions

### 1. Production mode is the public URL

`KINLIGHT_PUBLIC_URL` turns production on; there's no separate `KINLIGHT_ENV`, since the URL is needed anyway. `loadConfig` validates it: an absolute `https://` URL with no path.

In production `loadConfig` also refuses:
- `KINLIGHT_DEV_FIXTURES`;
- `KINLIGHT_KIDS_ALLOW_RETENTION`;
- `GENKIT_ENV=dev`, and any `GENKIT_REFLECTION_ENABLED`.

It names the setting, as the old-names guard does.

The URL then feeds:
- `auth.New(..., secure bool)` for the cookie's Secure flag;
- `aikit.Config.SiteURL` and `llm` for OpenRouter's site and referer (dev keeps localhost);
- the headers middleware.

### 2. Serving the app: a directory, with SPA fallback

`KINLIGHT_WEB_DIR` points at the built `frontend/dist` (`/app/web` in the image). When it's set, `routes` adds a `GET /` handler, the lowest-priority pattern:
- **A file in the directory:** served as-is. `/assets/*` (hashed names) get `Cache-Control: public, max-age=31536000, immutable`.
- **Anything else:** `index.html`, with `Cache-Control: no-cache`.
- **API paths:** `/api/...` paths never fall back. A new `/api/` catch-all returns the existing JSON 404.

*Alternative:* `go:embed` the dist into the binary. That's one fewer file, but `go build` would then need the frontend built first, which breaks `make check` and CI for the backend alone. Rejected.

### 3. Security headers

A middleware, in production only, sets:
- `Content-Security-Policy`:
  - `default-src 'self'`;
  - `script-src 'self' 'sha256-<inline>'`;
  - `style-src 'self' 'unsafe-inline'` (React writes `style` attributes for light levels);
  - `img-src 'self' data: https:` (thumbnails and the CSS masks);
  - `font-src 'self'`;
  - `connect-src 'self'`;
  - `frame-ancestors 'none'`, `base-uri 'self'`, `form-action 'self'`, `object-src 'none'`.
- `Strict-Transport-Security: max-age=31536000`.
- `X-Content-Type-Options: nosniff`.
- `Referrer-Policy: strict-origin-when-cross-origin`.
- `Permissions-Policy` turning off the camera, microphone and geolocation.

The inline script's hash is computed at startup, from every inline `<script>` in the served `index.html`. A change to the script can't silently break the policy, and a page with no web dir gets no hash.

### 4. Sign-up closes after the first family, in production

`store.CountFamilies(ctx)`. In production, `family.Handler`'s signup refuses with 403 "Sign-up is closed. Ask a parent in your family for an invite." when the count is above 0.

`GET /api/signup` (public) answers `{"open": bool}`. The sign-up page reads it and, when closed, shows that sentence and a link to log in instead of the form.

The check and the creation aren't one transaction. Two simultaneous first sign-ups could create two families, which is harmless for one household and fixable by hand. That's accepted.

### 5. Login throttling, in memory

`internal/auth/throttle.go`: a mutex-guarded map of failure timestamps, keyed by `email:<addr>` and `ip:<addr>`, pruned as it's read.
- **Before checking the password:** if either key has 5 failures in the last 15 minutes, answer 429 "Too many tries. Wait 15 minutes and try again."
- **After a failure:** record it on both keys.
- **On success:** clear the email key.

It's memory only: a restart clears it, which is acceptable for one household.

The client address is the first of:
- `CF-Connecting-IP`, when `KINLIGHT_TRUST_CLOUDFLARE=1` (only set when the tunnel is the only way in);
- otherwise, `r.RemoteAddr`'s host.

### 6. The image

The multi-stage `Dockerfile`:
1. **Frontend:** `node:22-alpine` runs `npm ci` and `npm run build`.
2. **Backend:** `golang:1.27.3-alpine` runs `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`.
3. **Litestream:** downloads `litestream-v0.5.17-linux-${TARGETARCH}.tar.gz` and verifies its SHA-256, which the Dockerfile pins per architecture (amd64 and arm64).
4. **Runtime:** `alpine` (for `sh`, CA certificates and tzdata) with:
   - `/app/kinlight`, `/app/web`, `/usr/local/bin/litestream`, `/etc/litestream.yml` and `/app/start.sh`;
   - user `kinlight` (uid 10001);
   - `/data` as a volume;
   - `ENV DB_PATH=/data/kinlight.db KINLIGHT_WEB_DIR=/app/web PORT=8080`, `EXPOSE 8080`;
   - a `HEALTHCHECK` that runs `wget -qO- http://127.0.0.1:8080/api/health`.

`deploy/start.sh`:
- **With `LITESTREAM_BUCKET` set:**
  - if the database file is missing, `litestream restore -if-replica-exists -o "$DB_PATH" "$DB_PATH"`;
  - then `exec litestream replicate -exec /app/kinlight`. Litestream supervises the server and forwards signals.
- **Without the bucket:** it logs a loud warning and `exec /app/kinlight`.

`deploy/litestream.yml` is one database at `$DB_PATH` with an S3 replica. Its settings come from the environment: `LITESTREAM_BUCKET`, `LITESTREAM_PATH` (default `kinlight`), `LITESTREAM_ENDPOINT` and `LITESTREAM_REGION` for non-AWS stores, and the access keys through `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY`.

### 7. `compose.yaml` for Coolify

One service, `kinlight`:
- `build: .`;
- a named volume at `/data`;
- `expose: 8080` (Coolify's proxy and the tunnel reach it on its network);
- environment variables listed by name with no values: the owner fills them in Coolify's UI. That's the OpenRouter keys, `BRAVE_API_KEY`, the models, `KINLIGHT_PUBLIC_URL`, `KINLIGHT_TRUST_CLOUDFLARE`, and the `LITESTREAM_*` settings;
- a health check;
- `restart: unless-stopped`.

`compose.test.yaml` adds MinIO and a one-shot bucket creator, for the local check only.

### 8. Verifying before the owner deploys

A scripted local check (`deploy/check.sh`, documented, not run in CI, since it needs Docker):
1. Build the image, and start it with MinIO through `compose.test.yaml` and a test `KINLIGHT_PUBLIC_URL`.
2. Sign up, then confirm a second sign-up is refused and the app's page and assets are served with their headers.
3. Wait for replication, remove the volume, start again, and confirm the account came back.

The Go parts have unit and route tests like everything else.

## Risks / Trade-offs

- **[Cloudflare Access hides the app's own login page behind Cloudflare's.]** → Intended for now. Both gates apply, and the docs say how to lift Access later.
- **[`KINLIGHT_TRUST_CLOUDFLARE` set while the port is also reachable directly would let anyone spoof their address.]** → The docs say to expose the port only to the tunnel (no published port in `compose.yaml`) and to set the flag only then.
- **[A checksum or release-name change at download time.]** → Pinned version and hashes. The build fails loudly, rather than running an unverified binary.
- **[The image runs Litestream as the parent process.]** → `-exec` is Litestream's documented supervisor mode; the server's exit stops the container, and Coolify restarts it.
- **[The CSP breaks a feature that loads something from elsewhere.]** → The browser check in tasks walks every page with the policy on, watching the console for violations.

## Migration Plan

1. The owner follows `docs/deploy.md`: bucket, Coolify app from the repo, environment, tunnel, Access, the first sign-up, invites.
2. **Rollback:** redeploy the previous image in Coolify; the volume and the backup are untouched. A bad database is recovered with `litestream restore -timestamp`.
