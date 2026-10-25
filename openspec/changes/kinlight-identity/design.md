## Context

See proposal.md for why. What exists today:

- **Tokens.** `frontend/src/styles/index.css` defines every color as a Tailwind v4 `@theme` token under role names (`bg`, `surface`, `line`, `ink`, `ink-soft`, `ink-muted`, `primary`, `primary-strong`, `primary-soft`, `accent`, `accent-strong`, `accent-soft`, `danger`, `danger-soft`). Components use only these, with three exceptions:
  - `text-white` on filled buttons (about a dozen places);
  - the avatar tints in `family/people.ts` (`bg-indigo-100`, `bg-emerald-100`, `bg-amber-100`);
  - `bg-white` in `CrisisAlert`.
- **Amber as a button fill.** `bg-accent` fills four buttons: kid chat's send, the chat card's "Ask Porchlight", the add-widget circle, and describe-a-widget's submit.
- **Fonts.** They load at runtime from `api.fontshare.com`, through two `<link>`s in `index.html`.
- **The mark.** `components/BrandMark.tsx` draws an amber dot and the word "Porchlight". `public/favicon.svg` is the old dot.
- **Where `/api/me` comes from.** `family.Handler` (`Me`, `MeUser`, `MeProfile`) serves it. The frontend caches it forever (`staleTime: 'static'`) and writes new answers into the cache.
- **The logo files.** `brand/logo/` has the finished files from `build.py`. `themed/*.svg` take their colors from `--mi` (ink), `--ml` (light) and `--mg` (glass core). The kinlight wordmark is outlines of Fraunces, so it doesn't need the font.
- **Serving.** No CSP header is set, and the Go server doesn't serve the frontend yet; `homelab-deployment` will.
- **The old name in code.** It appears in about 500 places outside the history folders:
  - the Go module path `porchlight/internal/...`, in every import;
  - six `PORCHLIGHT_*` variables (`KIDS_ALLOW_RETENTION`, `DEV_FIXTURES`, `LIVE_AI`, `BRIEFER`, `WEB_PORT`, `API`);
  - `auth.CookieName = "porchlight_session"`;
  - `DB_PATH`'s default `porchlight.db`, and `porchlight.db.demo` and `.test`;
  - the fixtures host `fixtures.porchlight.test`;
  - the frontend package `porchlight-frontend`;
  - `PorchlightConfig`, `PorchlightData` and `usePorchlightPuck`;
  - `bin/porchlight`.

  `loadConfig` calls `godotenv.Load(".env")` before reading the environment, so anything set in `backend/.env` is visible to a check made after it.

## Goals / Non-Goals

**Goals:**
- One token file, where Porch light is overrides of the same names.
- Contrast is checked by a machine, not by eye.
- No flash of the wrong theme.
- After the change, "porchlight" survives only in history: `devpost/`, archived changes, and `brand/`'s research and naming docs.

**Non-Goals:**
- Any animation beyond what exists. The current `widget-in` and `spin-calm` stay until `lantern-moments`.
- Textures, glass and illustration.
- A web manifest or PWA install.
- Theme choice for other members: parents don't set kids' themes.

## Decisions

### 1. Keep the token role names; change their values

The brief's roles map onto the existing names:

| Existing name | New role |
| --- | --- |
| `bg` | ground |
| `primary` | haint |
| `primary-soft` | haint tint |
| `accent` | lamplight |
| `accent-strong` | text-safe light |

New tokens fill the gaps:
- `on-primary` and `on-danger` replace `text-white` on filled buttons. Porch light's haint (`#9FD0C9`) and terracotta (`#E08A72`) are light colors, so their labels must be dark there.
- `ink-soft` gets a value in each theme, between ink and muted.
- `avatar-1` to `avatar-3` replace the hard-coded avatar tints.
- `glass-core` is the lantern's core color.
- `control` is the edge of a field or composer. The brief's `line` is about 1.3:1 against surface, too faint for WCAG's 3:1 rule for control boundaries, so `line` stays for decorative separators. Added during apply.

*Alternative:* rename to `haint`, `lamplight` and so on. That's clearer to read, but touches nearly every component for no change in behavior, and the role names already say what each is for. Rejected.

The four amber-filled buttons become `bg-primary text-on-primary`. Amber stays for the light: the tool-note chips, the arrange-mode overlay, the resize grip, and the lantern itself.

### 2. Porch light is one block of overrides, and the page always says which theme

Tailwind v4's `@theme` writes each token as a CSS variable on `:root`, and every utility reads `var(--color-*)`. So a dark theme is just the variables redefined:

```css
@theme { --color-bg: #f7f2ea; … }                 /* Daybreak */
@layer base {
  :root[data-theme="dark"] { --color-bg: #18120e; … color-scheme: dark; }
}
```

`<html>` always carries `data-theme="light"` or `"dark"`. The pre-paint script (decision 6) sets it before first paint, resolving Automatic with `matchMedia('(prefers-color-scheme: dark)')`, and the theme module keeps it current as the device or the member's choice changes. So Porch light's values are written once.

Tokens must not use `@theme inline`, which would bake values into the utilities and defeat the overrides. Puck's variables are already set from tokens, so the arrange editor follows the theme for free.

*Revised during apply:* the first plan let a `prefers-color-scheme` media query handle Automatic in CSS. That needed the same overrides written twice, once under the attribute and once under the media query. Resolving Automatic in the script that must run anyway removes the copy.

*Alternative:* Tailwind's `dark:` variant on each component. That doubles every color class and spreads the dark theme through a hundred files. Rejected, as the brief asks.

*Alternative:* CSS `light-dark()` in each token. That's one value pair per token, but Tailwind's opacity modifiers wrap tokens in `color-mix()`, and nesting `light-dark()` there is newer and less proven than plain variables. Rejected.

### 3. Contrast is a checked list of pairings

`frontend/scripts/contrast.mjs` (plain Node, no dependencies):
1. reads both themes' values from `index.css`;
2. checks a list of foreground-on-background pairings with the WCAG 2 formula, at 4.5:1 for text and 3:1 for focus rings and control borders;
3. exits non-zero on any failure.

Pairings checked include ink and muted ink on ground and surface, `on-primary` on primary, primary as link text on surface, `accent-strong` on surface, and danger on `danger-soft`. CI runs it with `npm run check:contrast` in the frontend job. If a brief value fails, the script says so and the value is adjusted, with the change noted in tasks.

### 4. Fonts from npm, bundled by Vite

The fonts come from `@fontsource-variable/fraunces` (its full-axes CSS, which includes `SOFT` and `opsz`) and `@fontsource-variable/figtree`, imported in `main.tsx`. Vite copies the woff2 files into the build, so they're served from the app's own origin. `--font-display` and `--font-sans` name them with the existing fallbacks. `font-display: swap` comes with the packages.

Headings set `font-variation-settings: "SOFT" 100`, and keep `font-synthesis: none`, since the variable weight axis covers bold.

*Alternative:* download the files into `public/fonts` by hand. That means no version pinning and manual updates. Rejected.

### 5. The mark as React components from the themed SVGs

`brand/logo/themed/lantern.svg` and `lockup.svg` become small React components, `LanternMark` and the reworked `BrandMark`.
- They map the SVG's variables to tokens: `--mi` is ink, `--ml` is accent, and `--mg` is `glass-core`.
- They're `aria-hidden`, with "Kinlight" in visually hidden text.
- `BrandMark` keeps its `familyName` prop and layout.

Static icons are copied from `brand/logo/` into `frontend/public/`: `favicon.svg`, `favicon.ico` and `apple-touch-icon.png`. `index.html` gets `<title>Kinlight</title>` and two `theme-color` metas, one per `prefers-color-scheme`. `brand/logo/` stays the source; a task note says to re-copy after `build.py` runs.

### 6. The theme choice: a column, two endpoints, a pre-paint cache

- **Storage.** `migrations/009_theme.sql` adds the column:

  ```sql
  ALTER TABLE users ADD COLUMN theme TEXT NOT NULL DEFAULT 'auto' CHECK (theme IN ('auto','light','dark'));
  ```

  It goes on `users` rather than `member_profiles`, since parents edit profiles but only the member sets a theme. The stored values are code-neutral; the UI names them Automatic, Daybreak and Porch light.
- **API.**
  - `MeUser` gains `theme`.
  - `PUT /api/me/theme` takes `{"theme": "dark"}`. It answers 200 with the updated `Me`, and 400 for an unknown value. It's under the existing `private` middleware, with the same CSRF rules as other `/api/me` writes.
  - The store method takes the session's user id only, so there's no path to another member.
- **Frontend.**
  - A theme module applies a choice by setting `data-theme` on `<html>` (resolving Automatic, and listening for device changes while it's chosen), setting the theme-color meta, and writing the choice to `localStorage` (`kinlight.theme`).
  - It runs whenever `me.theme` changes. Logging out clears the stored choice.
- **No flash.** A short inline script in `index.html`, before the stylesheet, reads `kinlight.theme` (default `auto`), resolves Automatic from `matchMedia`, and sets `data-theme` before first paint. On a new device, the first paint follows the device until `/api/me` answers, and is correct from then on.
- **The switch.** A three-way segmented control (Automatic, Daybreak, Porch light) in the sidebar's footer, by Log out, on narrow and wide layouts. It's a radio group with a visible label, "Theme". It updates optimistically, and reverts with an error message if the save fails.

*Alternative:* `localStorage` only. That's simpler, but the choice wouldn't follow the member to other devices, which was the decision. Rejected.

### 7. A full rename, in two passes

**Pass 1, code and settings (mechanical, no visible change).** One commit per kind, so each diff is easy to review:
1. **The Go module.** `go mod edit -module kinlight`, then rewrite the `porchlight/` imports. Verified by `gofmt -l` and `make check`.
2. **Settings and files.** Rename these old to new:
   - the six `PORCHLIGHT_*` variables to `KINLIGHT_*`;
   - `porchlight_session` to `kinlight_session`;
   - `porchlight.db*` to `kinlight.db*` (the default, the Makefile, `.gitignore`, `.env.example`, tests);
   - `fixtures.porchlight.test` to `fixtures.kinlight.test` (fixtures, seed, tests).
3. **The frontend.** The package name, `PorchlightConfig`, `PorchlightData`, `usePorchlightPuck` and its file, and `vite.config.ts`'s environment variables.
4. **Repo files.** CI comments, the OpenSpec `config.yaml` context (the name only), the main specs' Purpose lines, and the roadmap's series-2 decision.

**The startup guard.** `loadConfig` checks, after `.env` loads:
- a table of the six old variable names, each with its replacement;
- whether `DB_PATH`'s file name starts with `porchlight.db`.

On a match it returns an error naming the old setting and what to use instead, and the server exits before opening anything. Its unit test sets each old variable through `t.Setenv`.

The guard reads only names, never values, so it never reveals a secret. It stays until the homelab deployment is done, then can go.

*Alternative:* accept both names for a while. The app isn't deployed, and a second name is exactly what a clean break avoids. Rejected.

**Pass 2, display strings.**
- **Frontend:** every user-visible "Porchlight" (`AuthForm`, the invite page, `ChatCard`, `MessageList`'s agent label, `CrisisAlert`, `BirthYearField`, `ArticlesWidget`, the API client's error messages).
- **Backend:**
  - the three handler errors;
  - the LLM prompts in `internal/llm` (curate, events, spec, items, briefing);
  - `aikit`'s kid and parent prompts and the `kids_chats` tool description;
  - the `kids_chats` transcript's agent label;
  - `X-Title` / `X-OpenRouter-Title`, `aikit` `AppName`, and the calendar `User-Agent`;
  - comments that name the app.

**The end check.** `git grep -i porchlight` outside `devpost/`, `openspec/changes/archive/`, `brand/research.md`, `brand/naming.md` and `brand/brief.md` finds only:
- the startup guard's table;
- its test;
- the README's "formerly Porchlight" line.

### 8. Kids by name: a prompt rule plus an evaluation case

- **Prompt rule.** `parentIntro` gains: refer to family members by name, and don't guess anyone's pronouns from a name; use pronouns only as the parent did.
- **Evaluation case.** `cmd/evalparent` gains a case: the kids overview. The rule fails the reply if, outside quoted text, it uses he, him, his, she or her.
- **The kid agent.** It speaks to the kid ("you"), and the my-day tool lists other members by name, so the same sentence goes into `kidIntro` at no cost.

## Risks / Trade-offs

- **[A reference to the old name is missed.]** → The end grep, `make check`, the frontend build, and CI. The `-race` job runs with `CGO_ENABLED=1` on Linux, so it catches anything platform-specific.
- **[Someone's local `.env` still has old names.]** → The startup guard stops the server with the exact rename to make. Claude never reads `backend/.env`; the owner edits it.
- **[The brief's colors fail a pairing the brief didn't list.]** → The contrast script decides. Adjust the value, keeping hue, and record it in tasks.
- **[Fraunces' soft axis is missing from the npm package's default CSS.]** → Import the package's full-axes stylesheet. If `SOFT` isn't there, fall back to the default soft-less cut and note it. The wordmark is outlines, so the header is unaffected.
- **[A screen with a hard-coded light color looks wrong in Porch light.]** → The grep for raw colors runs in tasks. The end-to-end check walks every page in both themes: home, arrange, chat, Assistant, Reviews, member pages, sign-in, invite, and create-family.
- **[Font files make the first load heavier.]** Two variable woff2 files come to roughly 200 KB, cached after the first visit. Acceptable for a family app on a LAN.
- **[The inline script would break under a future strict CSP.]** → `homelab-deployment` adds its hash to the CSP if it adds one. Noted there.
- **[The kids' names rule is prompt-only, and models slip.]** → It's an evaluation case, not a guarantee. The spec says "SHALL", and the evaluation measures it.

## Migration Plan

1. **The owner's machine:**
   - rename the `PORCHLIGHT_*` variables in `backend/.env` to `KINLIGHT_*`, and `DB_PATH` if it's set;
   - `mv backend/porchlight.db backend/kinlight.db`;
   - `make demo` rebuilds `kinlight.db.demo`, and the old `porchlight.db.demo` can be deleted.
2. **Database.** Migration 009 runs at startup, and every existing user gets `theme = 'auto'`.
3. **Sessions.** The cookie has a new name, so everyone signs in again once.
4. **Rollback.** Revert the commits and rename the file and variables back. The extra column is ignored by the old code.
