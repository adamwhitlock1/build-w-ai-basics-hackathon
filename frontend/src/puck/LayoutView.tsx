import { Fragment, type ReactNode } from 'react'
import type { Layout, LayoutBlock, Span } from '../api/client'
import { ArticlesBlock } from './blocks/Articles'
import { BriefingBlock } from './blocks/Briefing'
import { CustomBlock } from './blocks/Custom'
import { LocalEventsBlock } from './blocks/LocalEvents'
import { TodayBlock } from './blocks/Today'
import { columnsLeftInLastRow } from './layout'

interface LayoutViewProps {
  /** The cards to show, in order, with their widths. */
  layout: Layout
  /** The widget that was just built, if any: its card gets the "fresh" look. */
  freshWidgetId?: number | null
  /** Shown above the cards, across the whole grid (the "Add a card" prompt). */
  top?: ReactNode
  /**
   * Shown across the whole grid just below the morning briefing, wherever
   * the briefing is (the chore cards), or at the top if there's no briefing.
   */
  belowBriefing?: ReactNode
  /** The "+" tile, placed after the cards. Without it, no empty spots are shown. */
  addTile?: ReactNode
}

/**
 * Grid classes for each width. One column on a phone; two on a tablet, where
 * a card of up to half the page takes one; the full four from `lg` up.
 * (Spelled out in full so Tailwind can find every class.)
 */
const SPAN_CLASSES: Record<Span, string> = {
  1: 'md:col-span-1 lg:col-span-1',
  2: 'md:col-span-1 lg:col-span-2',
  3: 'md:col-span-2 lg:col-span-3',
  4: 'md:col-span-2 lg:col-span-4',
}

/**
 * Your page as you normally see it: the saved layout drawn as a plain grid
 * (not Puck), so links and refresh buttons inside the cards just work. After
 * the cards come "This spot is empty" cards filling out the last row, with
 * the "+" tile in its last spot (or on a row of its own if the cards fill
 * theirs).
 */
export function LayoutView({ layout, freshWidgetId = null, top, belowBriefing, addTile }: LayoutViewProps) {
  const blocks = layout.root.props.blocks
  // The "+" takes one of the spots left in the last row; empty cards fill
  // the rest. On first use (no widgets of your own yet) the curated cards
  // fill row one, so the "+" starts a new row with two empty spots beside it,
  // as the PRD's first-use page describes. Only on wide screens, where the
  // grid has its four columns.
  const left = columnsLeftInLastRow(blocks)
  const firstUse = !blocks.some((block) => block.type === 'Custom')
  const emptySpots = !addTile ? 0 : left > 0 ? left - 1 : firstUse ? 2 : 0
  const hasBriefing = blocks.some((block) => block.type === 'Briefing')
  const below = belowBriefing && <div className="col-span-full">{belowBriefing}</div>

  return (
    <div className="grid grid-cols-1 items-start gap-5 sm:gap-6 md:grid-cols-2 lg:grid-cols-4">
      {top && <div className="col-span-full">{top}</div>}
      {!hasBriefing && below}

      {blocks.map((block) => (
        <Fragment key={block.props.id}>
          <div className={`min-w-0 ${SPAN_CLASSES[block.props.span] ?? SPAN_CLASSES[2]}`}>
            <BlockContent block={block} freshWidgetId={freshWidgetId} />
          </div>
          {block.type === 'Briefing' && below}
        </Fragment>
      ))}

      {Array.from({ length: emptySpots }, (_, i) => (
        <div
          key={`empty-${i}`}
          className="hidden min-h-40 items-center justify-center rounded-3xl border-2 border-dashed border-line p-6 text-center text-sm text-ink-muted lg:flex"
        >
          An empty spot. Add a card to fill it.
        </div>
      ))}

      {addTile}
    </div>
  )
}

/** The card a layout block stands for. */
function BlockContent({ block, freshWidgetId }: { block: LayoutBlock; freshWidgetId: number | null }) {
  switch (block.type) {
    case 'Briefing':
      return <BriefingBlock />
    case 'Today':
      return <TodayBlock />
    case 'Articles':
      return <ArticlesBlock />
    case 'LocalEvents':
      return <LocalEventsBlock />
    case 'Custom':
      return <CustomBlock widgetId={block.props.widgetId} fresh={block.props.widgetId === freshWidgetId} />
    default:
      // The server only stores these five kinds, but this runs on live
      // data: skip anything else rather than break the page.
      return null
  }
}
