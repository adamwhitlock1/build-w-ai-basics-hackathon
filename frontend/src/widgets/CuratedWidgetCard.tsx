import type { UseQueryResult } from '@tanstack/react-query'
import { useRef } from 'react'
import { useOpenInterestsEditor, type ProfileField } from '../account/interestsEditorContext'
import type { ApiError, CuratedWidget, EmptyReason } from '../api/client'
import { useShowKidSafe } from '../api/hooks'
import { RefreshButton } from './RefreshButton'
import { WidgetCard } from './WidgetCard'

/** What a curated widget says when the search found nothing (PRD > States and Boundaries). */
const NOTHING_NEW = 'Nothing new today — try refreshing later'

/** What a curated widget says, and offers, when it's missing something it needs. */
export interface MissingInfo {
  /** Warm words about what's missing. */
  message: string
  /** Label of the button that opens the interests editor, e.g. "Add your city". */
  action: string
  /** The editor field that button puts the cursor in. */
  field: ProfileField
}

interface CuratedWidgetCardProps {
  /** useArticles() or useEvents(). */
  result: UseQueryResult<CuratedWidget, ApiError>
  /** Gets new picks: useRefreshCuratedWidget() for the same widget. */
  onRefresh: () => void
  /** The heading, e.g. "Articles for you" (or "Their articles" on a family member's page). */
  title: string
  /** What it shows, in the plural, for labels: "articles", "events". */
  noun: string
  /** Shown during the first load, e.g. "Finding articles for you…". */
  loadingMessage: string
  /** Copy for each emptyReason this widget can come back with. */
  missing: Partial<Record<Exclude<EmptyReason, ''>, MissingInfo>>
}

/**
 * What the Articles and Local Events widgets share: a WidgetCard with a
 * refresh button, placeholder rows during the first load, an inline error
 * with "Try again", and a nudge toward the interests editor when the server
 * had nothing to go on. Refreshing keeps the current items up until the new
 * ones arrive.
 */
export function CuratedWidgetCard({ result, onRefresh, title, noun, loadingMessage, missing }: CuratedWidgetCardProps) {
  const { data, error, isFetching, isPending } = result
  const openEditor = useOpenInterestsEditor()
  const showKidSafe = useShowKidSafe(data)
  const refreshRef = useRef<HTMLButtonElement>(null)

  const hasItems = (data?.items?.length ?? 0) > 0
  const missingInfo = data?.emptyReason ? missing[data.emptyReason] : undefined
  // Placeholders for the first load, and for a reload with nothing worth
  // keeping on screen meanwhile (such as the "add your interests" nudge).
  const loading = isPending || (isFetching && !hasItems)
  // An error stays on the query while a retry runs; hide it until that ends.
  const failed = error && !isFetching ? error : null
  // Refreshing can't help until the missing details are filled in (but after
  // a failed reload it can, and "Try again" hands focus to it).
  const canRefresh = !missingInfo || isFetching || failed !== null

  function refresh() {
    // A click that lands before the spinner shows (a quick double-click)
    // joins the load already under way rather than starting a second,
    // paid one (see useRefreshCuratedWidget).
    onRefresh()
  }

  function retry() {
    // The "Try again" button is about to disappear; keep focus in the card.
    refreshRef.current?.focus()
    refresh()
  }

  let notice = null
  if (failed) {
    notice = <LoadError noun={noun} error={failed} stale={hasItems} onRetry={retry} />
  } else if (missingInfo) {
    notice = <MissingNudge info={missingInfo} onOpen={openEditor} />
  }

  return (
    <WidgetCard
      widget={data}
      title={title}
      showKidSafe={showKidSafe}
      loading={loading}
      loadingMessage={loadingMessage}
      emptyMessage={NOTHING_NEW}
      notice={notice}
      headerAction={
        <>
          {canRefresh && (
            <RefreshButton
              ref={refreshRef}
              label={`Refresh ${noun}`}
              refreshing={isFetching}
              onRefresh={refresh}
            />
          )}
          {/* Always mounted, so screen readers hear it when the text changes. */}
          <span role="status" className="sr-only">
            {isFetching && data ? `Refreshing ${noun}…` : ''}
          </span>
        </>
      }
    />
  )
}

interface LoadErrorProps {
  noun: string
  error: ApiError
  /** Earlier items are still showing below. */
  stale: boolean
  onRetry: () => void
}

/** A small inline error with a "Try again" button; the rest of the page carries on. */
export function LoadError({ noun, error, stale, onRetry }: LoadErrorProps) {
  return (
    <div
      role="status"
      className="flex flex-col gap-3 rounded-2xl bg-danger-soft px-4 py-3 text-sm @md:flex-row @md:items-center @md:justify-between"
    >
      <div>
        <p className="font-medium text-danger">
          {stale ? `Couldn’t load fresh ${noun}` : `Couldn’t load ${noun}`}
        </p>
        <p className="mt-0.5 text-ink-soft">{[error.message, error.hint].filter(Boolean).join(' ')}</p>
      </div>
      <button
        type="button"
        onClick={onRetry}
        className="shrink-0 self-start rounded-xl border border-primary/30 bg-surface px-3.5 py-2 font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary @md:self-auto"
      >
        Try again
      </button>
    </div>
  )
}

interface MissingNudgeProps {
  info: MissingInfo
  /** Opens the interests editor; null where the page has none, so there's no button. */
  onOpen: ((field: ProfileField) => void) | null
}

/** Says what's missing, with a button straight to the field that fixes it. */
export function MissingNudge({ info, onOpen }: MissingNudgeProps) {
  return (
    <div className="rounded-2xl bg-bg px-4 py-6 text-center">
      <p className="mx-auto max-w-xs text-sm leading-relaxed text-ink-soft">{info.message}</p>
      {onOpen && (
        <button
          type="button"
          onClick={() => onOpen(info.field)}
          className="mt-4 rounded-xl border border-primary/30 bg-surface px-3.5 py-2 text-sm font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          {info.action}
        </button>
      )}
    </div>
  )
}
