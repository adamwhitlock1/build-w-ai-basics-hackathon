// Checks the color tokens in src/styles/index.css against WCAG 2 AA, in
// both themes: 4.5:1 for text, 3:1 for focus rings and control edges.
// Daybreak is the @theme block; Porch light is the :root[data-theme="dark"]
// block, falling back to Daybreak for any token it doesn't redefine.
//
//   npm run check:contrast
//
// PAIRINGS lists every foreground-on-background pair the app uses. When a
// component puts a token on a new background, add the pair here.

import { readFileSync } from 'node:fs'

const css = readFileSync(new URL('../src/styles/index.css', import.meta.url), 'utf8')

/** The --color-* tokens declared in the block that starts at marker. */
function tokens(marker) {
  const start = css.indexOf(marker)
  if (start < 0) throw new Error(`index.css has no ${marker} block`)
  // Blocks here hold no nested braces before their color tokens end, so the
  // first closing brace after the marker ends the block.
  const block = css.slice(start, css.indexOf('}', start))
  const out = {}
  for (const [, name, hex] of block.matchAll(/--color-([\w-]+):\s*(#[0-9a-fA-F]{6})\s*;/g)) {
    out[name] = hex.toLowerCase()
  }
  return out
}

const daybreak = tokens('@theme {')
const themes = {
  Daybreak: daybreak,
  'Porch light': { ...daybreak, ...tokens(':root[data-theme="dark"] {') },
}

const TEXT = 4.5
const UI = 3

/** [foreground, background, minimum ratio, where it's used] */
const PAIRINGS = [
  ['ink', 'bg', TEXT, 'body text'],
  ['ink', 'surface', TEXT, 'card text'],
  ['ink-soft', 'bg', TEXT, 'secondary text'],
  ['ink-soft', 'surface', TEXT, 'secondary text on cards'],
  ['ink-muted', 'bg', TEXT, 'hints and captions'],
  ['ink-muted', 'surface', TEXT, 'hints and captions on cards'],
  ['ink', 'primary-soft', TEXT, 'the selected member row'],
  ['primary', 'bg', TEXT, 'links'],
  ['primary', 'surface', TEXT, 'links and outline buttons on cards'],
  ['primary', 'primary-soft', TEXT, 'notices and kind chips'],
  ['primary-strong', 'primary-soft', TEXT, 'the parent tag'],
  ['on-primary', 'primary', TEXT, 'primary buttons'],
  ['on-primary', 'primary-strong', TEXT, 'primary buttons, hovered'],
  ['ink-soft', 'accent-soft', TEXT, 'tool notes'],
  ['ink', 'accent-soft', TEXT, 'the chat mode chip'],
  ['accent-strong', 'bg', TEXT, 'text-safe light'],
  ['accent-strong', 'surface', TEXT, 'text-safe light on cards'],
  ['danger', 'bg', TEXT, 'errors'],
  ['danger', 'surface', TEXT, 'errors on cards'],
  ['danger', 'danger-soft', TEXT, 'flag notices'],
  ['on-danger', 'danger', TEXT, 'danger buttons and badges'],
  ['bg', 'ink-soft', TEXT, 'the "you" pill'],
  ['ink', 'avatar-1', TEXT, 'avatar initials'],
  ['ink', 'avatar-2', TEXT, 'avatar initials'],
  ['ink', 'avatar-3', TEXT, 'avatar initials'],
  ['primary', 'bg', UI, 'focus rings'],
  ['primary', 'surface', UI, 'focus rings on cards'],
  ['accent-strong', 'surface', UI, 'focus rings on light-colored controls'],
  ['control', 'bg', UI, 'field edges'],
  ['control', 'surface', UI, 'field and composer edges'],
]

function luminance(hex) {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function ratio(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

let failures = 0
for (const [theme, colors] of Object.entries(themes)) {
  for (const [fg, bg, min, use] of PAIRINGS) {
    if (!colors[fg] || !colors[bg]) {
      console.error(`${theme}: no token --color-${colors[fg] ? bg : fg} (${use})`)
      failures++
      continue
    }
    const r = ratio(colors[fg], colors[bg])
    if (r < min) {
      console.error(`${theme}: ${fg} on ${bg} is ${r.toFixed(2)}:1, needs ${min}:1 (${use})`)
      failures++
    }
  }
}

if (failures > 0) {
  console.error(`${failures} pairing(s) fail WCAG AA.`)
  process.exit(1)
}
console.log(`Contrast: ${PAIRINGS.length} pairings pass WCAG AA in both themes.`)
