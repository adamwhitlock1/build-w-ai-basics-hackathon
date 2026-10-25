import { ActionBar, useGetPuck } from '@puckeditor/core'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { Span } from '../api/client'
import { useDeleteWidget } from '../api/hooks'
import { TrashIcon } from '../widgets/icons'
import { SPANS } from './layout'
import { useKinlightPuck } from './useKinlightPuck'

interface BlockActionBarProps {
  /** The card type's label from the config, e.g. "Articles". */
  label?: string
  /** Puck's default actions (duplicate, delete); we offer our own instead. */
  children: ReactNode
  /** Puck's "select parent" action, for nested cards; ours aren't nested. */
  parentAction: ReactNode
}

/**
 * The small bar above the selected card in the Arrange editor (it replaces
 * Puck's own through `overrides.actionBar`): width buttons from 1/4 to 4/4,
 * which do what the resize grip does without dragging, and "Remove" for
 * widgets you described.
 */
export function BlockActionBar({ label }: BlockActionBarProps) {
  const item = useKinlightPuck((s) => s.selectedItem)
  const selector = useKinlightPuck((s) => s.appState.ui.itemSelector)
  const dispatch = useKinlightPuck((s) => s.dispatch)

  if (!item || !selector?.zone) return <ActionBar label={label} />
  const zone = selector.zone
  const index = selector.index

  // Swaps in the same card with the new width.
  const setSpan = (span: Span) => {
    if (span === item.props.span) return
    dispatch({
      type: 'replace',
      destinationZone: zone,
      destinationIndex: index,
      data: { ...item, props: { ...item.props, span } },
    })
  }

  return (
    <ActionBar label={label}>
      <ActionBar.Group>
        {SPANS.map((span) => (
          <ActionBar.Action
            key={span}
            label={`Make it ${span}/4 wide`}
            active={span === item.props.span}
            onClick={() => setSpan(span)}
          >
            <span className="px-0.5 text-xs font-semibold tabular-nums">{span}/4</span>
          </ActionBar.Action>
        ))}
      </ActionBar.Group>
      {item.type === 'Custom' && (
        // Keyed by card, so a question left open on one card doesn't carry
        // over when another is selected.
        <RemoveWidget key={item.props.id} cardId={item.props.id} widgetId={item.props.widgetId} />
      )}
    </ActionBar>
  )
}

interface RemoveWidgetProps {
  /** The card's id in the layout. */
  cardId: string
  /** The saved widget it shows. */
  widgetId: number
}

/**
 * "Remove" for a widget you described. Every saved widget has a card on your
 * page (the server puts back any that's missing), so removing the card means
 * deleting the widget itself, for good. It asks first, deletes the widget on
 * the server, and only then takes the card out of the editor, which saves the
 * layout like any other change.
 */
function RemoveWidget({ cardId, widgetId }: RemoveWidgetProps) {
  // Reads the editor's state when called. Unlike the hooks above, it still
  // works after this bar has gone (say, another card was selected while the
  // delete was on its way), as long as the editor is open.
  const getPuck = useGetPuck()
  const deleteWidget = useDeleteWidget()
  const [confirming, setConfirming] = useState(false)
  const keepRef = useRef<HTMLButtonElement>(null)

  // When the question appears, start on the safe answer.
  useEffect(() => {
    if (confirming) keepRef.current?.focus()
  }, [confirming])

  async function remove() {
    try {
      await deleteWidget.mutateAsync(widgetId)
    } catch {
      return // the bar says it didn't work; the card stays
    }
    // Look the card up now: it may have moved while the request was out.
    const puck = getPuck()
    const selector = puck.getSelectorForId(cardId)
    if (!selector) return
    const wasSelected = puck.selectedItem?.props.id === cardId
    puck.dispatch({ type: 'remove', zone: selector.zone, index: selector.index })
    // Don't leave the selection on whichever card slides into its place.
    if (wasSelected) puck.dispatch({ type: 'setUi', ui: { itemSelector: null } })
  }

  if (deleteWidget.isPending) {
    return (
      <ActionBar.Group>
        <ActionBar.Label label="Deleting…" />
      </ActionBar.Group>
    )
  }

  if (!confirming) {
    return (
      <ActionBar.Group>
        <ActionBar.Action label="Remove this widget" onClick={() => setConfirming(true)}>
          <span className="flex items-center gap-1 text-xs font-semibold">
            <TrashIcon className="size-3.5" />
            Remove
          </span>
        </ActionBar.Action>
      </ActionBar.Group>
    )
  }

  return (
    <ActionBar.Group>
      <ActionBar.Label label={deleteWidget.isError ? 'Couldn’t delete it. Try again?' : 'Delete this widget for good?'} />
      <ActionBar.Action label="Yes, delete this widget" onClick={() => void remove()}>
        <span className="text-xs font-semibold">Delete</span>
      </ActionBar.Action>
      <ActionBar.Action
        ref={keepRef}
        label="No, keep this widget"
        onClick={() => {
          setConfirming(false)
          deleteWidget.reset() // forget a failed attempt
        }}
      >
        <span className="text-xs font-semibold">Keep</span>
      </ActionBar.Action>
    </ActionBar.Group>
  )
}
