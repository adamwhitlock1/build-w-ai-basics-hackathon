import { useCallback, useId, useRef, useState, type RefObject } from 'react'
import type { ProfileField } from './interestsEditorContext'

interface InterestsEditorPanelOptions {
  /**
   * Says something through the page's live region: '' when the editor opens
   * (so the same words after the next save are announced again), and
   * `savedMessage` after a save that changed something. Pass something that
   * stays the same between renders, such as a state setter.
   */
  announce: (text: string) => void
  /** What to announce once a change is saved. */
  savedMessage: string
  /**
   * The page's own button that opens and closes the editor ("Your
   * interests"). Focus lands there after saving, since it stays put.
   */
  buttonRef: RefObject<HTMLButtonElement>
}

/**
 * Opening and closing a page's interests editor, and where keyboard focus
 * goes around it. Your page and a family member's page both use it:
 *
 * - the page's "Your interests" (or "Their interests") button, whose ref
 *   it passes in as `buttonRef`, gets `id` (for aria-controls), `isOpen`
 *   and `toggle`;
 * - while `focusRequest` is set, the page shows <InterestsEditor> with it,
 *   plus `id`, `handleSaved` and `handleCancel`;
 * - widgets open it with `open`, through InterestsEditorContext.
 */
export function useInterestsEditorPanel({ announce, savedMessage, buttonRef }: InterestsEditorPanelOptions) {
  // Null while closed, otherwise which field to focus. Each request is a new
  // object, so asking again while it's open (say, from a widget's "Add your
  // city" button) moves focus again.
  const [focusRequest, setFocusRequest] = useState<{ field: ProfileField } | null>(null)
  const id = useId()
  // Whatever opened the editor, so focus can go back there on Cancel.
  const openerRef = useRef<HTMLElement | null>(null)

  // useCallback keeps this function the same between renders, so the
  // widgets that read it from context don't re-render for nothing.
  const open = useCallback(
    (field: ProfileField = 'interests') => {
      const active = document.activeElement
      openerRef.current = active instanceof HTMLElement && active !== document.body ? active : null
      setFocusRequest({ field })
      announce('')
    },
    [announce],
  )

  /** Closes it without moving focus (say, when the page is about to be arranged). */
  const close = useCallback(() => setFocusRequest(null), [])

  /** Opens and closes it from the page's button; focus stays on the button. */
  function toggle() {
    if (focusRequest) setFocusRequest(null)
    else open()
  }

  function handleSaved(changed: boolean) {
    setFocusRequest(null)
    // The widget that opened the editor may be about to reload (and lose its
    // button), so land on the page's button, which stays put.
    buttonRef.current?.focus()
    if (changed) announce(savedMessage)
  }

  function handleCancel() {
    setFocusRequest(null)
    const opener = openerRef.current?.isConnected ? openerRef.current : buttonRef.current
    opener?.focus()
  }

  return { id, isOpen: focusRequest !== null, focusRequest, open, close, toggle, handleSaved, handleCancel }
}
