Verify visual work in the browser on the demo (`make demo`, then `KINLIGHT_KIDS_ALLOW_RETENTION=1 make demo-serve` and `npm --prefix frontend run dev`) in both themes. Run Go with `CGO_ENABLED=0 GOTOOLCHAIN=local` (or `make`). Commit after each task.

## 1. Materials

- [ ] 1.1 Add the brass and wood tokens for both themes, the `--grain` texture, and the `brass-edge`, `lantern-glass` and `wood-grain` utilities (design decision 1). Verify that `npm run build` passes and the CSS output has the three utilities.
- [ ] 1.2 Extend `contrast.mjs` with blended backgrounds for glass and wood, and their pairings (design decision 2). Adjust a material's strength if any pairing fails, and record it here. Verify that `npm run check:contrast` passes, and fails when a material's strength is raised on purpose.
- [ ] 1.3 Apply brass to the briefing card and the add-a-card button, glass to the arrange toolbar, Puck's action bar and both composers, and wood grain to the sidebar (design decision 3). Verify in the browser in both themes:
  - brass on exactly those two;
  - glass stays readable with cards scrolled under it, including Puck's bar in Porch light;
  - the sidebar grain is faint, and no card, chat or form has texture behind its text.

## 2. The illustration slot

- [ ] 2.1 Add the stand-in `porch.svg` and `PorchScene` with its `wide` and `small` slots (design decision 4). Put `wide` and the porch-board wood on `WelcomeLayout`, and `small` in the empty states: a new kid chat, a new Assistant conversation, Reviews with nothing to review, and the Assistant list with none. Write `brand/illustration.md`. Verify in the browser in both themes on each of those pages: no faces, hidden from screen readers. Swap in a test SVG made to the format and confirm it shows in both slots with no code change.

## 3. Voice

- [ ] 3.1 Add `scripts/copy-check.mjs` and `npm run check:copy` (design decision 5), with small tests in `tests/copy-check.test.ts` (a JSX case, a string-literal case, a Go `respond.Error` case, and a query key that passes), and run it in CI. Run it now and list what it finds here: that's the to-do list for 3.2 and 3.3.
- [ ] 3.2 Rename "widget" to "card" in every frontend string it found, write the empty-state and error copy, and change the kid-safe marker to "Picked with <name>'s age in mind" (design decision 5). Verify:
  - `npm run check:copy` passes;
  - in the browser: "Add a card", "Building your card…", an empty spot, a card's load error, and the marker on Jordan's page as Maya (and no marker as Jordan).
- [ ] 3.3 Rename "widget" to "card" in the backend messages and hints it found, and update the tests that quote them. Verify `make check` and `npm run check:copy`.

## 4. Docs and end to end

- [ ] 4.1 Update the README's "Look and themes" with the materials, the illustration slot (pointing to `brand/illustration.md`), the voice rules and `check:copy`. Verify by reading it.
- [ ] 4.2 On the demo, as Maya and as Jordan, in both themes: walk the welcome pages, home, arrange, Lily's page, chat, Assistant and Reviews. Check:
  - materials only where the spec allows;
  - contrast holds;
  - the illustration in both slots;
  - "card" everywhere people read;
  - the marker's wording.

  Record the results here.
- [ ] 4.3 Run `openspec validate brand-surfaces --strict`, `make check`, and the frontend lint, `npm test`, `check:contrast`, `check:copy` and build. Push, open the PR, and confirm CI passes on it. Add the "Brand surfaces" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
