// The member's theme: Daybreak (light), Porch light (dark), or Automatic,
// which follows the device. <html data-theme> always says which one is
// showing ("light" or "dark"), and styles/index.css redefines the color
// tokens for "dark". index.html's first script sets it before the first
// paint from the choice remembered here; this module keeps it current.
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
let chosen: Theme = 'auto'

function paint() {
  const showing = chosen === 'auto' ? (darkDevice.matches ? 'dark' : 'light') : chosen
  document.documentElement.dataset.theme = showing
  // index.html has one theme-color tag per device scheme. Automatic leaves
  // them to the device; a fixed choice gives both the chosen ground.
  document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]').forEach((meta) => {
    const scheme = meta.media.includes('dark') ? 'dark' : 'light'
    meta.content = GROUND[chosen === 'auto' ? scheme : chosen]
  })
}

darkDevice.addEventListener('change', () => {
  if (chosen === 'auto') paint()
})

/** Shows theme now, and remembers it on this device for the next page load. */
export function applyTheme(theme: Theme) {
  chosen = theme
  try {
    localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // Storage can be off (a private window); the page still shows the theme.
  }
  paint()
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
