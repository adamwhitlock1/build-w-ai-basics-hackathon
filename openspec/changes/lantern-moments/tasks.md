Frontend only. Verify visual behavior in the browser on the demo (`make demo`, then `KINLIGHT_KIDS_ALLOW_RETENTION=1 make demo-serve` and `npm --prefix frontend run dev`), in both themes, and again with reduced motion emulated. Commit after each task.

## 1. Foundations

- [x] 1.1 Add `src/motion/light.ts` with `holder`, `arrived`, `edgeLevel` and `isMorning` (design decisions 5 to 7), `tests/light.test.ts` on `node:test`, and `npm test`. Run it in the CI frontend job. Tests:
  - `holder` picks thinking over arrival over briefing, and the newest claim within a kind;
  - `arrived` is false for a cached result at mount and for an unchanged time, and true for a newer time or a first result written during the visit;
  - `edgeLevel` is 0.45 at 0, about half at 3 h, and 0 at 6 h and beyond;
  - `isMorning` switches at local noon.

  Verify `npm test` passes and fails on a broken case.
- [x] 1.2 Add the motion tokens, the keyframes and their reduced-motion fades to `index.css` (design decision 1). Remove nothing yet. Verify with `npm run build`, and that the CSS output has the reduced-motion block.
  - Done. The built CSS has the durations and the reduced-motion block. Tailwind v4 only emits keyframes that a used `animate-*` utility refers to, so the keyframes appear once the components use them (checked again in 2.2, 3.1 and 4.2).

## 2. The lantern

- [x] 2.1 Add `LanternMark` (design decision 2) and rebuild `BrandMark` from it plus the inlined wordmark, with the header lantern marked `data-header-lantern`. Verify in the browser in both themes, beside the old lockup in a screenshot: same proportions, ink and light from the theme, accessible name "Kinlight".
  - Done: measured against `brand/logo/lockup.svg` at the header's 32 px. The lantern is 31 px (30.7 expected), the wordmark 81.1 × 31.4 px (81.2 × 31.4), and the gap 4.8 px (4.8). Ink and light follow both themes, and the accessible name is "Kinlight". The lantern's ink is `currentColor`.
- [x] 2.2 Kindle on first mount per tab and after sign-in (design decision 3). Verify:
  - a new tab kindles once;
  - moving between pages doesn't;
  - signing in does;
  - under emulated reduced motion it's a fade.
  - Done (2026-10-25):
    - A new tab kindles: the glow, glass and flame animations run.
    - A full page load in the same tab doesn't.
    - Signing in kindles the lantern on the first page.
    - Moving to Reviews and back, which remounts the sidebar, doesn't.
    - **Not emulated:** the browser pane can emulate the color scheme but not `prefers-reduced-motion`. The built CSS maps all three kindle animations to the plain `fade-in` of the same length under that query.
    - **Fixed during apply:** React's StrictMode runs the `useState` initializer twice in development, and the second call found the flag already set. `takeKindle` now returns the same answer to calls in the same task.
    - **Also fixed (found during 3.1):** the `fade-in` keyframes the reduced-motion fades use weren't in the build. Tailwind keeps `@theme` keyframes only when a utility it can see uses them, and these are reached through variables. Without the fix, reduced motion would have shown no kindle at all, rather than a fade. The keyframes now sit outside `@theme`, and the build has them.

## 3. Thinking

- [x] 3.1 Add `Breathing` (design decision 4), and replace every spinner, every `animate-pulse` and both "Thinking…" lines with it. Remove `spin-calm`. Verify:
  - a grep finds no `animate-spin`, `spin-calm` or `animate-pulse` in `src`;
  - in the browser: a chat reply, an Assistant reply, a briefing being written, a described card, the sign-in button, a page loading and a refresh each show a breathing light with words;
  - a screen reader's status text is the words only;
  - lint and build pass.
  - Done (2026-10-25), in the browser, with slow requests held by a patched `fetch` where needed:
    - **Each wait showed the breathing light with its words:** an Assistant reply ("Thinking…"), Lily's briefing being written ("Writing their briefing…", over still placeholder rows), a card refreshing (the light replaces the refresh icon, and the button is busy), describing a card ("Building your widget…"), and the sign-in button ("Logging in…", in the button's own text color).
    - **Statuses:** each holds only words, since the lantern is `aria-hidden`.
    - **Grep:** finds no `animate-spin`, `spin-calm` or `animate-pulse`.
    - **Not seen live:** the router's "Just a moment…" page, which only appears after a one-second wait. It uses the same component.
  - **Changed from the plan:** kid chat and the Assistant don't keep a separate "Thinking…" line. The reply's own empty bubble shows the breathing light and "Thinking…" from the moment the message is sent, with tool notes above it. Before, a tool note replaced the line with a bare "…" bubble.
  - Skeleton rows now sit still under a visible status. The sidebar and member-page loading text was screen-reader-only before; now everyone sees it.

## 4. Light on cards

- [x] 4.1 Add `LightProvider` and `useLight` at the root (design decision 5), with thinking, arrival and briefing claims wired. Verify in the browser:
  - before noon, the briefing is lit when nothing else is;
  - while the Assistant writes, nothing else glows at full strength.

  Test the morning case by overriding the clock in devtools if it's after noon.
  - Done (2026-10-25). With the page's clock set to 8 am:
    - The briefing read "Good morning, Maya", and its top edge was fully lit.
    - Describing a card (held by a patched `fetch`) moved the glow to the breathing light, and the briefing's edge went to 0.
    - When the request ended, the briefing took the light back.
  - **Changed from the design:** the claims live in a module-level store read with `useSyncExternalStore`, not in a `LightProvider` component. It's the same single source of truth, with nothing to wrap the app in. Every breathing light claims `thinking`. One that doesn't hold the glow shows its light at half strength, without the glow.
- [x] 4.2 Add the arrival (design decision 6): the card's catch, the claim and the header lantern's swing. Remove `widget-in` and the amber ring. Verify:
  - describing a card catches it, swings the lantern, and the card keeps focus;
  - refreshing one card catches only that card;
  - opening a page catches nothing;
  - reduced motion gives a fade and no swing.
  - Done (2026-10-25), on Maya's page, with a recorder noting each catch and swing:
    - Opening the page (7 cards): no catch, no swing.
    - A real refresh of Articles: only "Articles for you" caught, the lantern swung, and the label read "Updated just now".
    - A real described card ("Next US Total Solar Eclipse"): it caught, the lantern swung, and it had focus.
    - **Reduced motion:** not emulatable here. The catch moves only opacity, and the swing is `none` under that query.
  - **Fixed during apply:**
    - A described card mounts before it's marked fresh, so checking `fresh` only at mount missed it. Turning fresh on at any point now counts.
    - To keep lint clean, arrivals are worked out during render, by comparing with the last update time kept in state, and counted rather than timed. The effect only runs the timer and the swing.
  - Two cards were described on the demo while checking. Rebuilding the demo removes them.
- [x] 4.3 Add the freshness edge (design decision 7) to every card with an update time. Verify:
  - a just-refreshed card has a warm edge;
  - a card from hours ago is dimmer, and one older than 6 h has none;
  - the edge dims while the page stays open (shorten the clock in devtools to check);
  - the "Updated …" label is unchanged.
  - Done (2026-10-25), on Maya's page:
    - Edges matched their labels: 0.449 at "just now", 0.446 at 2 min, 0.415 at 28 min.
    - With the page's clock moved three hours ahead, every edge dimmed to about half (0.19 to 0.22) at the next minute tick, without a reload. The labels read "Updated 3h ago". The briefing, by then before noon, became the lit thing at 1.0.
    - Nothing left after six hours is covered by the unit tests.
  - **Fixed during apply:** a card whose footer, with its time, isn't showing (an empty "Happening near you") still had an edge, which made the light its only signal. The edge now follows the same condition as the visible time.
  - **For checking:** while the browser pane is hidden it doesn't render frames, so CSS transitions freeze at their start, and the computed opacity read 0 for cards that loaded after the page. The inline style holds the real level.

## 5. Dusk

- [x] 5.1 Add `changeTheme` and the view-transition CSS (design decision 8). Call it from the switch and from Automatic's device listener, never from `applyTheme`. Verify:
  - choosing Porch light spreads from the header lantern, and Daybreak draws back in;
  - Automatic following the device does the same;
  - page load, reload and signing in don't animate;
  - reduced motion cross-fades;
  - a hidden tab changes at once.
  - Done (2026-10-25), watching the view transition's animations frame by frame:
    - **Choosing Daybreak:** `data-dusk="in"`, with `dusk-gather` on the old page. It was centred on the header lantern ((35, 56) in both the CSS variables and the lantern's box), and the marker cleared afterwards.
    - **Choosing Porch light:** `data-dusk="out"`, with `dusk-spread` on the new page for 1100 ms.
    - **Automatic, with the emulated device going from dark to light:** the light drew back in.
    - **No dusk:** a reload, signing in (which applied Maya's Porch light at once), and a tab reporting itself hidden (the theme changed at once).
    - **Reduced motion:** not emulatable here. The CSS swaps both animations for a cross-fade of the same length under that query.
  - **Fixed during apply:**
    - `applyTheme` now ignores a theme that's already chosen, so the root's sync can't paint a dusk's end state before the transition captures the old page.
    - For the same reason, the module now starts from the remembered choice, as the pre-paint script does. Otherwise a member whose theme became Automatic elsewhere would keep this device's old theme.

## 6. Docs and end to end

- [x] 6.1 Update the README's "Look and themes" with the moments, reduced motion, and `npm test`. Verify by reading it.
- [ ] 6.2 On the demo, as Maya and as Jordan, in both themes and with reduced motion: open the app, move around, describe a card, refresh one, chat, ask the Assistant, and switch themes. Check that every moment matches `brand/lantern-ux.md` and the spec, at most one thing glows at full strength, nothing loops except breathing, and every glow has words beside it. Record the results here.
- [ ] 6.3 Run `openspec validate lantern-moments --strict`, `make check`, and the frontend lint, `npm test`, contrast check and build. Push, open the PR, and confirm CI passes on it. Add the "Lantern moments" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
