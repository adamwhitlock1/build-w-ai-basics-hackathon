For work in `internal/aikit`, load the `developing-genkit-go` skill first; the design wins where they differ. Run every Go command with `CGO_ENABLED=0 GOTOOLCHAIN=local` (or through `make`). Never read `backend/.env`. Commit after each task.

## 1. Rename the code

These tasks are design decision 7, pass 1: mechanical, with no visible change.

- [x] 1.1 Rename the Go module to `kinlight` and rewrite every `porchlight/` import. Verify with `gofmt -l .` (empty) and `make check`.
- [x] 1.2 Rename the settings and files:
  - the six `PORCHLIGHT_*` variables → `KINLIGHT_*`;
  - `porchlight_session` → `kinlight_session`;
  - `porchlight.db*` → `kinlight.db*` (the default, the Makefile, `.gitignore`, `.env.example`, tests);
  - `fixtures.porchlight.test` → `fixtures.kinlight.test`;
  - `bin/porchlight` → `bin/kinlight`.

  Add the startup guard: old variable names and a `porchlight.db` path stop the server with the replacement named. The fixture host is part of the `demo-data` spec, so it gets a delta spec (added during apply). Test each old variable and the old path with `t.Setenv`, and check that the error names the new setting and never a value. Verify with `make check`.
- [x] 1.3 Rename the frontend: the package name, `PorchlightConfig`, `PorchlightData`, `usePorchlightPuck` and its file, and `vite.config.ts`'s `PORCHLIGHT_WEB_PORT` and `PORCHLIGHT_API`. Verify with lint and build.
- [x] 1.4 Rename the name in repo files: CI comments, the OpenSpec `config.yaml` context (the name only), the main specs' Purpose lines, and `frontend/README.md`. Verify with `openspec validate --specs --strict`.
- [x] 1.5 On the owner's machine:
  - `mv backend/porchlight.db backend/kinlight.db`;
  - rebuild the demo with `make demo`;
  - delete `porchlight.db.demo`.

  Ask the owner to rename the `PORCHLIGHT_*` variables and `DB_PATH` in their `backend/.env`, without reading it. Verify that the server starts on the renamed files. If it stops instead, the guard's message names what's left to rename.
  - Done (2026-10-25):
    - `porchlight.db` is now `kinlight.db`.
    - `make demo` rebuilt `kinlight.db.demo`, and its four calendar links are on `fixtures.kinlight.test`.
    - `porchlight.db.demo` and the old `bin/porchlight` are deleted.
    - The server started with the owner's `.env` and served `db=kinlight.db`, with no guard message, so the `.env` needed no renames.

## 2. Themes and fonts

- [x] 2.1 Put the brief's Daybreak values in `@theme` and the Porch light overrides under `:root[data-theme="dark"]` (design decisions 1 and 2), with `color-scheme` per theme. Add the new tokens: `on-primary`, `on-danger`, `control`, `avatar-1` to `avatar-3`, `glass-core`. Verify with `npm run build`, and by toggling `data-theme` in the browser devtools on the demo: every surface changes, and nothing stays light in Porch light.
- [x] 2.2 Add `frontend/scripts/contrast.mjs` and `npm run check:contrast` (design decision 3), and run it in the CI frontend job. Adjust any brief value that fails, keeping its hue, and record each adjustment here. Verify that it passes, and that it fails when a pairing is made to fail on purpose.
  - Done: 30 pairings, all passing in both themes. The brief's own values needed no change.
  - Added during apply: a `control` token for field and composer edges (Daybreak `#9a8670`, 3.4:1 on surface; Porch light `#7a6857`, 3.2:1). The brief's `line` is 1.3:1, so it stays for decorative separators only.
  - Made to fail on purpose (Porch light `control` at `#4a3d33`): it named both failing pairings and exited 1.
- [x] 2.3 Replace raw colors in components:
  - `text-white` → `text-on-primary` or `text-on-danger`;
  - the four amber-filled buttons → `bg-primary text-on-primary`;
  - avatar tints → `avatar-*`;
  - `CrisisAlert`'s whites → tokens.

  Verify with a grep for `-white`, `indigo`, `emerald`, `amber-` and hex colors in `frontend/src`: nothing is left outside `index.css`. Lint and build pass.
  - Done. Beyond the list:
    - `bg-ink text-white` (dark buttons and the resize label) became `text-bg`, and the "you" pill's white became `text-bg`, since both would be unreadable in Porch light.
    - The amber borders on the add-widget tile and the chat mode cards became haint, with the buttons they frame.
    - Fields and the two chat composers got the `control` edge.
    - Avatars use four brand tints (haint, pine, lamplight, cedar) instead of six Tailwind hues.
    - Amber stays on light only: spinners, the fresh-card edge, the thinking state, and the arrange grip.
  - The contrast list grew to 35 pairings. It caught one: muted ink on Porch light's amber tint was 4.31:1, so that derived tint went from 14% to 12% over surface (`#3d2d1c`).
- [x] 2.4 Replace Fontshare with `@fontsource-variable/fraunces` (full axes) and `@fontsource-variable/figtree`, imported in `main.tsx`. Point `--font-display` and `--font-sans` at them, and give headings the soft axis. Verify in the browser's network panel that no request goes to another host for fonts, and that headings render in soft Fraunces.
  - Done: the only font requests are the app's own Latin Figtree and Fraunces files (about 141 kB together). Headings compute to "Fraunces Variable" with `"SOFT" 100`, and body text to "Figtree Variable".
  - `npm audit` reports a high-severity advisory in `happy-dom`, through `@puckeditor/core`. It was there before this change and has no fix yet; the font packages add none.

## 3. The mark and the name

- [x] 3.1 Copy `favicon.svg`, `favicon.ico` and `apple-touch-icon.png` from `brand/logo/` into `frontend/public/`. In `index.html`, set the title to Kinlight, link the icons, and add a `theme-color` meta per color scheme. Rework `BrandMark` into the themed lantern and wordmark lockup (design decision 5), with hidden text "Kinlight" and the family name kept. Verify in the browser in both themes: the tab icon, the header lockup, and the screen reader name (read_page shows "Kinlight").
  - Done: the tab is titled Kinlight and links the ICO, SVG and touch icons. The SVG favicon follows the device's scheme on its own. The header lockup is 32 px tall (24 px left the wordmark too small beside the glow), with ink and light from the theme in both themes, and the accessible name is "Kinlight". The lockup is copied to `frontend/src/brand/lockup.svg` and inlined.
- [x] 3.2 Rename the user-visible strings and comments in the frontend (design decision 7, pass 2). Verify with lint and build.
- [x] 3.3 Rename the user-visible strings, AI context and comments in the backend (design decision 7, pass 2): handler errors, `internal/llm` prompts, `aikit` prompts and the tool description, the transcript label, the OpenRouter titles, `AppName`, and the calendar `User-Agent`. Update the tests and evaluation text that quote them. Verify with `make check`, and with the end check of design decision 7: `git grep -i porchlight` outside the history folders finds only the guard, its test and the README's "formerly" line.
  - Done. `make check` passes. `migrations/001_init.sql`'s comment changed too: migrations are tracked by file name, so editing a comment is safe.
  - End check (outside `devpost/`, archived changes, `brand/` research, naming and brief, and this change's own docs), what's left:
    - the guard's table, comments and message in `backend/main.go`, and its test;
    - `openspec/config.yaml`'s "Kinlight (formerly Porchlight)";
    - the roadmap's hackathon-era section (history) and its note on the rename;
    - `openspec/specs/demo-data` (this change's delta renames the fixture host at archive);
    - the README (task 6.1).

## 4. The theme choice

- [x] 4.1 Add `migrations/009_theme.sql`, the store method that sets the session user's theme, `theme` in `MeUser`, and `PUT /api/me/theme`. Route tests:
  - the default is `auto`;
  - setting `dark` is returned by `/api/me`;
  - an unknown value gets a 400 and changes nothing;
  - a cross-site PUT is rejected;
  - logged out gets a 401;
  - another member's theme is untouched.

  Verify with `make check`.
  - Done: `store.User.Theme` comes with every user load, so the session's user carries it. A store test covers the default, saving, another member, a refused value and a missing user. `TestTheme` covers the routes, and `/api/me/theme` is in the login-required list. `TestAcceptInvite` compares the whole `Me`, so it now expects `auto`.
- [x] 4.2 Add the frontend theme module (apply `data-theme`, theme-color and the `localStorage` cache on every `me.theme`, and clear the cache on logout), the pre-paint script in `index.html`, and the Theme switch in the sidebar footer (design decision 6). Verify in the browser:
  - choosing Porch light persists across a reload with no flash;
  - Automatic follows an emulated dark/light color scheme without a reload;
  - a failed save reverts and shows an error;
  - signing in as another member applies their theme.
  - Done (2026-10-25), on the demo with the device emulated light or dark:
    - Choosing Porch light saved `dark`, and a reload on a light device stays Porch light.
    - The pre-paint script is the only blocking script in `<head>`. Run on its own, it resolves all five cases (each stored choice, and none stored, against each device scheme) to the right theme.
    - On Automatic, the page followed the device from light to dark and back without a reload.
    - With saving made to fail, the switch went back to Porch light and said "Your theme wasn't saved. Try again in a moment."
    - Logging out forgets the choice: the sign-in page follows the device.
    - Signing in as Sam (Automatic) and then Maya (Porch light), without reloading, applied each one's theme.
  - Bug found and fixed: signing in didn't apply a member's saved theme until a reload. `startSession` removed the cached `me` query before setting the new one, so the root, which applies the theme, kept watching the removed query. `startSession` now overwrites `me` in place, as `endSession` already did.
  - The sidebar footer now shows at every width, with the switch. Its "Log out" is wide-screen only, since narrow screens have it in the top strip.

## 5. Kids by name

- [x] 5.1 Add the names rule to `parentIntro` and `kidIntro` (design decision 8), with prompt tests that the rule is present, and an `evalparent` case: the kids overview, failing on he, him, his, she or her outside quotes. Run `make ai-eval-parent` and record the result here. Verify with `go test ./internal/aikit/...`.
  - Result (2026-10-25), on Claude Haiku 5.5:
    - **First attempt:** a short rule placed near the top of the prompt didn't hold. The overview reply used "her" and "His" for the kids.
    - **The rule now:** it says what to write instead ("Lily's chat", "talk with Lily", even if it sounds a little repetitive). In the Assistant's prompt it sits last, beside the format rule. The kid prompt has it after the intro.
    - **Results:** the names case then passed in 4 of 5 runs; one reply had a single "she". All other cases passed every run. The kids' evaluation passed 17 of 17 with the rule in its prompt.
    - **Limits:** the rule is prompt-only, so the evaluation measures it and doesn't guarantee it, as the design says.
    - **Changes to the evaluation's fixtures:**
      - The flag reason and Lily's quoted message no longer use pronouns, so a reply can't pick one up from the data.
      - The older Lily case now accepts "lives" or "address".
      - The names case requires "Lily", not "Jordan": the overview reasonably leaves out a kid with no flags.
      - A failing case now prints the whole reply.

## 6. Docs and end to end

- [x] 6.1 Update the README: the Kinlight name and the lockup at the top, a "formerly Porchlight" line, the renamed variables and files, the theme switch, and self-hosted fonts. Verify by reading it.
- [x] 6.2 On the demo, in the browser, walk every page in Daybreak and in Porch light: sign-in, invite, create-family, home, arrange, chat, Assistant, Reviews, and member pages. Check the header lockup, the fonts, the theme switch, and that nothing is unreadable or still light in Porch light. Check the guard: starting with an old variable set stops with its message. Record the results here, with any page fixed.
  - Result (2026-10-25), on the demo:
    - **Pages, in both themes:** sign-in, invite, create-family (with a throwaway account, removed by rebuilding the demo), home, arrange mode, Lily's page as a parent, Reviews, a flagged transcript, the Assistant list and a conversation with a real reply, Jordan's page, the chat list, and a homework chat.
    - **What was checked:** every page shows the lockup, soft Fraunces headings, Figtree text, haint buttons and links, and a theme switch (on member pages). Nothing is unreadable or left light in Porch light. Fields and composers show their `control` edge, and native controls follow the theme.
    - **Minor, left as is:** Puck's own dark action bar in arrange mode sits close to Porch light's ground, though its text stays readable.
    - **The names rule:** one Assistant reply ("connected on her member page") slipped, as the evaluation predicted.
    - **The guard:** starting with `PORCHLIGHT_DEV_FIXTURES=1`, or with `DB_PATH=porchlight.db`, stops with exit 1 and names the replacement. No `porchlight.db` was created.
    - **Fixes during the walk:** none were needed beyond 4.2's sign-in fix. The screenshots showing light cards in Porch light were taken mid-transition; the computed colors were right.
- [ ] 6.3 Run `openspec validate kinlight-identity --strict`, `make check`, and the frontend lint, build and contrast check. Push, open the PR, and confirm CI passes on it. Add the "Kinlight identity" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
  - Progress (2026-10-24): the change and all 12 specs validate strictly, `make check` passes, and the frontend lint, contrast check (35 pairings) and build pass. The Post-PoC line is in both files. Pushing, the PR and CI are next.
