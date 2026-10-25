## Why

`kinlight-identity` gave the app the lantern, the colors and the type. `brand/lantern-ux.md` says what the lantern *does*: light is attention, it warms rather than switches on, and it points at what changed, then gets out of the way.

Today the app still waits with generic spinners and pulsing skeletons, marks a new card with a static amber ring, and switches themes instantly. This change gives the identity its behavior, before the family starts using the app on the homelab.

## What Changes

- **Motion tokens:** warm and cool easings, the lantern's swing, and five durations (kindle 900 ms, dim 700 ms, catch 1200 ms, dusk 1100 ms, breathe 2800 ms), in the same token file as the colors.
- **Kindle:** when the app opens in a tab, and after signing in, the header lantern's glass warms from dark to amber, with one small flicker and the glow spreading.
- **Thinking:** one breathing-light indicator replaces every spinner and pulsing skeleton:
  - the AI writing a reply, a briefing or a described card;
  - pages loading;
  - forms saving;
  - cards refreshing.

  "Thinking…" in kid chat and the Assistant becomes this indicator, still with its words. Breathing is the only loop in the app.
- **A card arrives:** when a card is added, or its content changes while the page is open, the header lantern swings and the card's top edge catches the light, then cools. This replaces today's static amber ring. Cards present when the page loads don't catch.
- **Fresh to settled:** every card with an update time gets a warm top edge that fades over about six hours, a second signal beside "Updated 2h ago". The text stays.
- **Dusk:** changing the theme spreads Porch light outward from the header lantern; going back to Daybreak draws the light in. It happens when the member uses the switch, or when Automatic follows the device, never on page load.
- **One lit thing per screen:** at most one element glows at full strength: the AI while it works, otherwise a card that just arrived, otherwise the morning briefing before noon. The freshness edges are dimmer and don't count.
- **Reduced motion:** with the device's reduced-motion setting on, every moment becomes a plain fade of the same length: no swing, flicker or spreading.

Out of scope:
- **Need features that don't exist yet:** "Off you go" (morning steps), "Who's home" (check-in presence), and pull to refresh.
- **Sound and haptics.**
- **For `brand-surfaces`:** brass, glass and wood, and the illustration.

## Capabilities

### New Capabilities
- `lantern-motion`: how the app moves: the kindle, thinking, a card arriving, the dusk theme change, one lit thing per screen, and reduced motion.

### Modified Capabilities
- `widget-freshness`: a card's age also shows as light, a warm top edge fading over about six hours, alongside the update time it already shows.

## Impact

- **Frontend only**, with no API or backend change:
  - `styles/index.css`: motion tokens and keyframes;
  - the header lantern becomes its own animatable component beside the wordmark;
  - a small "light" coordinator that grants the one full glow;
  - a breathing indicator that replaces the spinner, `animate-pulse` skeletons and "Thinking…" in about a dozen places;
  - `WidgetCard` gets the catch and the freshness edge;
  - the theme module runs the dusk change through the View Transitions API, where the browser has it.
- **Tests:** pure timing and priority logic in plain TypeScript modules, tested with Node's own test runner (`npm test`), which CI runs. Behavior is also checked in the browser.
- **Docs:** the README's "Look and themes" section.
