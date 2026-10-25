// The member's theme: Daybreak (light), Porch light (dark), or Automatic,
// which follows the device. <html data-theme> always says which one is
// showing ("light" or "dark"), and styles/index.css redefines the color
// tokens for "dark". index.html's first script sets it before the first
// paint from the choice remembered here; this module keeps it current.
//
// A change a person causes (the switch, or the device going dark while
// Automatic is chosen) is shown as dusk (brand/lantern-ux.md): night spreads
// out from the header lantern, or the light draws back into it, through a
// view transition. Page loads, signing in and logging out never animate.
import type { Theme } from './api/client'

/** What each choice is called in the app. */
export const themeNames: Record<Theme, string> = {
  auto: 'Automatic',
  light: 'Daybreak',
  dark: 'Porch light',
}

/** Where the last choice on this device is kept, for index.html's script. */
const STORAGE_KEY = 'kinlight.theme'

/** Each theme's ground, for the browser bar (the theme-color meta tags). */
const GROUND = { light: '#f7f2ea', dark: '#18120e' }

const darkDevice = window.matchMedia('(prefers-color-scheme: dark)')
/** What index.html's script showed before the first paint: the remembered choice. */
function remembered(): Theme {
  try {
    const theme = localStorage.getItem(STORAGE_KEY)
    if (theme === 'light' || theme === 'dark' || theme === 'auto') return theme
  } catch {
    // Storage off: the script fell back to Automatic too.
  }
  return 'auto'
}

let chosen: Theme = remembered()

/** The theme a choice shows on this device. */
function showingFor(theme: Theme): 'light' | 'dark' {
  return theme === 'auto' ? (darkDevice.matches ? 'dark' : 'light') : theme
}

function paint() {
  document.documentElement.dataset.theme = showingFor(chosen)
  // index.html has one theme-color tag per device scheme. Automatic leaves
  // them to the device; a fixed choice gives both the chosen ground.
  document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]').forEach((meta) => {
    const scheme = meta.media.includes('dark') ? 'dark' : 'light'
    meta.content = GROUND[chosen === 'auto' ? scheme : chosen]
  })
}

darkDevice.addEventListener('change', () => {
  if (chosen === 'auto') dusk(paint)
})

function remember(theme: Theme) {
  chosen = theme
  try {
    localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // Storage can be off (a private window); the page still shows the theme.
  }
}

/**
 * Shows theme now, without dusk, and remembers it on this device for the
 * next page load: for loading a page and signing in. A theme that's already
 * chosen is left alone, so a dusk that changeTheme started isn't undone by
 * painting its end state early.
 */
export function applyTheme(theme: Theme) {
  if (theme === chosen) return
  remember(theme)
  paint()
}

/** A change a person made: shows theme with dusk, and remembers it. */
export function changeTheme(theme: Theme) {
  remember(theme)
  dusk(paint)
}

/**
 * Runs update, which paints the theme, as dusk when the theme showing
 * changes: a view transition clipped to a circle from the header lantern
 * (styles/index.css). Without view transitions, or in a tab nobody's
 * looking at (where it would wait for a frame that never comes), the theme
 * just changes.
 */
function dusk(update: () => void) {
  const root = document.documentElement
  const to = showingFor(chosen)
  if (root.dataset.theme === to || !document.startViewTransition || document.visibilityState !== 'visible') {
    update()
    return
  }
  // The header lantern's centre, or the top-left corner if there isn't one,
  // and the radius that reaches the farthest corner of the window.
  const lantern = document.querySelector('[data-header-lantern]')?.getBoundingClientRect()
  const x = lantern ? lantern.left + lantern.width / 2 : 0
  const y = lantern ? lantern.top + lantern.height / 2 : 0
  const r = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
  root.style.setProperty('--dusk-x', `${x}px`)
  root.style.setProperty('--dusk-y', `${y}px`)
  root.style.setProperty('--dusk-r', `${r}px`)
  // "out": the porch light comes on and night spreads; "in": it draws back.
  root.dataset.dusk = to === 'dark' ? 'out' : 'in'
  const transition = document.startViewTransition(update)
  void transition.finished.finally(() => delete root.dataset.dusk)
}

/** Forgets the choice on this device (on logout): back to Automatic. */
export function forgetTheme() {
  chosen = 'auto'
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // Nothing was stored, then.
  }
  paint()
}
