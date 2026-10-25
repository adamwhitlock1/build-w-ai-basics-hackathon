Verify visual work in the browser on the demo (`make demo`, then `KINLIGHT_KIDS_ALLOW_RETENTION=1 make demo-serve` and `npm --prefix frontend run dev`) in both themes. Run Go with `CGO_ENABLED=0 GOTOOLCHAIN=local` (or `make`). Commit after each task.

## 1. Materials

- [x] 1.1 Add the brass and wood tokens for both themes, the `--grain` texture, and the `brass-edge`, `lantern-glass` and `wood-grain` utilities (design decision 1). Verify that `npm run build` passes and the CSS output has the three utilities.
  - Done. Tailwind emits an `@utility` only when a class uses it, so I checked with one build forcing the classes in through `@source inline`: `brass-edge`, `lantern-glass`, `wood-grain` and a fourth, `porch-boards`, all compiled, with a solid fallback and `-webkit-backdrop-filter`. Porch boards are their own utility, so the sidebar can have grain without boards. The textures are SVG noise used as a mask over a theme color, so one file works on both grounds.
- [x] 1.2 Extend `contrast.mjs` with blended backgrounds for glass and wood, and their pairings (design decision 2). Adjust a material's strength if any pairing fails, and record it here. Verify that `npm run check:contrast` passes, and fails when a material's strength is raised on purpose.
  - Done. The check reads each material's strengths from its `@utility` block and composites the worst case: glass over ink with its grain, and wood's seam plus grain over the ground. It adds 8 pairings, for 43 in all.
  - The design's first strengths failed: muted ink on glass was 4.08:1 (Daybreak) and 3.60:1 (Porch light), and on wood 4.06:1, against 4.5. Muted ink is close to the line even on the plain ground (5.08:1), so the materials were lowered, not the text:
    - glass from 88% to **95%** opaque, and its grain from 0.03 to **0.02**;
    - board seams from 14% to **7%**, and wood grain from 0.05 to **0.03**.

    These are the strongest values that pass.
  - Raising the seam to 20% on purpose made the check fail (exit 1).
- [x] 1.3 Apply brass to the briefing card and the add-a-card button, glass to the arrange toolbar, Puck's action bar and both composers, and wood grain to the sidebar (design decision 3). Verify in the browser in both themes:
  - brass on exactly those two;
  - glass stays readable with cards scrolled under it, including Puck's bar in Porch light;
  - the sidebar grain is faint, and no card, chat or form has texture behind its text.
  - Done (2026-10-25), on the demo as Maya:
    - **Brass:** exactly two elements, the briefing card and the add button's "+".
    - **The arrange toolbar's glass** stayed readable with the briefing scrolled under it in Porch light.
    - **Puck's card action bar** is glass with ink text and a hairline edge in both themes, which fixes the earlier note that its dark bar sat too close to Porch light's ground. Its active width moved from amber to the text-safe `accent-strong`, which is readable on light glass. That pairing was added to the contrast check, now 44 pairings.
    - **The Assistant's composer** sticks as glass over a long reply.
    - **The sidebar grain** is faint, and nothing else has texture yet.

## 2. The illustration slot

- [x] 2.1 Add the stand-in `porch.svg` and `PorchScene` with its `wide` and `small` slots (design decision 4). Put `wide` and the porch-board wood on `WelcomeLayout`, and `small` in the empty states: a new kid chat, a new Assistant conversation, Reviews with nothing to review, and the Assistant list with none. Write `brand/illustration.md`. Verify in the browser in both themes on each of those pages: no faces, hidden from screen readers. Swap in a test SVG made to the format and confirm it shows in both slots with no code change.
  - Done (2026-10-25):
    - **The wide slot** on the welcome layout (checked on the not-found page) and **the small slot** in a new "About the kids" conversation show the stand-in in both themes. Both are hidden from screen readers and have no faces.
    - **Welcome pages:** porch boards and grain on the ground, with the card plain.
    - **The swap:** a test SVG made to the format showed in both slots, filled from the theme (`--pw` resolved to pine), with no code change. The stand-in is restored.
    - **Changed from the design:** the wide slot keeps the drawing's own 2:1 shape instead of a fixed 144 to 176 px height. The fixed height cropped off the haint-blue ceiling, which is part of the brand. `brand/illustration.md` says so.
    - **Also:** the Assistant list had no empty state at all; it now says "No conversations yet. Pick one of the two above to start." The four empty states share an `EmptyState` component.

## 3. Voice

- [x] 3.1 Add `scripts/copy-check.mjs` and `npm run check:copy` (design decision 5), with small tests in `tests/copy-check.test.ts` (a JSX case, a string-literal case, a Go `respond.Error` case, and a query key that passes), and run it in CI. Run it now and list what it finds here: that's the to-do list for 3.2 and 3.3.
  - Done. There are 7 tests, including two false positives the first run found: an interpolation like `${widget.title}`, and code between two tags taken for JSX text. Both are now ignored and tested. It runs in CI's frontend job.
  - **Found, frontend (17):**
    - `FamilySidebar` 177;
    - `MemberPage` 100;
    - `BlockActionBar` 122, 134, 135, 140;
    - `LayoutView` 65;
    - `Custom` 39;
    - `puck/config` 80;
    - `routes/_authed/index` 254, 286, 341 (×2);
    - `invite.$token` 103;
    - `DescribeWidgetPrompt` 78, 109, 167.
  - **Found, backend (8):**
    - `aikit/structured.go` 85 and `llm/client.go` 220 ("Try describing the widget differently.");
    - `widgets/errors.go` 94, 96, 98, 156, 215;
    - `widgets/handler.go` 338.
- [x] 3.2 Rename "widget" to "card" in every frontend string it found, write the empty-state and error copy, and change the kid-safe marker to "Picked with <name>'s age in mind" (design decision 5). Verify:
  - `npm run check:copy` passes;
  - in the browser: "Add a card", "Building your card…", an empty spot, a card's load error, and the marker on Jordan's page as Maya (and no marker as Jordan).
  - Done (2026-10-25). `npm run check:copy` finds nothing in the frontend, and lint, build and `npm test` (11) pass. In the browser as Maya:
    - "Add a card", and "Building your card…" (held by a patched `fetch`);
    - a failed Articles refresh: "Articles didn't refresh. Try again in a minute.", with the server's reason below and the old articles kept;
    - on Jordan's page, "Picked with Jordan's age in mind" on each kid-safe card.

    Jordan's own view is checked in 4.2.
  - **The empty spot** reads "An empty spot. Add a card to fill it."
  - **The design's "new page" empty state has no home:** a page always has its built-in cards, so the empty spot is the page's empty state.
  - **Load errors** use each card's noun: the card's title for a described card, and "today's events" for Today.
  - **The marker:** `useKidSafeFor` replaces the `showKidSafe` flag with the kid's name ("Jordan's", or "their" until the family list loads).
- [x] 3.3 Rename "widget" to "card" in the backend messages and hints it found, and update the tests that quote them. Verify `make check` and `npm run check:copy`.
  - Done: the 6 distinct messages (8 places) say "card", and the tests that quote them are updated, across 10 files. `make check` passes. `npm run check:copy` now finds nothing anywhere. No frontend code matches those messages; only the copy check's own test fixtures still say "widget", on purpose.

## 4. Docs and end to end

- [x] 4.1 Update the README's "Look and themes" with the materials, the illustration slot (pointing to `brand/illustration.md`), the voice rules and `check:copy`. Verify by reading it.
  - Done. The README's own product prose now says "card" too: the intro, arranging, freshness, the data file, the demo table and usage. Code names (`WidgetBuilder`, `internal/llm`'s description) and the historical status line keep "widget".
- [x] 4.2 On the demo, as Maya and as Jordan, in both themes: walk the welcome pages, home, arrange, Lily's page, chat, Assistant and Reviews. Check:
  - materials only where the spec allows;
  - contrast holds;
  - the illustration in both slots;
  - "card" everywhere people read;
  - the marker's wording.

  Record the results here.
  - Result (2026-10-25), on the demo. Each part was checked as it was built (1.3 to 3.2, recorded above), then together:
    - **Welcome pages:** the sign-in and not-found pages show porch boards and grain on the ground, with the card plain and the porch scene below, in both themes.
    - **As Maya:**
      - brass on the briefing card and the add button only;
      - glass on the arrange toolbar, Puck's card bar and the Assistant's composer, readable with content scrolled under it in both themes;
      - faint sidebar grain;
      - "Add a card" and "Building your card…";
      - a plain-sentence load error;
      - "Picked with Jordan's age in mind" on Jordan's kid-safe cards;
      - a small porch scene in an empty "About the kids" conversation.
    - **As Jordan:** no kid-safe marker on his own page, "Add a card", and a new Explore chat's empty state with the small scene, in Porch light.
    - **Checks:** `check:contrast` (44 pairings, including the worst case on glass and wood) and `check:copy` pass.
    - The demo was rebuilt afterwards.
- [x] 4.3 Run `openspec validate brand-surfaces --strict`, `make check`, and the frontend lint, `npm test`, `check:contrast`, `check:copy` and build. Push, open the PR, and confirm CI passes on it. Add the "Brand surfaces" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
  - Progress (2026-10-25): the change and all 14 specs validate strictly, and `make check` passes. The frontend lint is clean, `npm test` passes 11 tests, `check:contrast` passes 44 pairings, `check:copy` is clean, and the build passes. The Post-PoC line is in both files. Pushing, the PR and CI are next.
  - CI passed on PR #19, and the owner merged it (2026-10-25).
