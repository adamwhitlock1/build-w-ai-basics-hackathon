import { useEffect, useRef } from 'react'
import type { SavedWidget } from '../../api/client'
import { usePageHomepage, usePageOwner, useKidSafeFor, useRefreshCustomWidget } from '../../api/hooks'
import { LoadError } from '../../widgets/CuratedWidgetCard'
import { RefreshButton } from '../../widgets/RefreshButton'
import { WidgetCard } from '../../widgets/WidgetCard'

interface CustomBlockProps {
  /** Which saved widget to show (SavedWidget.id). */
  widgetId: number
  /** Just built: amber border and a soft fade-in (see WidgetCard). */
  fresh?: boolean
}

/**
 * Stale widgets already sent off for a background refresh, as
 * "<id>@<updatedAt>", so each is refreshed once per visit: a refresh that
 * fails leaves the same stale widget behind, and mustn't retry in a loop.
 */
const staleRefreshed = new Set<string>()

/**
 * A widget someone described, as placed on their page and in the Arrange
 * editor. Its content came with the page (Homepage.customWidgets), so it's
 * read from that cached copy rather than fetched on its own: from your page,
 * or from the family member's a parent is looking at. Only its owner can
 * refresh it, so on a family member's page it's shown as it is.
 */
export function CustomBlock({ widgetId, fresh = false }: CustomBlockProps) {
  const page = usePageHomepage()
  const { isYou } = usePageOwner()
  const widget = page.customWidgets.find((w) => w.id === widgetId)
  const kidSafeFor = useKidSafeFor(widget)

  if (!widget) {
    // The layout mentions a widget this page doesn't have (the server tidies
    // that up on the next load), so say so quietly rather than show nothing.
    return (
      <div className="flex min-h-40 items-center justify-center rounded-3xl border-2 border-dashed border-line p-6 text-center text-sm text-ink-muted">
        This card is gone
      </div>
    )
  }
  // Someone else's page is a parent looking at a family member's, the only
  // place the kid-safe marker shows (see useKidSafeFor).
  if (!isYou) return <WidgetCard widget={widget} fresh={fresh} kidSafeFor={kidSafeFor} />
  return <OwnCustomWidget widget={widget} fresh={fresh} />
}

/**
 * One of your own described widgets: a refresh button, a quiet background
 * refresh when the server says it's stale, and an inline error that keeps
 * the current items when a refresh fails.
 */
function OwnCustomWidget({ widget, fresh }: { widget: SavedWidget; fresh: boolean }) {
  const refresh = useRefreshCustomWidget(widget.id)
  const refreshRef = useRef<HTMLButtonElement>(null)
  // mutate is stable across renders, so the effect can depend on it.
  const { mutate } = refresh

  const staleSince = widget.stale ? widget.updatedAt : undefined
  useEffect(() => {
    if (!staleSince) return
    const once = `${widget.id}@${staleSince}`
    if (staleRefreshed.has(once)) return
    staleRefreshed.add(once)
    mutate()
  }, [widget.id, staleSince, mutate])

  const failed = refresh.isError && !refresh.isPending ? refresh.error : null

  function retry() {
    // The "Try again" button is about to disappear; keep focus in the card.
    refreshRef.current?.focus()
    mutate()
  }

  return (
    <WidgetCard
      widget={widget}
      fresh={fresh}
      notice={failed && <LoadError noun={widget.title} error={failed} stale={widget.items.length > 0} onRetry={retry} />}
      headerAction={
        <>
          <RefreshButton
            ref={refreshRef}
            label={`Refresh ${widget.title}`}
            refreshing={refresh.isPending}
            onRefresh={() => mutate()}
          />
          {/* Always mounted, so screen readers hear it when the text changes. */}
          <span role="status" className="sr-only">
            {refresh.isPending ? `Refreshing ${widget.title}…` : ''}
          </span>
        </>
      }
    />
  )
}
