# Kinlight: the porch illustration

`PorchScene` uses the detailed porch artwork on the landing and welcome pages, and a simpler SVG in small empty states.

## Where it appears

| Slot | Where | Size on screen | How it's fitted |
| --- | --- | --- | --- |
| Wide | Landing-page hero; under the card on sign-in, sign-up, invite and create-family pages | The container's width, at the artwork's 2:1 ratio | The entire scene stays visible, including the central lantern and steps |
| Small | Beside the sentence in empty states (a new chat, a new Assistant conversation, nothing to review, no conversations yet) | 112 × 56 to 128 × 64 px | The whole scene, centred |

## Wide artwork

- **Sources.** `brand/illustrations/kinlight-porch-hero-v2.png` for Daybreak and `brand/illustrations/kinlight-porch-night.png` for Porch light, with their generation prompts beside them.
- **Web assets.** `frontend/src/brand/porch-hero.webp` (about 202 KiB) and `frontend/src/brand/porch-hero-night.webp` (about 97 KiB), both 1774 × 887 px. Vite gives each CSS image URL a content hash.
- **Themes.** CSS selects the artwork from the root's resolved `data-theme`, so it follows saved choices and Automatic mode, including changes while the page is open. Only the selected background needs to load. The artwork keeps its own colors without a filter.
- **Layout.** A CSS aspect ratio reserves a 2:1 area before the image loads. The background fits inside it without cropping, stretching or changing the layout when themes switch.
- **Accessibility.** The scene is a decorative background hidden from screen readers. Page text carries the meaning.

To regenerate the web asset from the approved source:

```sh
cwebp -q 84 -m 6 -metadata none brand/illustrations/kinlight-porch-hero-v2.png -o frontend/src/brand/porch-hero.webp
cwebp -q 84 -m 6 -metadata none brand/illustrations/kinlight-porch-night.png -o frontend/src/brand/porch-hero-night.webp
```

## Small SVG

- **Format.** One SVG, saved over `frontend/src/brand/porch.svg`.
- **Canvas.** `viewBox="0 0 1200 600"`. No `width`, `height` or `preserveAspectRatio` attributes; the app sets the fit per slot.
- **Safe area.** Both slots show the whole canvas today. Keep what matters between x 150 and 1050 and below y 60 anyway, so the scene survives a tighter crop later.
- **Colors.** Every fill and stroke uses one of these CSS variables, set from the current theme. Never a fixed color.

| Variable | Is | Daybreak | Porch light |
| --- | --- | --- | --- |
| `--pi` | Ink: outlines, the lantern's frame | walnut `#2B211B` | `#F4E9DB` |
| `--pl` | Light: the lantern's glass and glow | lamplight `#F3A73B` | `#FFB547` |
| `--pg` | The lantern's bright core | `#FFE3A6` | `#FFE8B8` |
| `--pw` | Wood | pine `#DDBE95` | cedar `#B07A52` |
| `--pd` | Seams, shade, deep wood | cedar `#9A6440` | `#6B4529` |
| `--ph` | The porch ceiling | haint pale `#CFE5E1` | `#34342F` |

  Write them as `style="fill:var(--pw)"`. Opacity is fine for shading and glow.
- **Content.** A porch at the time of day the theme suggests: boards, steps, posts, the haint-blue ceiling, the lantern. No people, no faces, no text, and nothing that has to be seen to understand the page: the scene is decoration and hidden from screen readers.
- **Inside the file.** No embedded raster images, scripts, fonts or external references. Keep it under about 60 KB.

## Checking a new drawing

1. Run `npm --prefix frontend run dev` and open `/` while signed out. Check mobile and desktop widths in Daybreak and Porch light: the correct artwork appears, the lantern and steps remain visible and the page does not scroll sideways. Check a saved theme opposite the device theme, and change the device theme while using Automatic.
2. Check the sign-in and sign-up pages in both themes for the narrower wide slot.
3. Open an empty chat or the Assistant with no conversations for the small SVG slot.
