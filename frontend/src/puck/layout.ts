// Small, Puck-free helpers for page layouts, shared by the normal page, the
// Arrange editor and the API hooks. (Only types come from Puck here, so the
// normal page never loads the editor's code.)

import type { Homepage, Layout, LayoutBlock, SavedWidget, Span } from '../api/client'
import type { PorchlightData } from './types'

/** The page grid's columns on a wide screen; spans count these. */
export const COLUMNS = 4

/** Every width a card can take, narrowest first. */
export const SPANS: readonly Span[] = [1, 2, 3, 4]

/** How wide a new card starts: half the page. */
export const DEFAULT_SPAN: Span = 2

/** The page-unique id of a saved widget's card, e.g. "custom-7". */
export function customBlockId(widgetId: number) {
  return `custom-${widgetId}`
}

/** A half-width card for a saved widget. */
export function customBlock(widgetId: number): LayoutBlock {
  return { type: 'Custom', props: { id: customBlockId(widgetId), span: DEFAULT_SPAN, widgetId } }
}

/** A layout with these cards, in this order. */
export function layoutWith(blocks: LayoutBlock[]): Layout {
  return { root: { props: { blocks } }, content: [], zones: {} }
}

/**
 * The page anyone starts with (the server builds the same one): Today across
 * the top, Articles and Local Events side by side, then each saved widget,
 * oldest first.
 */
export function defaultLayout(widgets: SavedWidget[]): Layout {
  return layoutWith([
    { type: 'Today', props: { id: 'today', span: 4 } },
    { type: 'Articles', props: { id: 'articles', span: 2 } },
    { type: 'LocalEvents', props: { id: 'events', span: 2 } },
    ...widgets.map((w) => customBlock(w.id)),
  ])
}

/**
 * The page's layout. The server always sends one, but if something odd comes
 * back (an older backend, say) the page still shows every card rather than
 * crashing.
 */
export function layoutOf(page: Homepage): Layout {
  const blocks: unknown = page.layout?.root?.props?.blocks
  return Array.isArray(blocks) ? page.layout : defaultLayout(page.customWidgets)
}

/**
 * The layout with a card for this widget at the end, the way the server adds
 * one when a widget is built. Unchanged if the widget already has a card.
 */
export function withCustomBlock(layout: Layout, widgetId: number): Layout {
  const blocks = layout.root.props.blocks
  if (blocks.some((b) => b.type === 'Custom' && b.props.widgetId === widgetId)) return layout
  return layoutWith([...blocks, customBlock(widgetId)])
}

/** The layout without this widget's card, the way the server drops it when the widget is deleted. */
export function withoutCustomBlock(layout: Layout, widgetId: number): Layout {
  return layoutWith(layout.root.props.blocks.filter((b) => !(b.type === 'Custom' && b.props.widgetId === widgetId)))
}

/**
 * Turns the Arrange editor's data back into a plain layout. Puck may add
 * bookkeeping of its own to the data; only the fields the server stores are
 * kept, so comparing two layouts compares what matters.
 */
export function fromPuckData(data: PorchlightData): Layout {
  const blocks = data.root.props?.blocks ?? []
  return layoutWith(
    blocks.map((block): LayoutBlock => {
      const { id, span } = block.props
      return block.type === 'Custom'
        ? { type: 'Custom', props: { id, span, widgetId: block.props.widgetId } }
        : { type: block.type, props: { id, span } }
    }),
  )
}

/** A string that's equal for two layouts exactly when they look the same. */
export function layoutKey(layout: Layout): string {
  return JSON.stringify(layout.root.props.blocks)
}

/**
 * How many columns are left over in the grid's last row, once every card is
 * placed the way CSS grid places them: left to right, and a card too wide for
 * the space left in a row starts the next one. 0 when the last row is full.
 */
export function columnsLeftInLastRow(blocks: LayoutBlock[], columns = COLUMNS): number {
  let used = 0
  for (const block of blocks) {
    const span = Math.min(block.props.span, columns)
    if (used + span > columns) used = 0
    used = (used + span) % columns
  }
  return used === 0 ? 0 : columns - used
}
