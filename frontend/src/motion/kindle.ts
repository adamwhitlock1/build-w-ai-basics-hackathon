// When the header lantern kindles: the first time it's drawn in a browser
// tab, and again after someone signs in. sessionStorage is per tab and
// survives reloads, so moving between pages doesn't kindle it again.

const KEY = 'kinlight.kindled'

// True until the current task ends after a kindle was taken, so a render
// that runs twice (React's StrictMode does, in development) gets the same
// answer both times.
let justTaken = false

/** Whether the lantern should kindle now; the first caller in a tab gets true, once. */
export function takeKindle(): boolean {
  if (justTaken) return true
  try {
    if (sessionStorage.getItem(KEY)) return false
    sessionStorage.setItem(KEY, '1')
  } catch {
    // Storage off: kindling on each mount is harmless.
  }
  justTaken = true
  setTimeout(() => {
    justTaken = false
  })
  return true
}

/** Lets the lantern kindle again, for the first page after signing in. */
export function rekindle() {
  try {
    sessionStorage.removeItem(KEY)
  } catch {
    // Nothing was stored, then.
  }
}
