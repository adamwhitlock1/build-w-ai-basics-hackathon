## Why

Kinlight now has its name, colors, type (`kinlight-identity`) and behavior (`lantern-moments`). What's left from `brand/brief.md` and `brand/lantern-ux.md` is how it *feels to the touch* and how it *talks*:
- the materials: brass, lantern glass and wood;
- a place for the porch illustration;
- the voice: plain, warm and specific.

The app still says "widget" everywhere, while the brief, and how a family would say it, is "card". It's the last brand change before the homelab deployment.

## What Changes

- **Brass.** A 1 px brass hairline (a gradient from `#E7C98E` to `#B08D57`) on the few things that matter: the briefing card and the add-a-card button. The app icon's lantern already has its brass rim from `brand/logo/build.py`. Brass replaces colored outlines there, and is used nowhere else.
- **Lantern glass.** Surfaces that float over the page get frosted, faintly warm glass (background blur, a 4% amber tint and a whisper of grain): the sticky arrange toolbar, Puck's card action bar, and the sticky chat and Assistant composers. This replaces today's plain blur, and Puck's own dark bar.
- **Wood.** A faint grain texture on brand surfaces only, never behind content:
  - the sign-in, sign-up, invite and create-family pages;
  - the sidebar;
  - empty states.

  Porch boards are a layout rhythm on the welcome pages: horizontal bands with a fine seam.
- **The porch illustration, as a slot for a commission.** The owner chose to commission the scene. This change builds where it goes:
  - a wide slot on the welcome pages, and a smaller one in empty states;
  - both themed for Daybreak and Porch light;
  - with a simple stand-in (porch boards, the haint-blue ceiling and the lantern; no people, no faces);
  - and a documented drop-in format, so the commissioned SVG replaces the stand-in without code changes.
- **Voice** (brief section 8):
  - **"card", not "widget".** Every label, message and hint people read says "card" ("Add a card", "Building your card…", "That card doesn't exist."). Code, API paths, the database and the AI's internal instructions keep "widget".
  - **The kid-safe marker** tells a parent "Picked with Jordan's age in mind" instead of "Kid-safe".
  - **Empty states** say what to do in plain words ("Nothing here yet. Tell Kinlight what you'd like to see.").
  - **Errors** say what didn't work and when to try again, with no status codes or jargon.
  - **Banned phrases** never appear: "mental load", "in sync", "supercharge", "seamless" and "powered by AI". A check in CI enforces it.
- **Contrast.** Contrast stays WCAG AA in both themes; textures and glass never lower the contrast of text on them.

Out of scope:
- **Need features that don't exist yet:** morning steps ("Off you go") and the evening moment.
- Sound, the marketing site and a wall-display mode.
- Commissioning the illustration itself.

## Capabilities

### New Capabilities
<!-- none: the materials, illustration slot and voice extend app-identity -->

### Modified Capabilities
- `app-identity`: adds brass, lantern glass and wood, the illustration slot, and the voice, including "card" in everything people read.
- `kid-safe-content`: the parent-facing marker reads "Picked with <name>'s age in mind" instead of "Kid-safe".

## Impact

- **Frontend:**
  - `styles/index.css`: brass, glass and wood tokens and utilities for both themes, and a grain texture as an inline SVG;
  - `WelcomeLayout`, the sidebar, empty states, `BriefingWidget`, the add tile, the arrange toolbar, Puck's action bar and the chat composers;
  - a `PorchScene` component with its slots;
  - the copy in about 25 places, including the kid-safe marker.
- **Backend:** about 8 user-facing error messages and hints that say "widget", plus the tests that quote them.
- **Checks:**
  - `check:contrast` gains the pairings for text on the textured and glass surfaces;
  - a small copy check (`npm run check:copy`) fails on "widget" in visible strings or a banned phrase, and CI runs it.
- **Docs:** the README's "Look and themes"; `brand/` gets the illustration's drop-in format.
