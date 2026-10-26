// The Arrange editor. Only loaded once someone taps "Arrange" (see the lazy
// import in routes/_authed/index.tsx), so the normal page never downloads
// Puck. Puck's stylesheet comes with it; the "no-external" build skips the
// web font Puck's own toolbars use, which we don't show.
import '@puckeditor/core/no-external.css'

import { Puck } from '@puckeditor/core'
import { useCallback } from 'react'
import type { Layout } from '../api/client'
import { BlockActionBar } from './BlockActionBar'
import { config } from './config'
import { fromPuckData } from './layout'
import type { KinlightData } from './types'
import type { SaveState } from './useLayoutAutosave'
import { Breathing } from '../components/Breathing'

interface ArrangeCanvasProps {
  /**
   * The layout to start from. Puck reads it once, when the editor opens;
   * after that the editor owns the layout until "Done".
   */
  initial: Layout
  /** Called with the new layout after every change (a move, a resize, a removal). */
  onChange: (layout: Layout) => void
  /** How saving is going, for the toolbar. */
  save: SaveState
}

/**
 * Your page in Arrange mode: the same cards, now draggable and resizable.
 *
 * Puck is used in "composition" mode: instead of its full editor (header,
 * side panels, block library), we render only its preview of the page plus
 * our own small toolbar. Selecting, dragging, hover outlines and the action
 * bar above the selected card still work. The preview renders into the page
 * itself rather than an iframe, so our styles apply as usual.
 */
export function ArrangeCanvas({ initial, onChange, save }: ArrangeCanvasProps) {
  const handleChange = useCallback((data: KinlightData) => onChange(fromPuckData(data)), [onChange])

  return (
    // .arrange-canvas (styles/index.css) gives Puck's outlines our colours.
    <div className="arrange-canvas">
      <Puck
        config={config}
        data={initial}
        onChange={handleChange}
        iframe={{ enabled: false }}
        overrides={{ actionBar: BlockActionBar }}
        // Cards come from the "+" and the server, not from Puck.
        permissions={{ duplicate: false, insert: false }}
      >
        <ArrangeToolbar save={save} />
        <Puck.Preview />
      </Puck>
    </div>
  )
}

/**
 * The strip above the cards: how arranging works, and whether it's saved.
 * It stays pinned to the top as you scroll, and the space below it leaves
 * room for the action bar Puck shows above a selected card in the first row.
 */
function ArrangeToolbar({ save }: { save: SaveState }) {
  return (
    <div className="lantern-glass sticky top-3 lg:top-[calc(var(--demo-notice-height,0px)+0.75rem)] z-20 mb-14 rounded-2xl border border-line px-4 py-3 shadow-card sm:px-5">
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-1">
        <p className="text-sm text-ink-soft">
          Drag cards to reorder <span aria-hidden="true">·</span> drag a card’s right edge to resize
        </p>
        <SaveStatus save={save} />
      </div>
      {save.status === 'error' && save.error && (
        <p className="mt-1 text-sm text-ink-soft">{[save.error.message, save.error.hint].filter(Boolean).join(' ')}</p>
      )}
    </div>
  )
}

function SaveStatus({ save }: { save: SaveState }) {
  let content = <span className="text-ink-muted">Changes save as you go</span>
  if (save.status === 'saving') {
    content = (
      <span className="flex items-center gap-2 text-ink-soft">
        <Breathing className="size-4" />
        Saving…
      </span>
    )
  } else if (save.status === 'saved') {
    content = <span className="text-ink-soft">Saved</span>
  } else if (save.status === 'error') {
    content = (
      <span className="font-medium text-danger">
        {save.retrying ? 'Couldn’t save — retrying' : 'Couldn’t save'}
      </span>
    )
  }
  return (
    // Announced politely, so screen-reader users hear "Saved" without losing their place.
    <p role="status" className="text-sm">
      {content}
    </p>
  )
}
