import { registerOverlayPortal, useGetPuck } from '@puckeditor/core'
import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import type { Span } from '../api/client'
import { COLUMNS } from './layout'

interface ResizeHandleProps {
  /** The card's id in the layout. */
  id: string
  /** The card's saved width. */
  span: Span
  /**
   * Called while dragging with the width the card would snap to now, so the
   * card can show it live; null when the drag ends or is cancelled.
   */
  onPreview: (span: Span | null) => void
}

/**
 * The amber grip on a card's right edge in the Arrange editor. Drag it and the
 * card snaps between 1/4 and 4/4 of the page as you go; let go and the new
 * width is saved like any other change.
 *
 * Puck has no resizing of its own (only reordering), so this is ours. It only
 * mounts inside the editor, because it talks to Puck through its hooks.
 */
export function ResizeHandle({ id, span, onPreview }: ResizeHandleProps) {
  const getPuck = useGetPuck()
  const ref = useRef<HTMLDivElement>(null)
  // The width shown in the badge while dragging (null when not dragging).
  const [dragSpan, setDragSpan] = useState<Span | null>(null)
  // Removes the window listeners of a drag under way (see startResize).
  const endDragRef = useRef<(() => void) | null>(null)

  // Tell Puck this element is ours: pressing it mustn't start Puck's own
  // drag (which would move the card instead of resizing it).
  useEffect(() => registerOverlayPortal(ref.current, { disableDrag: true }), [])

  // If the card disappears mid-drag, stop listening.
  useEffect(() => () => endDragRef.current?.(), [])

  // A capture handler, because Puck stops this pointerdown from bubbling
  // (that's how registerOverlayPortal keeps its drag away), so a plain
  // onPointerDown would never run.
  function startResize(e: ReactPointerEvent<HTMLDivElement>) {
    if (e.button !== 0) return
    const card = ref.current?.parentElement
    const grid = card?.parentElement
    if (!card || !grid) return
    e.preventDefault()
    e.stopPropagation()

    // One column's width, from the grid's real size, so snapping matches
    // what's on screen at any window width.
    const gap = parseFloat(getComputedStyle(grid).columnGap) || 0
    const column = (grid.getBoundingClientRect().width - gap * (COLUMNS - 1)) / COLUMNS
    const startX = e.clientX
    const startWidth = card.getBoundingClientRect().width
    let next = span

    const onMove = (ev: PointerEvent) => {
      // Snap to the nearest whole number of columns (a card of n columns is
      // n columns plus the n - 1 gaps between them wide).
      const width = startWidth + (ev.clientX - startX)
      const snapped = Math.min(COLUMNS, Math.max(1, Math.round((width + gap) / (column + gap)))) as Span
      if (snapped === next) return
      next = snapped
      setDragSpan(next)
      onPreview(next)
    }
    const onUp = () => finish(true)
    const onCancel = () => finish(false)
    const onKeyDown = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape') return
      // Escape cancels the resize only; preventDefault tells the page not to
      // leave Arrange mode as well.
      ev.preventDefault()
      ev.stopPropagation()
      finish(false)
    }

    const endDrag = () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
      window.removeEventListener('pointercancel', onCancel)
      window.removeEventListener('keydown', onKeyDown, true)
      document.body.style.removeProperty('cursor')
      endDragRef.current = null
    }

    function finish(keep: boolean) {
      endDrag()
      setDragSpan(null)
      if (!keep || next === span) {
        onPreview(null)
        return
      }
      // Swap in the same card with its new width. The card keeps showing
      // the preview until Puck re-renders it with the new span.
      const puck = getPuck()
      const selector = puck.getSelectorForId(id)
      const item = puck.getItemById(id)
      if (!selector || !item) {
        onPreview(null)
        return
      }
      puck.dispatch({
        type: 'replace',
        destinationZone: selector.zone,
        destinationIndex: selector.index,
        data: { ...item, props: { ...item.props, span: next } },
      })
    }

    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    window.addEventListener('pointercancel', onCancel)
    // Capture phase, so this runs before the page's own Escape handler.
    window.addEventListener('keydown', onKeyDown, true)
    document.body.style.cursor = 'ew-resize'
    endDragRef.current = endDrag
    setDragSpan(span)
  }

  return (
    <div
      ref={ref}
      onPointerDownCapture={startResize}
      title="Drag to resize"
      // Purely a pointer control; the width buttons above a selected card do
      // the same job from the keyboard.
      aria-hidden="true"
      className="group absolute inset-y-0 -right-3 z-10 flex w-6 cursor-ew-resize touch-none items-center justify-center outline-none!"
    >
      <span
        className={`h-14 w-1.5 rounded-full shadow-sm transition-colors ${
          dragSpan ? 'bg-accent-strong' : 'bg-accent/70 group-hover:bg-accent'
        }`}
      />
      {dragSpan && (
        <span className="absolute right-full mr-2 whitespace-nowrap rounded-full bg-ink px-2.5 py-1 text-xs font-medium text-white shadow-card">
          {dragSpan}/4 wide
        </span>
      )}
    </div>
  )
}
