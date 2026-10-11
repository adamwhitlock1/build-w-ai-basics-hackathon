import type { PuckContext } from '@puckeditor/core'
import { useState, type ReactNode } from 'react'
import type { Span } from '../api/client'
import { ResizeHandle } from './ResizeHandle'

interface ArrangeFrameProps {
  /** The card's id in the layout. */
  id: string
  /** How many of the four columns the card spans. */
  span: Span
  /** Puck's drag handle for the card (from the `puck` prop of a render function). */
  dragRef: PuckContext['dragRef']
  /** True inside the editor (from the same `puck` prop). */
  editing: boolean
  /** The card itself. */
  children: ReactNode
}

/**
 * Wraps a card in the Arrange editor. Its outer element is the grid item and
 * Puck's drag handle (the whole card can be dragged), and it carries the
 * resize grip on the right edge.
 *
 * The cards are registered with Puck as `inline`, so Puck adds no wrapper of
 * its own: this element must take Puck's `dragRef` and set the grid span
 * itself, or the spans and dragging break.
 */
export function ArrangeFrame({ id, span, dragRef, editing, children }: ArrangeFrameProps) {
  // During a resize, the width the card would snap to. After the drop it
  // stays until Puck re-renders the card with its new span, so the card
  // doesn't flicker back to the old width in between.
  const [preview, setPreview] = useState<Span | null>(null)
  // Then it's dropped, so a later change of width (say, from the width
  // buttons) shows as it should. This is React's pattern for adjusting state
  // when a prop changes: setting state during render makes React re-render
  // straight away, before anything reaches the screen.
  const [lastSpan, setLastSpan] = useState(span)
  if (span !== lastSpan) {
    setLastSpan(span)
    setPreview(null)
  }
  const shown = preview ?? span

  return (
    <div ref={dragRef} style={{ gridColumn: `span ${shown} / span ${shown}` }} className="relative min-w-0 rounded-3xl">
      {/* The card can't be used while arranging (Puck's overlay takes the
          clicks anyway); `inert` also keeps keyboard focus out of its links. */}
      <div ref={makeInert}>{children}</div>
      {editing && (
        <ResizeHandle id={id} span={span} onPreview={setPreview} />
      )}
    </div>
  )
}

/** React 18 doesn't know the `inert` attribute yet, so it's set directly on the element. */
function makeInert(el: HTMLDivElement | null) {
  el?.setAttribute('inert', '')
}
