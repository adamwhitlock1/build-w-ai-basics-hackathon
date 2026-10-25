// A card that arrives asks the header lantern to swing (brand/lantern-ux.md,
// "A card arrives"). It's an event rather than shared state because the
// lantern and the cards live in different parts of the page.

const EVENT = 'kinlight:lantern-swing'

/** Swings the header lantern, if one is showing. */
export function swingLantern() {
  window.dispatchEvent(new Event(EVENT))
}

/** Calls onSwing each time a card arrives; returns the unsubscribe. */
export function onLanternSwing(onSwing: () => void): () => void {
  window.addEventListener(EVENT, onSwing)
  return () => window.removeEventListener(EVENT, onSwing)
}
