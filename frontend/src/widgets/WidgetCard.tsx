import { useEffect, useRef, type ReactNode } from 'react'
import type { Freshness, SourceName, Widget } from '../api/client'
import { ShieldIcon } from './icons'
import { WidgetItems } from './renderers/WidgetItems'
import { formatUpdatedAgo, useNow } from './updatedAgo'
import { Breathing } from '../components/Breathing'

interface WidgetCardProps {
  /**
   * What to show. It can be left out while `loading`, or when a `notice`
   * explains why there's nothing yet. With an `updatedAt`, the footer says
   * how long ago its items were updated.
   */
  widget?: Widget & Partial<Freshness>
  /**
   * The heading, e.g. "Articles for you". Left out, it's the widget's own
   * title. (The curated cards always set it, so on a family member's page
   * theirs can speak about them.)
   */
  title?: string
  /** Just arrived: amber border plus a soft fade-and-slide in. */
  fresh?: boolean
  /** A small control beside the heading, such as a refresh button. */
  headerAction?: ReactNode
  /** Show the "Kid-safe" marker by the heading (see useShowKidSafe). */
  showKidSafe?: boolean
  /** What to say when the widget has no items. */
  emptyMessage?: string
  /** The first load is under way: soft placeholder rows instead of items. */
  loading?: boolean
  /** Warm words above the placeholder rows, e.g. "Finding articles for you…". */
  loadingMessage?: string
  /**
   * Shown above the items: an error with a "Try again" button, say, or what's
   * missing before anything can show. It replaces the empty message.
   */
  notice?: ReactNode
  /**
   * The card's own body, for a card that isn't a list of widget items (the
   * Today card's events). It replaces the items and the empty message, and
   * shows below `notice`.
   */
  children?: ReactNode
  /** The footer's left side, for a card with `children`, e.g. "From your Google Calendar". */
  footerNote?: ReactNode
  /** When a card with `children` was last updated (RFC 3339), for "Updated … ago". */
  updatedAt?: string
}

const SOURCE_LABELS: Record<SourceName, string> = {
  news_search: 'a news search',
  events_search: 'an events search',
  web_search: 'a web search',
  weather_forecast: 'a forecast',
}

// What a described widget says when its latest search found nothing (or
// only things that have already happened). Refreshing can turn up more.
const DEFAULT_EMPTY_MESSAGE = 'Nothing turned up just now — try refreshing later.'

/**
 * A widget on the page. Purely presentational: it renders a Widget (or its
 * loading and error states) and owns no data fetching, so it can later be
 * wrapped as a Puck block.
 */
export function WidgetCard({
  widget,
  title,
  fresh = false,
  headerAction,
  showKidSafe = false,
  emptyMessage = DEFAULT_EMPTY_MESSAGE,
  loading = false,
  loadingMessage = 'Loading…',
  notice,
  children,
  footerNote,
  updatedAt = widget?.updatedAt,
}: WidgetCardProps) {
  const items = widget?.items ?? []
  const sourceLabel = widget ? (SOURCE_LABELS[widget.source] ?? 'a search') : ''
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
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-1.5">
          <h2 className="min-w-0 break-words text-2xl leading-tight text-ink">{title || widget?.title}</h2>
          {showKidSafe && <KidSafeTag />}
        </div>
        {headerAction && <div className="-mt-1 -mr-2 shrink-0">{headerAction}</div>}
      </div>

      <div className="mt-5 flex-1">
        {loading ? (
          <LoadingRows message={loadingMessage} />
        ) : (
          <>
            {notice}
            {children !== undefined ? (
              children && <div className={notice ? 'mt-4' : undefined}>{children}</div>
            ) : widget && items.length > 0 ? (
              <div className={notice ? 'mt-4' : undefined}>
                <WidgetItems archetype={widget.archetype} items={items} />
              </div>
            ) : (
              !notice && (
                <p className="rounded-2xl bg-bg px-4 py-6 text-center text-sm text-ink-muted">{emptyMessage}</p>
              )
            )}
          </>
        )}
      </div>

      {(widget?.query || footerNote) && !loading && (
        <div className="mt-6 flex items-baseline gap-3 border-t border-line pt-4 text-xs text-ink-muted">
          {footerNote ? (
            <p className="min-w-0 flex-1 truncate">{footerNote}</p>
          ) : widget?.source === 'weather_forecast' ? (
            <WeatherCredit place={widget.query} />
          ) : (
            <p className="min-w-0 flex-1 truncate" title={widget?.query}>
              From {sourceLabel} for “{widget?.query}”
            </p>
          )}
          {updatedAt && <UpdatedAgo updatedAt={updatedAt} />}
        </div>
      )}
    </article>
  )
}

/**
 * The footer of a weather card: the place it's for, and the credit
 * Open-Meteo's CC BY 4.0 license asks for, linked to their site. The credit
 * gets its own line so a narrow card never truncates it; only the place can
 * be cut short.
 */
function WeatherCredit({ place }: { place: string }) {
  return (
    <div className="min-w-0 flex-1">
      <p className="truncate" title={place}>
        Forecast for “{place}”
      </p>
      <p className="mt-0.5">
        Weather data by{' '}
        <a
          href="https://open-meteo.com/"
          target="_blank"
          rel="noopener noreferrer"
          className="whitespace-nowrap rounded-sm underline decoration-line underline-offset-2 hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          Open-Meteo.com
        </a>
      </p>
    </div>
  )
}

/** Tells a parent this card was searched and chosen for their kid. */
function KidSafeTag() {
  return (
    <span
      title="Searched with strict safe-search and chosen for under-18s"
      className="inline-flex shrink-0 items-center gap-1 rounded-full bg-bg px-2 py-0.5 text-xs font-medium text-ink-soft ring-1 ring-line"
    >
      <ShieldIcon className="size-3.5" />
      Kid-safe
    </span>
  )
}

/**
 * "Updated 2h ago", kept current while the page stays open. It's its own
 * component so the minute tick re-renders only this label, not the card.
 */
function UpdatedAgo({ updatedAt }: { updatedAt: string }) {
  const now = useNow(60_000)
  const ago = formatUpdatedAgo(updatedAt, now)
  if (!ago) return null
  return (
    <time dateTime={updatedAt} title={new Date(updatedAt).toLocaleString()} className="shrink-0">
      Updated {ago}
    </time>
  )
}

/** Widths for the placeholder rows, so they look like text rather than bars. */
const PLACEHOLDER_WIDTHS = [
  ['w-3/4', 'w-1/2'],
  ['w-2/3', 'w-2/5'],
  ['w-4/5', 'w-1/3'],
]

/** Soft, gently pulsing placeholder rows while a widget's first load runs. */
function LoadingRows({ message }: { message: string }) {
  return (
    <div>
      <p role="status" className="flex items-center gap-2.5 text-sm text-ink-muted">
        <Breathing className="size-5" />
        {message}
      </p>
      {/* Still placeholders, so the card keeps its size while it waits. */}
      <ul aria-hidden="true" className="mt-4 space-y-3">
        {PLACEHOLDER_WIDTHS.map(([first, second], i) => (
          <li key={i} className="rounded-2xl border border-line bg-bg/60 p-3.5">
            <div className={`h-3.5 rounded-full bg-line ${first}`} />
            <div className={`mt-2.5 h-3 rounded-full bg-line/60 ${second}`} />
          </li>
        ))}
      </ul>
    </div>
  )
}
