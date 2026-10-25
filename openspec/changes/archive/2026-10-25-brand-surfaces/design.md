## Context

See proposal.md for why. What exists in `frontend/src`:

- **Tokens.** Colors and motion are tokens in `styles/index.css`, with Porch light as overrides under `:root[data-theme="dark"]`. `scripts/contrast.mjs` checks 35 pairings in both themes.
- **Welcome pages.** Sign-in, sign-up, invite, create-family and not-found all use `WelcomeLayout`: the lockup, a title, then a plain surface card on the ground.
- **Floating surfaces:**
  - the arrange toolbar (`ArrangeCanvas`, sticky, `bg-surface/95 backdrop-blur`);
  - Puck's card action bar, styled through `--puck-actionbar-*` variables, and dark by default;
  - the chat and Assistant composers (`sticky bottom-0 bg-bg`).

  There are no modal sheets or menus.
- **Empty states:** a new kid chat or Assistant conversation (a line of text), Reviews with nothing to review, the Assistant with no conversations yet, and "This spot is empty" tiles on a page.
- **"widget" in copy.** About 12 frontend strings (the add tile, its status and title, the sidebar's note, the arrange action bar's remove and confirm labels, a gone card, the invite intro, the member-page intro, empty spots). About 8 backend messages and hints (`widgets/errors.go`, `widgets/handler.go`, `layout/data.go`, `llm/client.go` and `aikit/structured.go`'s refusal hint), and the tests that quote them. The AI's own instructions in `internal/llm` say "widget" only to the model.
- **The kid-safe marker.** `WidgetCard`'s `KidSafeTag` reads "Kid-safe" for a parent.
- **The app icon.** `brand/logo/build.py` already draws the tile's lantern with a brass rim (`#D9B77A`).

## Goals / Non-Goals

**Goals:**
- Each material is one token set and one utility, usable anywhere and themed.
- Textures can never lower text contrast, and a check proves it.
- The illustration is a file swap, not a code change.

**Non-Goals:**
- Drawing the real illustration.
- Animating materials.
- Restyling cards or forms beyond the brass on two elements.
- Renaming code, APIs or data.
- Changing the AI's internal prompts.

## Decisions

### 1. Material tokens and utilities

In `@theme`, with Porch light overrides:
- **Brass:** `--color-brass-light` `#E7C98E` and `--color-brass-deep` `#B08D57`, the same in both themes, since brass is a metal, not a theme color.
- **Wood:** `--color-wood` (pine `#DDBE95` in Daybreak, cedar `#B07A52` in Porch light) and `--color-wood-deep` (cedar `#9A6440`, and a darker cedar in Porch light).

Utilities in `@layer utilities`:
- **`brass-edge`.** A 1 px transparent border with two backgrounds: the element's own fill on `padding-box` and the brass gradient on `border-box`. That keeps the rounded corners, which `border-image` can't do.
- **`lantern-glass`.**
  - Background: `color-mix()` of surface and 4% accent at 88% opacity, with `backdrop-filter: blur(14px) saturate(1.1)`.
  - Grain: a pseudo-element with the grain texture at about 3%.
  - Fallback: without `backdrop-filter`, the same color at full opacity.
- **`wood-grain`.** An `::before` layer behind the element's children:
  - porch boards: a `repeating-linear-gradient` of horizontal bands 56 px tall, with a 1 px seam of `wood-deep` at about 14%;
  - grain: an `feTurbulence` SVG, stretched horizontally, at about 5%.

The grain is one inline SVG data URI (`--grain`). No image request is made.

### 2. Contrast under textures, worked out

`contrast.mjs` gains blended backgrounds. It computes the colors text actually sits on in the worst case, and checks text against them in both themes:
- **Glass:** surface/accent glass at 88%, over the darkest thing that can scroll under it (ink in Daybreak, and the lightest in Porch light).
- **Wood:** the ground with the seam color at full seam strength, the darkest pixel.

The pairings: ink, muted ink and primary on glass; ink and muted ink on wood. If one fails, the material's strength is lowered, not the text's color, and the change is recorded in tasks.

### 3. Where each material goes

- **Brass:** the briefing card (`WidgetCard` gets a `brass` prop, set only by `BriefingWidget`) and the add-a-card button (the "+" circle).
- **Glass:**
  - the arrange toolbar;
  - Puck's action bar, by setting its background variables to the glass color and adding the class to Puck's action-bar root through the existing `.arrange-canvas` overrides;
  - both composers' sticky form wrappers.
- **Wood:**
  - `WelcomeLayout`'s page;
  - the sidebar `aside`, as grain only, without boards, since it holds content;
  - each empty state's frame.

  Cards, chats and forms keep plain surfaces.

### 4. The illustration: a themed SVG slot

`frontend/src/brand/porch.svg` is the stand-in. `PorchScene` inlines it with `?raw`, as the wordmark is inlined, and sets its color variables from the theme:

| Variable | Token | Meaning |
| --- | --- | --- |
| `--pi` | ink | ink |
| `--pl` | accent | light |
| `--pg` | glass-core | glass core |
| `--pw` | wood | wood |
| `--pd` | wood-deep | seams and shade |
| `--ph` | primary-soft | the haint ceiling |

The scene is `aria-hidden`, and never carries words. Its `viewBox` is `0 0 1200 600`. It has two slots:
- **`wide`:** on the welcome pages, full width, about 160 px tall, `preserveAspectRatio="xMidYMax slice"` (anchored to the porch floor);
- **`small`:** in empty states, about 120 px wide, `xMidYMid meet`.

The stand-in is deliberately plain: three floor boards with seams, two posts, the haint ceiling band, and the lantern from `LanternMark`'s drawing, hung from the ceiling.

`brand/illustration.md` documents the drop-in format for an illustrator:
- the canvas and the safe area for both crops;
- the six variables and the rule that every fill uses one of them;
- no text, no faces, no embedded raster images;
- where to save the file.

### 5. Voice: copy changes plus a check

**Copy changes:**
- **Frontend:** "widget" becomes "card" in the strings listed under Context.
- **Empty states:**
  - a new page: "Nothing here yet. Tell Kinlight what you'd like to see.";
  - an empty spot: "An empty spot. Add a card to fill it.".
- **Errors:** `LoadError` gives one short sentence per card ("The weather didn't load. Try again in a minute."), using the card's noun. The API's message stays as the hint.
- **The kid-safe marker:** "Picked with <name>'s age in mind", with the name from the page owner (`nameFor`), and the existing tooltip kept.
- **Backend:** the 8 messages and hints say "card", and tests are updated.

**`scripts/copy-check.mjs`** (`npm run check:copy`, run in CI's frontend job) scans:
- **The frontend:** string literals and JSX text in `src/**/*.{ts,tsx}`.
- **The backend:** the Go string literals on lines that call `respond.Error` or set a `Hint:`.

It fails when:
- text contains a space and the word "widget(s)", which is copy rather than a key like `'widgets'`;
- any text contains a banned phrase.

Comments are skipped. The AI prompts in `internal/llm` aren't scanned, because the model's instructions aren't copy.

## Risks / Trade-offs

- **[`backdrop-filter` is costly on low-end devices.]** → It's used only on three small sticky surfaces, with a solid fallback.
- **[Grain or boards look busy, or band against content.]** → The strengths are low (3 to 5%), and only on brand surfaces. Compare in both themes in the browser and adjust.
- **[The copy check's heuristics miss text or flag a key.]** → It errs toward flagging. A key that legitimately contains a space and "widget" can be allow-listed with a comment. The check's own tests cover a JSX case, a string-literal case, and a query key that passes.
- **[A stand-in looks unfinished.]** → It's intentionally plain, and the drop-in doc makes the commission easy. The owner chose this.
- **[Renaming to "card" in errors may confuse old logs.]** → Logs and code keep "widget". Only people-facing text changes.

## Migration Plan

Frontend and copy only; no data change. Rollback is a revert.
