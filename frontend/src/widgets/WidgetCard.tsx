import { useEffect, useRef } from 'react'
import type { SourceName, Widget } from '../api/client'
import { WidgetItems } from './renderers/WidgetItems'

interface WidgetCardProps {
  widget: Widget
  /** Just arrived: amber border plus a soft fade-and-slide in. */
  fresh?: boolean
}

const SOURCE_LABELS: Record<SourceName, string> = {
  news_search: 'a news search',
  events_search: 'an events search',
  web_search: 'a web search',
}

/**
 * A widget on the page. Purely presentational: it renders a Widget and
 * owns no data fetching, so it can later be wrapped as a Puck block.
 */
export function WidgetCard({ widget, fresh = false }: WidgetCardProps) {
  const items = widget.items ?? []
  const sourceLabel = SOURCE_LABELS[widget.source] ?? 'a search'
  const ref = useRef<HTMLElement>(null)

  // A widget that just arrived takes focus, which also scrolls it into view
  // and lets screen-reader users land on it. The amber border marks it.
  useEffect(() => {
    if (fresh) ref.current?.focus()
  }, [fresh])

  return (
    <article
      ref={ref}
      tabIndex={-1}
      // @container lets renderers adapt to the card's width, not the window's.
      className={`@container flex flex-col rounded-3xl border bg-surface p-6 shadow-card transition-[border-color,box-shadow] duration-1000 focus:outline-none sm:p-7 ${
        fresh ? 'border-accent ring-4 ring-accent/15 motion-safe:animate-widget-in' : 'border-line'
      }`}
    >
      <h2 className="text-2xl leading-tight text-ink">{widget.title}</h2>

      <div className="mt-5 flex-1">
        {items.length === 0 ? (
          // Widgets can't be refreshed yet (their content is saved as it was
          // built), so point to what works now: describing it differently.
          <p className="rounded-2xl bg-bg px-4 py-6 text-center text-sm text-ink-muted">
            Nothing turned up for that just now — try describing it a little differently.
          </p>
        ) : (
          <WidgetItems archetype={widget.archetype} items={items} />
        )}
      </div>

      {widget.query && (
        <p className="mt-6 truncate border-t border-line pt-4 text-xs text-ink-muted" title={widget.query}>
          From {sourceLabel} for “{widget.query}”
        </p>
      )}
    </article>
  )
}
