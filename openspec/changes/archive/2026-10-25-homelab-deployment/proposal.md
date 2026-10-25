## Why

Kinlight has only ever run on a laptop, as two dev servers. The family is meant to use it every day, on the owner's homelab, reachable from anywhere at `https://kinlight.codestuff.io`. That puts it on the public web, which the code isn't ready for:
- the session cookie isn't marked HTTPS-only;
- anyone who finds the page can sign up;
- logins aren't throttled;
- the Go server doesn't serve the web app;
- there are no backups.

This is the last change of series 2.

## What Changes

**Owner decisions.** Kinlight is deployed with Docker Compose through Coolify on a Linux x86-64 machine. It's exposed through a Cloudflare Tunnel, with Cloudflare Access (allowed emails) in front for now; the owner configures Access, and the app's own login stays the real gate. The database is backed up continuously by Litestream to S3-compatible storage.

- **Production mode.** Setting `KINLIGHT_PUBLIC_URL` (the HTTPS address people use) turns it on:
  - session cookies are HTTPS-only;
  - OpenRouter is told the real site;
  - security headers are sent, including a Content-Security-Policy that allows the pre-paint theme script by its hash;
  - the server refuses to start with any development-only setting: dev calendar fixtures, `KINLIGHT_KIDS_ALLOW_RETENTION`, or Genkit's reflection server.
- **One server.** The Go server serves the built web app as well as the API, so there's a single process and port. Client-side routes fall back to the app.
- **In production, sign-up closes once a family exists.** The first account creates the family. After that, new members join only through an invite, so the public page can't create accounts. Without production mode (local dev and the tests), sign-up stays open, as today.
- **Login throttling.** Repeated failed logins are slowed down and then refused for a while, per email and per client address. Behind Cloudflare, the client address comes from Cloudflare's header, trusted only when configured.
- **A container image** (multi-stage `Dockerfile`):
  - it builds the frontend and a static Go binary;
  - it runs as a non-root user in a small image;
  - it bundles a pinned, checksum-verified Litestream.
- **`compose.yaml` for Coolify:**
  - a volume for the database;
  - a health check on `/api/health`;
  - at start, restore from the backup if the database is missing, then serve under `litestream replicate`, so every write is backed up.
- **`docs/deploy.md`.** Step by step for the owner:
  - Coolify and its environment variables, including the kids' zero-retention key;
  - the Cloudflare Tunnel and Access;
  - the backup bucket;
  - the first run: create the family, invite everyone;
  - a restore drill;
  - what must never run on the homelab (Genkit's Dev UI and reflection server, the fixtures).

Out of scope:
- The owner does the Coolify, Cloudflare and bucket setup and enters every secret. Claude never handles real keys or the live server.
- Multi-family hosting, email sending, and monitoring beyond the health check and logs.

## Capabilities

### New Capabilities
- `production-hosting`: production mode and its guards, one server for the app and API, closed sign-up, login throttling, and backups with restore.

### Modified Capabilities
<!-- none: no existing spec covers accounts or hosting -->

## Impact

- **Backend:**
  - `main.go` config: `KINLIGHT_PUBLIC_URL`, `KINLIGHT_WEB_DIR`, `KINLIGHT_TRUST_CLOUDFLARE`, and the production checks;
  - `auth` (the cookie's Secure flag);
  - `family` (signup closes once a family exists);
  - a login throttle;
  - static file serving with SPA fallback;
  - a security-headers middleware;
  - `aikit` and `llm` (the site URL);
  - route tests for each.
- **Frontend:** the sign-up page explains that joining is by invite when sign-up is closed (`GET /api/signup` says whether it's open).
- **New files:** `Dockerfile`, `.dockerignore`, `compose.yaml`, `deploy/litestream.yml` and its start script, `docs/deploy.md`.
- **Dev:**
  - local dev and the tests keep working unchanged without `KINLIGHT_PUBLIC_URL`;
  - a local Docker + MinIO check verifies the image, the restore and the replication.
