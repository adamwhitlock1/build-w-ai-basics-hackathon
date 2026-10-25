# Kinlight: the porch illustration

The app has a place for a porch scene, today filled by a plain stand-in (`frontend/src/brand/porch.svg`). This is the format a commissioned illustration follows, so that replacing that one file puts it everywhere, in both themes, with no code change.

## Where it appears

| Slot | Where | Size on screen | How it's fitted |
| --- | --- | --- | --- |
| Wide | Under the card on the sign-in, sign-up, invite and create-family pages | The card's width (up to 576 px), at the drawing's own 2:1, so the whole scene shows | Fills the width, anchored to the bottom (if a screen ever gives it a wider shape, the ceiling crops first) |
| Small | Beside the sentence in empty states (a new chat, a new Assistant conversation, nothing to review, no conversations yet) | 112 × 56 to 128 × 64 px | The whole scene, centred |

## The file

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

1. Save it over `frontend/src/brand/porch.svg`.
2. Run `npm --prefix frontend run dev` and open the sign-in page, then switch between Daybreak and Porch light.
3. Open an empty chat or the Assistant with no conversations for the small slot.
