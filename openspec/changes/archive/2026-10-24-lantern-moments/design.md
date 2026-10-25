## Context

See proposal.md for why. What exists today in `frontend/src`:

- **Motion.** `styles/index.css` has two animations: `widget-in` (a 520 ms rise for a new card) and `spin-calm` (a spinner). Spinners appear in eight places: `SubmitButton`, `RouteStates`, `MemberPage`, `ArrangeCanvas`, the invite page (twice), `DescribeWidgetPrompt` and `RefreshButton`. `animate-pulse` skeletons are in `WidgetCard`'s `LoadingRows`, `MemberPage` and `FamilySidebar`'s `MembersLoading`. Kid chat and the Assistant show "Thinking…" as text.
- **A new card.** `WidgetCard` takes `fresh`, from `LayoutView`'s `freshWidgetId`. A fresh card gets an amber border, a ring and `widget-in`, and takes focus. Cards carry `updatedAt`, and `useNow` keeps "Updated …" labels current.
- **The header lantern.** `BrandMark` inlines `brand/lockup.svg`: the lantern and wordmark in one drawing. That leaves no handle on the lantern's glass, glow or flame. `brand/logo/themed/lantern.svg` (a 64-unit grid) and `wordmark.svg` are available separately. In the lockup the lantern is about 96% of the height, with a gap of about 15% of the height before the wordmark.
- **Themes.** `theme.ts` paints `data-theme`. The root applies `me.user.theme` on every change, and `useSetTheme` updates the cached `me` optimistically.
- **Tests.** The frontend has no test runner. Node 22 runs TypeScript directly with `--experimental-strip-types`, and the app's tsconfig already sets `erasableSyntaxOnly`.

## Goals / Non-Goals

**Goals:**
- Every moment's timing and rule lives in one place: tokens in CSS, decisions in small pure functions with tests.
- Motion never carries meaning on its own.
- Every moment degrades to a fade, or to nothing, without breaking anything.

**Non-Goals:**
- Animating layout, size or color directly (only opacity and transform; clip-path for the dusk).
- An animation library.
- Animating route changes.
- The moments that need missing features (see the proposal).

## Decisions

### 1. Tokens and keyframes in `index.css`

`@theme` gains these tokens:
- the easings `--ease-warm: cubic-bezier(0.22, 0.8, 0.2, 1)` and `--ease-cool: cubic-bezier(0.4, 0, 0.6, 1)`;
- the durations `--duration-kindle` (900 ms), `--duration-dim` (700 ms), `--duration-catch` (1200 ms), `--duration-dusk` (1100 ms) and `--duration-breathe` (2800 ms);
- `--animate-*` entries for `kindle-glass`, `kindle-glow`, `kindle-flame`, `swing`, `catch` and `breathe`.

Keyframes animate only `opacity` and `transform`. The swing is damped through 7°, −4.5°, 2.5°, −1° and 0 over 1.4 s, pivoting on the lantern's ring (`transform-origin` at the ring's centre). Inside `@media (prefers-reduced-motion: reduce)`, the same `--animate-*` names point at fade-only keyframes of the same length, and the swing becomes `none`. So components never branch on motion settings.

`spin-calm`, `widget-in` and every `animate-pulse` use go away.

### 2. The lantern as its own component

`LanternMark` is the 64-unit lantern drawn in JSX, with named parts:
- the ring and cap;
- an unlit glass (ink at low opacity) and, above it, the lit glass (the light);
- the flame core (`glass-core`);
- the two glow circles.

Each lit part is a separate element, so kindling is the lit glass and the glow fading in, plus the flame rising (`translateY`), never a color change. `BrandMark` becomes `LanternMark` followed by `wordmark.svg` inlined as before, laid out to match the lockup's proportions (lantern height ≈ the wordmark's, gap ≈ 15% of the height).

The header lantern carries `data-header-lantern`, so the dusk can find where to start from.

### 3. Kindle: once per tab, and on sign-in

`LanternMark` in the header kindles on mount unless `sessionStorage['kinlight.kindled']` is set, then sets it. `startSession` (sign-in, sign-up, accepting an invite) clears it, so the lantern on the first signed-in page kindles. Storage can fail; then it kindles on mounts, which is harmless.

### 4. Thinking: one `Breathing` component

`Breathing` is a small lantern glyph (`LanternMark` at 16 to 20 px) whose glass and glow breathe on `--duration-breathe`, with its words beside it. `role="status"` and the text are on the wrapper, and the glyph is `aria-hidden`.

Its uses:
- **Inline**, beside "Thinking…", "Writing your briefing…" and "Loading your family…".
- **In buttons**, in place of `SubmitButton`'s spinner, next to the busy label. It uses `on-primary` colors there.
- **`RefreshButton`**, where the icon is swapped for the glyph while refreshing; its accessible label already says so.

Skeleton rows stay as static shapes under the breathing status, so the layout doesn't jump, but they stop pulsing.

### 5. One lit thing: a light coordinator

`LightProvider` at the root holds the current claims. A component calls `useLight(kind, active)` with:
- `kind`: `'thinking'`, `'arrival'` or `'briefing'`;
- `active`: whether it wants the full glow now.

`useLight` returns whether this component holds the glow. The pure function `holder(claims)` in `motion/light.ts` picks:
- the first claim of the highest-priority kind (thinking, then arrival, then briefing);
- the newest claim within a kind.

Components without the glow still show their words, and their light at the reduced level. A briefing card claims `briefing` only while `isMorning(now)` (before 12:00 local) and it has text.

*Alternative:* each component decides alone. That can't keep the rule when two things glow on one screen. Rejected.

*Revised during apply:* the claims live in a module-level store (`motion/useLight.ts`) read with `useSyncExternalStore`, rather than in a `LightProvider` component's state. Every component on the screen shares the one module, so there's nothing to wrap, and only components whose answer changes re-render.

### 6. A card arriving

`WidgetCard` remembers when it mounted and the `updatedAt` it last saw. The pure function `arrived(prev, next, mountedAt)` says a card arrived when:
- `next` is set;
- it differs from `prev`;
- `next` is no earlier than mount time minus 5 s.

That's content written during this visit, not a cached result loaded at page open. A `fresh` prop (a just-described card) is an arrival too.

On arrival the card:
- claims `arrival` for 1.2 s;
- plays `catch` on its top edge;
- dispatches a `lantern:swing` event, which the header `LanternMark` answers by swinging, unless a swing is already playing.

Focus on a fresh card stays as today.

### 7. The freshness edge

The pure function `edgeLevel(updatedAt, now)` returns 0.45 just after an update, falling linearly to 0 at 6 hours. That's below the arrival glow's full 1, so it never competes. `WidgetCard` draws the edge as an absolutely positioned top strip: a horizontal gradient of `accent` that fades out at both ends, plus a soft glow beneath, clipped to the card's rounded corners. Its opacity is set from `edgeLevel` on each `useNow` tick (minutes), and the catch animates it to 1 and back. Cards without `updatedAt` have no edge.

### 8. Dusk through View Transitions

`theme.ts` gains `changeTheme(theme)` for changes a person caused: the switch, or Automatic following the device.
- With `document.startViewTransition` available, and the page visible, it runs the theme update inside a view transition, after setting `data-dusk="out"` (to Porch light) or `"in"` (to Daybreak) on `<html>`, and `--dusk-x` and `--dusk-y` from the header lantern's centre (or the top-left corner if there's none).
- CSS animates the transition:
  - **Going dark:** `::view-transition-new(root)` grows a `clip-path: circle()` from 0 to the far corner over `--duration-dusk` with `--ease-warm`.
  - **Going light:** `::view-transition-old(root)` (the dark page) shrinks back into the lantern with `--ease-cool`, drawn above the new page.
- Under reduced motion, both become a plain cross-fade of the same length.
- Without view transitions, the theme just changes.

`applyTheme` (page load, sign-in, the root's sync with `me`) never animates. `useSetTheme`'s `onMutate` calls `changeTheme` before updating the cache. When the root then applies the same theme, nothing is left to change. The device listener in `theme.ts` calls `changeTheme` while Automatic is chosen.

*Alternative:* a full-screen overlay animated by hand. That's more code, a flash risk and worse performance. View Transitions are in current Chrome, Edge and Safari, and the fallback is today's behavior. Accepted.

### 9. Tests with Node's runner

The decisions sit in `frontend/src/motion/light.ts`, which has no imports: `holder`, `arrived`, `edgeLevel` and `isMorning`. They're tested in `frontend/tests/light.test.ts` with `node:test`. The tests live outside `src`, so the app's tsconfig, which has no Node types, never sees them. The tests import the module with its `.ts` extension, which Vite and `allowImportingTsExtensions` already accept. The script is `npm test` (`node --experimental-strip-types --test tests/`), and the CI frontend job runs it.

Everything visual is checked in the browser, including with reduced motion emulated.

## Risks / Trade-offs

- **[A hidden tab, or a tab in the background, doesn't run animations, so a view transition could stall the theme change.]** → `changeTheme` only uses a view transition when `document.visibilityState === 'visible'`; otherwise it paints at once.
- **[A card that refreshes in the background during a visit catches the light, maybe while the member is looking elsewhere.]** → Intended: the edge still shows it's fresh afterwards, and the catch is short.
- **[The breathing light is the only loop, but it's on screen during every wait.]** → It moves only brightness and a little scale, slowly. Under reduced motion it's opacity alone.
- **[Rebuilding the lockup from two parts could drift from the drawn lockup.]** → The proportions come from the lockup's own viewBox. Compare the two side by side in the browser.
- **[`--experimental-strip-types` prints a warning on Node 22.]** → Harmless. The flag is stable enough for plain modules with erasable syntax, which the tsconfig already enforces.

## Migration Plan

Frontend only, with no data or API change. Rollback is a revert.
