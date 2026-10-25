# Deploying Kinlight on the homelab

Kinlight runs as one container: the Go server serving the web app beside its API, under Litestream, which backs the database up to S3-compatible storage as it changes. Coolify builds and runs it from this repository's `compose.yaml`. A Cloudflare Tunnel makes it reachable at `https://kinlight.codestuff.io` without opening a port, and Cloudflare Access can keep everyone but the family out while it settles in.

You'll need:
- a Linux x86-64 machine with Coolify;
- a Cloudflare account with `codestuff.io`;
- an S3-compatible bucket;
- the API keys from `backend/.env.example`.

Keys never go in the repository; they go into Coolify's environment settings.

Before deploying a new version, you can check the image on a laptop with Docker: `deploy/check.sh` builds it, runs it against a local S3 store, and tests sign-up, the security headers, and a restore from backup.

## 1. A bucket for backups

Any S3-compatible store works. Cloudflare R2 is the simplest, since you're already on Cloudflare:

1. **Create the bucket.** In the Cloudflare dashboard, open **R2 → Create bucket**, named for example `kinlight-backups`.
2. **Create an API token.** Under **R2 → Manage API tokens**, create a token with **Object Read & Write** on that bucket only. Note the **Access Key ID**, the **Secret Access Key**, and the S3 endpoint, which looks like `https://<account id>.r2.cloudflarestorage.com`.

With Backblaze B2, AWS or another store, create a bucket and a key limited to it, and note its endpoint and region.

## 2. The app in Coolify

1. **Create the resource.** In your Coolify project: **New Resource → Public/Private Repository**. Pick this repository and branch `main`, build pack **Docker Compose**, compose file `/compose.yaml`.
2. **Fill in the environment variables.** Coolify lists every variable from `compose.yaml` under **Environment Variables**. Fill them in. The ones marked required stop the deploy if empty.

   | Variable | What to set |
   | --- | --- |
   | `KINLIGHT_PUBLIC_URL` | `https://kinlight.codestuff.io`. This turns on production mode. |
   | `KINLIGHT_TRUST_CLOUDFLARE` | `1`, once the tunnel is the only way in (step 3). |
   | `OPENROUTER_API_KEY` | Your main OpenRouter key. |
   | `BRAVE_API_KEY` | Your Brave Search key. |
   | `OPENROUTER_KIDS_API_KEY` | The kids' separate key (section 5). Empty keeps kid chat and "About the kids" off. |
   | `OPENROUTER_MODEL`, `GENKIT_MODEL`, `KIDS_CHAT_MODEL`, `KIDS_SAFETY_MODEL`, `PARENT_CHAT_MODEL` | Leave empty for the defaults in the README. |
   | `LITESTREAM_BUCKET` | The bucket's name. |
   | `LITESTREAM_ENDPOINT` | The S3 endpoint, e.g. `https://<account id>.r2.cloudflarestorage.com`. Empty means AWS. |
   | `LITESTREAM_REGION` | `auto` for R2, or your store's region. |
   | `LITESTREAM_FORCE_PATH_STYLE` | `true` if your store needs path-style URLs (MinIO, SeaweedFS); otherwise `false`. |
   | `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY` | The bucket key from step 1. |
   | `LITESTREAM_PATH` | Leave as `kinlight`. It's the folder inside the bucket. |

   Mark the keys as secrets in Coolify so they don't show in its logs.
3. **Don't give the service a domain or a published port in Coolify.** The tunnel reaches it on Coolify's Docker network (step 3). `compose.yaml` only exposes port 8080 to that network.
4. **Deploy.** The build takes a few minutes the first time. When it's up, Coolify shows the container as healthy: the image checks `/api/health` every 30 seconds.

**What the log should show:** "kinlight API listening", Litestream starting replication, and, on a first run, "no database … restoring from the backup if there is one". If it says the database is NOT being backed up, `LITESTREAM_BUCKET` is empty.

**If it refuses to start**, the log says why: a development setting in production, a non-`https` public URL, or a key OpenRouter rejected.

## 3. The Cloudflare Tunnel

The tunnel connects outward from the homelab to Cloudflare, so nothing on your router opens.

1. **Create the tunnel.** In Cloudflare **Zero Trust → Networks → Tunnels**, create a tunnel (cloudflared) named for example `homelab`, and copy its token.
2. **Run `cloudflared` on the homelab in Coolify.** Coolify has a one-click **Cloudflared** service: add it with the token. It must share a Docker network with the Kinlight service. In Coolify, connect both to the same network, or use the project's network.
3. **Add a public hostname.** In the tunnel's settings: `kinlight.codestuff.io` → **HTTP** → `http://<kinlight service name>:8080`. The service name is the container's name on that network, which Coolify shows under the resource. Leave the host header as it is: Kinlight checks that a request's `Origin` matches its `Host`.
4. **Set `KINLIGHT_TRUST_CLOUDFLARE=1` and redeploy.** Login throttling then counts each visitor by their real address, from Cloudflare's `CF-Connecting-IP` header. Only set it while the tunnel is the only way in: anyone who can reach the port directly could fake that header.

Open `https://kinlight.codestuff.io`: the sign-in page should load.

## 4. Cloudflare Access, while it settles in

Access asks for a one-time email code before anyone reaches Kinlight. Kinlight's own login still applies after it.

1. **Create the application.** In **Zero Trust → Access → Applications → Add an application → Self-hosted**, use the domain `kinlight.codestuff.io`.
2. **Add a policy.** **Allow**, with **Include → Emails**: each family member's email address.
3. **Choose the login method.** Leave **One-time PIN** as the identity provider unless you have another.

To open Kinlight up later, delete the application, or change its policy to **Bypass → Everyone**. Kinlight's login, closed sign-up and throttling remain.

## 5. First run

1. **Create the family.** Open `https://kinlight.codestuff.io/signup` and create the first account: a parent. In production only the first account can sign up. After that, the sign-up page says to ask for an invite.
2. **Set it up.** Name the family and add your interests, location and calendar.
3. **Invite everyone.** In the sidebar, **Invite member**: their email, and **Parent** or **Kid**. Kinlight doesn't send email; copy each invite link and send it yourself. Each person opens it and chooses a password.
4. **Set each kid's birth year** on their page. It sets how the kid agent talks with them.
5. **Turn on kid chat.** Kids' conversations must only go to providers that keep nothing. Before kids use chat:
   1. Create a separate OpenRouter key for the kids, with credits.
   2. On your laptop, put it in `backend/.env` as `OPENROUTER_KIDS_API_KEY` and run `make ai-smoke` and `make ai-eval-kids` from `backend/`. Both check the real zero-retention routing.
   3. Set the same key in Coolify and redeploy.

## Updating

Push to `main`, then **Redeploy** in Coolify, or turn on Coolify's automatic deploys for the repository. The database volume and the backup stay. The container restarts on the new image, and every database migration runs at start.

## Backups and restore

Litestream sends each change to the bucket within seconds, takes a full snapshot daily, and keeps enough to restore to a recent point. Restores:
- **Lost volume:** start the container. With no database file, it restores from the bucket before serving.
- **Back to a point in time** (say, before a mistake):
  1. Stop the service in Coolify.
  2. Run a one-off container from the same image, with the volume and the same environment:

     ```bash
     litestream restore -config /etc/litestream.yml -timestamp 2026-10-25T08:00:00Z -o /data/restored.db /data/kinlight.db
     ```

  3. Replace `kinlight.db` with `restored.db` on the volume, and start the service.

**The restore drill.** Do it once now, and again after any change to the backup settings. It restores the backup into a throwaway container that serves the copy and never writes back to the bucket:

1. **Make an env file.** On the homelab, create `kinlight.env` with the same `LITESTREAM_*`, `OPENROUTER_API_KEY`, `BRAVE_API_KEY` and `KINLIGHT_PUBLIC_URL` values as in Coolify.
2. **Restore and serve the copy.** Run the image Coolify built. Its name is under the resource, or in `docker images`.

   ```bash
   docker run --rm -p 127.0.0.1:18090:8080 --env-file kinlight.env --entrypoint sh <image> -c 'litestream restore -config /etc/litestream.yml -o /tmp/drill.db "$DB_PATH" && DB_PATH=/tmp/drill.db exec /app/kinlight'
   ```

3. **Log in.** From your laptop, open an SSH tunnel (`ssh -L 18090:127.0.0.1:18090 <homelab>`), then visit `http://127.0.0.1:18090` and log in. Your family, pages and chats should be there.
4. **Clean up.** Stop the container with Ctrl-C, and delete `kinlight.env`.

## Never on the homelab

- **`GENKIT_ENV=dev` or `GENKIT_REFLECTION_ENABLED`, or the Genkit Dev UI (`make dev-ui`).** The reflection server runs any AI flow without a login.
- **`KINLIGHT_DEV_FIXTURES`:** fake calendars for the demo family.
- **`KINLIGHT_KIDS_ALLOW_RETENTION`:** kid chat on providers that may keep what kids write.
- **A published port while `KINLIGHT_TRUST_CLOUDFLARE=1` is set.**

With `KINLIGHT_PUBLIC_URL` set, Kinlight refuses to start if any of the first three are on.
