import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useState } from 'react'
import { ApiError, type Layout } from '../api/client'
import { saveLayout, setCachedLayout } from '../api/hooks'
import { layoutKey } from './layout'

/** How long arranging pauses before a change is saved, so a burst of moves is one save. */
const SAVE_DELAY_MS = 500
/** How long to wait before trying a failed save again. */
const RETRY_DELAY_MS = 5000

/** Where saving stands, for the Arrange toolbar. */
export interface SaveState {
  /**
   * idle: nothing changed yet; saving: a change is waiting or on its way;
   * saved: the server has everything; error: the last save failed.
   */
  status: 'idle' | 'saving' | 'saved' | 'error'
  /** Why the last save failed (status 'error'). */
  error: ApiError | null
  /** A failed save will be tried again by itself (the server or network hiccupped). */
  retrying: boolean
}

const IDLE: SaveState = { status: 'idle', error: null, retrying: false }

/**
 * Saves the layout while you arrange your page: each change is sent after a
 * short pause, one save at a time and in order, and a save that fails is
 * tried again (on the next change, on "Done", and by itself after a moment
 * if the server or network hiccupped).
 *
 * Call `start(layout)` when arranging begins, `change(layout)` on every
 * change, and `finish()` on "Done": it sends anything unsaved and resolves
 * true once the server has it all (the page's cached layout is updated
 * then), or false if that last save failed.
 */
export function useLayoutAutosave() {
  const queryClient = useQueryClient()
  const [state, setState] = useState<SaveState>(IDLE)
  // One saver for the page's lifetime; it keeps its bookkeeping outside
  // React, since timers and requests don't need to re-render anything.
  const [saver] = useState(() => createLayoutSaver(setState))

  // Pause the saver while the page is gone (no stray timers or requests
  // after logging out, say). Written as an on/off pair because React's
  // StrictMode unmounts and remounts every component once in development.
  useEffect(() => {
    saver.setMounted(true)
    return () => saver.setMounted(false)
  }, [saver])

  // While a change isn't saved yet, closing the tab asks first.
  const unsaved = state.status === 'saving' || state.status === 'error'
  useEffect(() => {
    if (!unsaved) return
    const warn = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [unsaved])

  const finish = useCallback(async () => {
    const layout = await saver.finish()
    if (layout) setCachedLayout(queryClient, layout)
    return layout !== null
  }, [saver, queryClient])

  return { state, start: saver.start, change: saver.change, finish }
}

/**
 * The saving logic itself, as plain functions sharing a few variables (a
 * closure), so it behaves the same however often React re-renders.
 * `onState` is told every time the save state changes.
 */
function createLayoutSaver(onState: (state: SaveState) => void) {
  let status: SaveState['status'] = 'idle'
  let mounted = false
  // The newest layout from the editor; null when not arranging.
  let latest: Layout | null = null
  // What the server has: its key (for comparing) and its own copy.
  let savedKey = ''
  let saved: Layout | null = null
  let timer: number | undefined
  // The save on its way, if any; resolves to whether it worked.
  let inFlight: Promise<boolean> | null = null

  function report(state: SaveState) {
    status = state.status
    onState(state)
  }

  function schedule(delay: number) {
    window.clearTimeout(timer)
    timer = window.setTimeout(() => void flush(), delay)
  }

  /** Saves the newest layout unless the server already has it. Resolves to whether it has now. */
  async function flush(): Promise<boolean> {
    window.clearTimeout(timer)
    // One save at a time: wait for the one on its way (it may carry an
    // older layout), then look again.
    while (inFlight) await inFlight
    if (!latest || !mounted) return false
    const layout = latest
    const key = layoutKey(layout)
    if (key === savedKey) {
      // Nothing new to send (say, a change was undone): say it's saved,
      // which also clears an earlier failure.
      if (status !== 'idle') report({ status: 'saved', error: null, retrying: false })
      return true
    }

    inFlight = send(layout, key)
    try {
      return await inFlight
    } finally {
      inFlight = null
    }
  }

  async function send(layout: Layout, key: string): Promise<boolean> {
    report({ status: 'saving', error: null, retrying: false })
    try {
      saved = await saveLayout(layout)
      savedKey = key
      // If something changed while this was on its way, its own timer
      // saves that next, and the status stays "Saving…" until then.
      if (latest && layoutKey(latest) === key) report({ status: 'saved', error: null, retrying: false })
      return true
    } catch (err) {
      const error =
        err instanceof ApiError ? err : new ApiError(0, 'Your layout didn’t save.', 'Try again in a moment.')
      // Worth trying again by itself only if the server or network hiccupped;
      // a refusal (say, a 400) needs a different layout, so it waits for the
      // next change or "Done".
      const retrying = error.status === 0 || error.status >= 500
      if (retrying && mounted && latest) schedule(RETRY_DELAY_MS)
      report({ status: 'error', error, retrying })
      return false
    }
  }

  return {
    setMounted(value: boolean) {
      mounted = value
      if (!value) window.clearTimeout(timer)
    },

    /** Begins arranging from this layout, which the server already has. */
    start(layout: Layout) {
      window.clearTimeout(timer)
      latest = layout
      saved = layout
      savedKey = layoutKey(layout)
      report(IDLE)
    },

    /** Notes the editor's newest layout and saves it after a short pause. */
    change(layout: Layout) {
      if (!latest) return
      latest = layout
      // Always (re)start the timer, even if this matches what the server
      // has: a save of something else may be on its way and need following.
      schedule(SAVE_DELAY_MS)
      if (layoutKey(layout) !== savedKey) report({ status: 'saving', error: null, retrying: false })
    },

    /**
     * Ends arranging once everything is saved. Resolves with the layout as
     * the server stored it, or null if the last save failed (arranging
     * carries on, and saving is retried).
     */
    async finish(): Promise<Layout | null> {
      const ok = await flush()
      if (!ok || !saved) return null
      latest = null
      return saved
    },
  }
}
