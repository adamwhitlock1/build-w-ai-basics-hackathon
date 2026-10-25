## Why

`brand/` holds a finished identity: the Kinlight name, the lantern mark, two themes and new typefaces. The app still wears the PoC look, with indigo buttons, an amber dot, Fontshare fonts and no dark theme. The family's first real use, on the homelab (series 2, change 4), should be the finished identity. This change lays its foundation; motion (`lantern-moments`) and materials (`brand-surfaces`) build on it.

## What Changes

- **The name.** Everything a family member reads says **Kinlight**:
  - the page title, the header, sign-in and invite pages, chat labels, and error messages;
  - the agents' and the other AI prompts' references to the app;
  - the OpenRouter app title and the calendar reader's user agent;
  - the README header.

  Code, the repo, the Go module, `PORCHLIGHT_*` environment variables, the database file and cookie names keep "porchlight", so nothing breaks and a later rename is a copy change.
- **The lantern.** The favicon (SVG and ICO), the Apple touch icon and the header lockup come from `brand/logo/`.
  - The header lockup replaces the amber dot and the "Porchlight" text.
  - The lockup is themed, so one drawing works in both themes.
- **Typefaces.** Fraunces (display, with its soft axis) and Figtree (body and UI) are bundled with the app. The runtime Fontshare links go away, so no font request leaves the family's server.
- **Two themes.** The brief's color tokens become the app's tokens:
  - **Daybreak** (light) and **Porch light** (dark, warm walnut).
  - Haint blue replaces indigo as the UI color.
  - Amber stops being a button fill and is kept for "light" (fresh, new, thinking).
  - Errors stay terracotta.
  - Porch light is a set of token overrides, not a second stylesheet.
  - Every text and control pairing meets WCAG AA in both themes, checked by a script in CI.
- **The theme switch.** The theme follows the device by default. Each member can pick Daybreak, Porch light or Automatic, and the choice is saved on the server so it follows them to every device. The page never flashes the wrong theme while loading.
- **Kids by name.** The parents' Assistant refers to the kids by name rather than guessing pronouns.

Out of scope, and planned next:
- motion: kindle, breathing, the dusk animation and freshness edges (`lantern-moments`);
- wood, brass and glass, the illustration, and the copy voice pass (`brand-surfaces`).

## Capabilities

### New Capabilities
- `app-identity`: the name the app shows, its icons and header lockup, its self-hosted typefaces, the two themes and their contrast, and each member's theme choice.

### Modified Capabilities
- `parent-assistant`: the Assistant refers to the kids by name rather than guessing pronouns.

## Impact

- **Frontend:**
  - `index.html` (title, icons, theme-color, a tiny pre-paint theme script, no Fontshare);
  - `public/` icons;
  - `src/styles/index.css` (tokens for both themes);
  - `BrandMark` becomes the lantern lockup;
  - buttons that use amber fills or `text-white`;
  - avatar tints in `family/people.ts`;
  - a theme switch in the sidebar;
  - the name in about a dozen strings;
  - new dependencies `@fontsource-variable/fraunces` and `@fontsource-variable/figtree`;
  - a contrast-check script run by CI.
- **Backend:**
  - migration `009_theme.sql` (a `theme` column on `users`);
  - `theme` in `/api/me`, and a new `PUT /api/me/theme`;
  - the name in user-facing errors, LLM prompts, the OpenRouter title and the calendar user agent;
  - the Assistant's prompt rule about kids' names;
  - tests and evaluation text that quote the old name.
- **Docs:** the README header and intro. `devpost/` gets only the Post-PoC line at archive.
- **Not affected:** the API paths, sessions, the database file, environment variables, and the demo seed's data.
