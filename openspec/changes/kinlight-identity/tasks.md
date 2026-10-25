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
- [ ] 1.3 Rename the frontend: the package name, `PorchlightConfig`, `PorchlightData`, `usePorchlightPuck` and its file, and `vite.config.ts`'s `PORCHLIGHT_WEB_PORT` and `PORCHLIGHT_API`. Verify with lint and build.
- [ ] 1.4 Rename the name in repo files: CI comments, the OpenSpec `config.yaml` context (the name only), the main specs' Purpose lines, and `frontend/README.md`. Verify with `openspec validate --specs --strict`.
- [ ] 1.5 On the owner's machine:
  - `mv backend/porchlight.db backend/kinlight.db`;
  - rebuild the demo with `make demo`;
  - delete `porchlight.db.demo`.

  Ask the owner to rename the `PORCHLIGHT_*` variables and `DB_PATH` in their `backend/.env`, without reading it. Verify that the server starts on the renamed files. If it stops instead, the guard's message names what's left to rename.

## 2. Themes and fonts

- [ ] 2.1 Put the brief's Daybreak values in `@theme` and the Porch light overrides under `:root[data-theme="dark"]` and the `prefers-color-scheme` rule (design decisions 1 and 2), with `color-scheme` per theme. Add the new tokens: `on-primary`, `on-danger`, `avatar-1` to `avatar-3`, `glass-core`. Verify with `npm run build`, and by toggling `data-theme` in the browser devtools on the demo: every surface changes, and nothing stays light in Porch light.
- [ ] 2.2 Add `frontend/scripts/contrast.mjs` and `npm run check:contrast` (design decision 3), and run it in the CI frontend job. Adjust any brief value that fails, keeping its hue, and record each adjustment here. Verify that it passes, and that it fails when a pairing is made to fail on purpose.
- [ ] 2.3 Replace raw colors in components:
  - `text-white` → `text-on-primary` or `text-on-danger`;
  - the four amber-filled buttons → `bg-primary text-on-primary`;
  - avatar tints → `avatar-*`;
  - `CrisisAlert`'s whites → tokens.

  Verify with a grep for `-white`, `indigo`, `emerald`, `amber-` and hex colors in `frontend/src`: nothing is left outside `index.css`. Lint and build pass.
- [ ] 2.4 Replace Fontshare with `@fontsource-variable/fraunces` (full axes) and `@fontsource-variable/figtree`, imported in `main.tsx`. Point `--font-display` and `--font-sans` at them, and give headings the soft axis. Verify in the browser's network panel that no request goes to another host for fonts, and that headings render in soft Fraunces.

## 3. The mark and the name

- [ ] 3.1 Copy `favicon.svg`, `favicon.ico` and `apple-touch-icon.png` from `brand/logo/` into `frontend/public/`. In `index.html`, set the title to Kinlight, link the icons, and add a `theme-color` meta per color scheme. Rework `BrandMark` into the themed lantern and wordmark lockup (design decision 5), with hidden text "Kinlight" and the family name kept. Verify in the browser in both themes: the tab icon, the header lockup, and the screen reader name (read_page shows "Kinlight").
- [ ] 3.2 Rename the user-visible strings and comments in the frontend (design decision 7, pass 2). Verify with lint and build.
- [ ] 3.3 Rename the user-visible strings, AI context and comments in the backend (design decision 7, pass 2): handler errors, `internal/llm` prompts, `aikit` prompts and the tool description, the transcript label, the OpenRouter titles, `AppName`, and the calendar `User-Agent`. Update the tests and evaluation text that quote them. Verify with `make check`, and with the end check of design decision 7: `git grep -i porchlight` outside the history folders finds only the guard, its test and the README's "formerly" line.

## 4. The theme choice

- [ ] 4.1 Add `migrations/009_theme.sql`, the store method that sets the session user's theme, `theme` in `MeUser`, and `PUT /api/me/theme`. Route tests:
  - the default is `auto`;
  - setting `dark` is returned by `/api/me`;
  - an unknown value gets a 400 and changes nothing;
  - a cross-site PUT is rejected;
  - logged out gets a 401;
  - another member's theme is untouched.

  Verify with `make check`.
- [ ] 4.2 Add the frontend theme module (apply `data-theme`, theme-color and the `localStorage` cache on every `me.theme`, and clear the cache on logout), the pre-paint script in `index.html`, and the Theme switch in the sidebar footer (design decision 6). Verify in the browser:
  - choosing Porch light persists across a reload with no flash;
  - Automatic follows an emulated dark/light color scheme without a reload;
  - a failed save reverts and shows an error;
  - signing in as another member applies their theme.

## 5. Kids by name

- [ ] 5.1 Add the names rule to `parentIntro` and `kidIntro` (design decision 8), with prompt tests that the rule is present, and an `evalparent` case: the kids overview, failing on he, him, his, she or her outside quotes. Run `make ai-eval-parent` and record the result here. Verify with `go test ./internal/aikit/...`.

## 6. Docs and end to end

- [ ] 6.1 Update the README: the Kinlight name and the lockup at the top, a "formerly Porchlight" line, the renamed variables and files, the theme switch, and self-hosted fonts. Verify by reading it.
- [ ] 6.2 On the demo, in the browser, walk every page in Daybreak and in Porch light: sign-in, invite, create-family, home, arrange, chat, Assistant, Reviews, and member pages. Check the header lockup, the fonts, the theme switch, and that nothing is unreadable or still light in Porch light. Check the guard: starting with an old variable set stops with its message. Record the results here, with any page fixed.
- [ ] 6.3 Run `openspec validate kinlight-identity --strict`, `make check`, and the frontend lint, build and contrast check. Push, open the PR, and confirm CI passes on it. Add the "Kinlight identity" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
