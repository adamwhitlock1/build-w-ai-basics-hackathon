# Kinlight: the lantern in the product

2026-10-23. The lantern (logo concept 1) is the chosen mark, and Kinlight is the working name. This is how the lantern shapes the product itself: what it means, how it moves, and the details that make the app feel made with care. The logo files are in [`logo/`](logo/); the live demos are on the lantern system page.

## What the lantern means

**Light is attention.** A lit lantern says "this is for you, now." So light is spent carefully:

1. **One lit thing per screen.** At most one element glows at full strength at a time: the briefing in the morning, a card that just arrived, the AI while it works. Everything else is lit evenly and calmly. When everything glows, nothing does.
2. **Light means fresh.** A card's warm edge fades as its content ages. "Updated 2h ago" becomes something you see at a glance as well as read. The text stays for accessibility; the glow is a second signal, never the only one.
3. **Lit means someone's there.** In the family list, each member has a small lantern, lit once they've checked in today (opened their page or finished their morning steps). It's opt-in, it resets overnight, and it never uses location.
4. **The lantern is a physical object.** It hangs from a ring, it has weight, it swings when something lands near it, it warms up rather than switching on. Its motion follows physics, so it never bounces like a toy.

## Moments

| Moment | Where | What the lantern does | Why |
| --- | --- | --- | --- |
| **Kindle** | Opening the app, signing in | The glass warms from dark to amber with a single small flicker, the flame rises, and the glow spreads. 900 ms. | The first second sets the tone: a light being lit for you, not a splash screen. |
| **Thinking** | The AI writing a briefing or a described card | The flame breathes: the glass brightens and settles every 2.8 s, the glow follows. Replaces every spinner. | Waiting feels like someone's working by lamplight, not like a stalled machine. |
| **A card arrives** | A new or refreshed card lands | The header lantern swings a few degrees and settles; the new card's top edge catches the light, then cools to normal over 1.2 s. | The light points you to what changed, then gets out of the way. |
| **Fresh to settled** | Every card | The card's warm top edge fades over about 6 hours. | Freshness at a glance. |
| **Dusk** | Theme change at sunset (opt-in) or by hand | Night spreads outward from the header lantern in a circle, 1.1 s. Back to Daybreak, the light draws in. | The theme switch becomes the porch light coming on. |
| **Off you go** | Finishing the morning steps | The last step lights, then the lantern dims softly: the porch light goes off as everyone heads out. 700 ms. | The morning ends with a send-off, not a confetti burst. |
| **Who's home** | Family list | Members' lanterns light when they check in today. | Presence without tracking. |
| **Pull to refresh** | Phone | Pulling lowers the lantern on its chain; release, and it kindles. | The gesture already looks like pulling a lamp chain. |

## Motion tokens

| Token | Value | Use |
| --- | --- | --- |
| `--ease-warm` | `cubic-bezier(0.22, 0.8, 0.2, 1)` | Light warming up or spreading: quick start, long settle |
| `--ease-cool` | `cubic-bezier(0.4, 0, 0.6, 1)` | Light dimming: even and unhurried |
| `--swing` | Damped keyframes: 7°, −4.5°, 2.5°, −1°, 0 over 1.4 s | The lantern, pivoting on its ring |
| `--t-kindle` | 900 ms | Light coming on |
| `--t-dim` | 700 ms | Light going off |
| `--t-catch` | 1200 ms | A card catching the light, then cooling |
| `--t-dusk` | 1100 ms | Theme change |
| `--t-breathe` | 2800 ms, looping | Thinking; the only loop in the product |

Rules: animate only `opacity` and `transform` (and `clip-path` for dusk), so motion stays smooth at 120 Hz. Nothing loops except thinking. With reduced motion turned on, every moment becomes a plain fade of the same length, with no swing, flicker or spreading.

## Materials that make it feel premium

- **Brass.** A brass hairline (a 1 px gradient from `#E7C98E` to `#B08D57`) on the few things that matter: the briefing card, the add button, the app icon's lantern. Brass replaces chrome and colored outlines.
- **Lantern glass.** Sheets and menus use frosted, faintly warm glass: background blur, a 4% amber tint and a whisper of grain, so they look like seeing through a lantern pane.
- **Real light.** Glows fall off smoothly over several steps and carry fine noise, so they never show bands. Shadows fall away from the header lantern (down and slightly right), and in the Porch light theme card edges facing the lantern catch a little light.
- **The pane.** The trapezoid lantern pane is a small recurring shape: the i's dots in the wordmark, the selected-tab marker, the unread dot.
- **Sound and touch.** Off by default and opt-in. A soft lamp-switch click on kindle, nothing else. Haptics: a soft impact on kindle and when a card lands; nothing on routine taps.

## What we never do

- Glow on errors, buttons or decoration. Errors are terracotta and still.
- Flicker more than once, or flicker to show something broken.
- Make the lantern a mascot: it never has a face, never talks, never celebrates.
- Use light to nag. A kid's unlit lantern is never shown to them as a failure.
- Use light as the only signal. Every glow has words beside it.
