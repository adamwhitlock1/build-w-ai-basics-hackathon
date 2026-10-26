import { Fragment, useLayoutEffect, useRef, type ReactNode } from 'react'
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
  const below = belowBriefing && <GridItem className="col-span-full">{belowBriefing}</GridItem>

  return (
    // Tiny rows and no row gap: each card claims rows for its own height
    // plus a gap (see GridItem), so a short card doesn't leave a hole beside
    // a tall one. Cards still go left to right in the page's order.
    <div className="grid auto-rows-[4px] grid-cols-1 items-start gap-x-5 sm:gap-x-6 md:grid-cols-2 lg:grid-cols-4">
      {top && <GridItem className="col-span-full">{top}</GridItem>}
      {!hasBriefing && below}

      {blocks.map((block) => (
        <Fragment key={block.props.id}>
          <GridItem className={`min-w-0 ${SPAN_CLASSES[block.props.span] ?? SPAN_CLASSES[2]}`}>
            <BlockContent block={block} freshWidgetId={freshWidgetId} />
          </GridItem>
          {block.type === 'Briefing' && below}
        </Fragment>
      ))}

      {Array.from({ length: emptySpots }, (_, i) => (
        <GridItem
          key={`empty-${i}`}
          className="hidden min-h-40 items-center justify-center rounded-3xl border-2 border-dashed border-line p-6 text-center text-sm text-ink-muted lg:flex"
        >
          An empty spot. Add a card to fill it.
        </GridItem>
      ))}

      {addTile && <GridItem className="grid">{addTile}</GridItem>}
    </div>
  )
}

/** The height of the grid's rows, in pixels: each card spans enough of them. */
const ROW = 4

/**
 * One thing in the page grid, spanning as many of its tiny rows as its
 * height needs, plus one column gap's worth below it, so the grid packs
 * cards like bricks. It measures itself whenever it or the grid changes
 * size (a card loading, "Done" opening, a new screen width). offsetHeight
 * ignores transforms, so a card's arrival animation doesn't count. Hidden
 * things (an empty spot on a narrow screen) measure 0 and claim nothing.
 */
function GridItem({ className = '', children }: { className?: string; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    const item = ref.current
    const grid = item?.parentElement
    if (!item || !grid) return
    const fit = () => {
      const height = item.offsetHeight
      const gap = parseFloat(getComputedStyle(grid).columnGap) || 0
      item.style.gridRowEnd = height ? `span ${Math.ceil((height + gap) / ROW)}` : ''
    }
    fit()
    const observer = new ResizeObserver(fit)
    observer.observe(item)
    observer.observe(grid)
    return () => observer.disconnect()
  }, [])
  return (
    <div ref={ref} className={className}>
      {children}
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
