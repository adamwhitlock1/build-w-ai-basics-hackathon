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

The backups go to Backblaze B2, through its S3-compatible API.

1. **Create the bucket.** In the Backblaze web console, open **B2 Cloud Storage → Buckets → Create a Bucket**:
   - **Name:** for example `kinlight-backups`. B2 bucket names are global, so add something of your own if it's taken.
   - **Files in Bucket are:** **Private**.
   - **Default Encryption:** nothing to choose. B2 encrypts new files at rest with its own keys (SSE-B2), which changes nothing for Litestream.
   - **Object Lock:** **Disable**. Litestream deletes old backup files as it compacts them, which locked files would block.
2. **Note the bucket's S3 endpoint.** It's shown under the bucket and looks like `s3.us-west-004.backblazeb2.com`. The part after `s3.` and before `.backblazeb2.com` is the bucket's region, here `us-west-004`. You need both.
3. **Keep only the latest version of each file.** B2 keeps every version of every file by default, and "deleting" a file only hides it. Litestream deletes old backup files as it compacts them, so without this setting the bucket would grow forever. On the bucket, open **Lifecycle Settings** and choose **Keep only the last version of the file**.
4. **Create an application key for this bucket only.** In **Application Keys → Add a New Application Key**:
   - **Name:** `kinlight-litestream`;
   - **Allow access to Bucket(s):** only `kinlight-backups`;
   - **Type of Access:** **Read and Write**;
   - leave the file-name prefix and duration empty.

   B2 shows the **keyID** and the **applicationKey** once: copy both now. The keyID is the access key ID, and the applicationKey is the secret access key.

Don't use your account's master application key: a key limited to this bucket can't touch anything else if it leaks.

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
   | `LITESTREAM_ENDPOINT` | The bucket's S3 endpoint with `https://`, e.g. `https://s3.us-west-004.backblazeb2.com`. |
   | `LITESTREAM_REGION` | The region from that endpoint, e.g. `us-west-004`. It must match, or B2 refuses the requests. |
   | `LITESTREAM_FORCE_PATH_STYLE` | `false`. B2 accepts the usual bucket-in-the-hostname requests. |
   | `LITESTREAM_ACCESS_KEY_ID` | The application key's **keyID** from step 1. |
   | `LITESTREAM_SECRET_ACCESS_KEY` | The application key's **applicationKey** from step 1. |
   | `LITESTREAM_PATH` | Leave as `kinlight`. It's the folder inside the bucket. |

   Mark the keys as secrets in Coolify so they don't show in its logs.
3. **Let Compose publish the local port; don't add a domain or another port mapping in Coolify.** `compose.yaml` maps `127.0.0.1:1337` on the host to port 8080 in the container. cloudflared runs directly on that host and reaches Kinlight through this loopback address (step 3). Keep port 1337 free on the host.
4. **Deploy.** The build takes a few minutes the first time. When it's up, Coolify shows the container as healthy: the image checks `/api/health` every 30 seconds.

**What the log should show:** "kinlight API listening", Litestream starting replication, and, on a first run, "no database … restoring from the backup if there is one". If it says the database is NOT being backed up, `LITESTREAM_BUCKET` is empty.

**If it refuses to start**, the log says why: a development setting in production, a non-`https` public URL, or a key OpenRouter rejected.

**If Litestream can't reach B2,** its lines in the log name the problem:
- **"AuthorizationHeaderMalformed" or a signature error:** `LITESTREAM_REGION` doesn't match the endpoint.
- **"AccessDenied":** the key isn't allowed on this bucket, or it's read-only.
- **"NoSuchBucket":** `LITESTREAM_BUCKET` doesn't match the bucket's name.

The server keeps running either way, but nothing is backed up until it's fixed. You can check in the B2 console that files appear under `kinlight/` within a minute of the first sign-up.

## 3. The Cloudflare Tunnel

The tunnel connects outward from the homelab to Cloudflare, so nothing on your router opens.

1. **Create the tunnel.** In Cloudflare **Zero Trust → Networks → Tunnels**, create a tunnel (cloudflared) named for example `homelab`, and copy its token.
2. **Run `cloudflared` directly on the Coolify server.** Install it as a host service using Cloudflare's instructions and the tunnel token. If it is already running on that server, use the existing tunnel.
3. **Add a public hostname.** In the tunnel's settings: `kinlight.codestuff.io` → **HTTP** → `http://127.0.0.1:1337`. This reaches the port Compose publishes on the host. Leave the host header as it is: Kinlight checks that a request's `Origin` matches its `Host`.
4. **Set `KINLIGHT_TRUST_CLOUDFLARE=1` and redeploy.** Login throttling then counts each visitor by their real address, from Cloudflare's `CF-Connecting-IP` header. Only set it while the tunnel is the only way in: anyone who can reach the port directly could fake that header.

After deploying, run `curl --fail http://127.0.0.1:1337/api/health` on the server to check the local connection. Then open `https://kinlight.codestuff.io`: the sign-in page should load after Cloudflare Access authentication, if enabled.

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
- **A port published to the LAN or all interfaces while `KINLIGHT_TRUST_CLOUDFLARE=1` is set.** Keep the mapping bound to `127.0.0.1`; local processes on the host must be trusted.

With `KINLIGHT_PUBLIC_URL` set, Kinlight refuses to start if any of the first three are on.
