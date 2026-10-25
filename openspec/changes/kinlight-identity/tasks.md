For work in `internal/aikit`, load the `developing-genkit-go` skill first; the design wins where they differ. Run every Go command with `CGO_ENABLED=0 GOTOOLCHAIN=local` (or through `make`). Commit after each task.

## 1. Themes and fonts

- [ ] 1.1 Put the brief's Daybreak values in `@theme` and the Porch light overrides under `:root[data-theme="dark"]` and the `prefers-color-scheme` rule (design decisions 1 and 2), with `color-scheme` per theme. Add the new tokens: `on-primary`, `on-danger`, `avatar-1` to `avatar-3`, `glass-core`. Verify with `npm run build`, and by toggling `data-theme` in the browser devtools on the demo: every surface changes, and nothing stays light in Porch light.
- [ ] 1.2 Add `frontend/scripts/contrast.mjs` and `npm run check:contrast` (design decision 3), and run it in the CI frontend job. Adjust any brief value that fails, keeping its hue, and record each adjustment here. Verify that it passes, and that it fails when a pairing is made to fail on purpose.
- [ ] 1.3 Replace raw colors in components:
  - `text-white` → `text-on-primary` or `text-on-danger`;
  - the four amber-filled buttons → `bg-primary text-on-primary`;
  - avatar tints → `avatar-*`;
  - `CrisisAlert`'s whites → tokens.

  Verify with a grep for `-white`, `indigo`, `emerald`, `amber-` and hex colors in `frontend/src`: nothing is left outside `index.css`. Lint and build pass.
- [ ] 1.4 Replace Fontshare with `@fontsource-variable/fraunces` (full axes) and `@fontsource-variable/figtree`, imported in `main.tsx`. Point `--font-display` and `--font-sans` at them, and give headings the soft axis. Verify in the browser's network panel that no request goes to another host for fonts, and that headings render in soft Fraunces.

## 2. The mark and the name

- [ ] 2.1 Copy `favicon.svg`, `favicon.ico` and `apple-touch-icon.png` from `brand/logo/` into `frontend/public/`. In `index.html`, set the title to Kinlight, link the icons, and add a `theme-color` meta per color scheme. Rework `BrandMark` into the themed lantern and wordmark lockup (design decision 5), with hidden text "Kinlight" and the family name kept. Verify in the browser in both themes: the tab icon, the header lockup, and the screen reader name (read_page shows "Kinlight").
- [ ] 2.2 Rename the user-visible strings in the frontend (design decision 7). Verify with a grep: every remaining "Porchlight" in `frontend/src` is an identifier or a comment. Lint and build pass.
- [ ] 2.3 Rename the user-visible strings and AI context in the backend (design decision 7): handler errors, `internal/llm` prompts, `aikit` prompts and the tool description, the transcript label, the OpenRouter titles, `AppName`, and the calendar `User-Agent`. Update the tests and evaluation text that quote them. Verify with `make check`, and with a grep where every remaining "Porchlight" in `backend` is an identifier, a comment, an environment variable or a file name.

## 3. The theme choice

- [ ] 3.1 Add `migrations/009_theme.sql`, the store method that sets the session user's theme, `theme` in `MeUser`, and `PUT /api/me/theme`. Route tests:
  - the default is `auto`;
  - setting `dark` is returned by `/api/me`;
  - an unknown value gets a 400 and changes nothing;
  - a cross-site PUT is rejected;
  - logged out gets a 401;
  - another member's theme is untouched.

  Verify with `make check`.
- [ ] 3.2 Add the frontend theme module (apply `data-theme`, theme-color and the `localStorage` cache on every `me.theme`, and clear the cache on logout), the pre-paint script in `index.html`, and the Theme switch in the sidebar footer (design decision 6). Verify in the browser:
  - choosing Porch light persists across a reload with no flash;
  - Automatic follows an emulated dark/light color scheme without a reload;
  - a failed save reverts and shows an error;
  - signing in as another member applies their theme.

## 4. Kids by name

- [ ] 4.1 Add the names rule to `parentIntro` and `kidIntro` (design decision 8), with prompt tests that the rule is present, and an `evalparent` case: the kids overview, failing on he, him, his, she or her outside quotes. Run `make ai-eval-parent` and record the result here. Verify with `go test ./internal/aikit/...`.

## 5. Docs and end to end

- [ ] 5.1 Update the README: the Kinlight name and the lockup at the top, a note that code, environment variables and the database keep "porchlight", the theme switch, and self-hosted fonts. Verify by reading it.
- [ ] 5.2 On the demo, in the browser, walk every page in Daybreak and in Porch light: sign-in, invite, create-family, home, arrange, chat, Assistant, Reviews, and member pages. Check the header lockup, the fonts, the theme switch, and that nothing is unreadable or still light in Porch light. Check that a session from before the update stays signed in. Record the results here, with any page fixed.
- [ ] 5.3 Run `openspec validate kinlight-identity --strict`, `make check`, and the frontend lint, build and contrast check. Push, open the PR, and confirm CI passes on it. Add the "Kinlight identity" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
