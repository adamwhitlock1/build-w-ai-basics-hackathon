import { useRef } from 'react'
import { useOpenInterestsEditor } from '../account/interestsEditorContext'
import type { TodayEvent, TodayWidget as TodayData } from '../api/client'
import { usePageOwner, useRefreshToday, useToday } from '../api/hooks'
import { LoadError, MissingNudge, type MissingInfo } from './CuratedWidgetCard'
import { RefreshButton } from './RefreshButton'
import { useNow } from './updatedAgo'
import { WidgetCard } from './WidgetCard'

/** From this hour (local time) the card also shows tomorrow's events. */
const EVENING_HOUR = 18

/**
 * The Today card: what's on the calendar of whoever's page it's on, today
 * (and, from 6 pm, tomorrow), read from the calendar link they connected.
 * Like the curated cards it loads its own data, has a refresh button, and
 * nudges toward the profile panel when there's no calendar yet.
 */
export function TodayWidget() {
  const { data, error, isFetching, isPending } = useToday()
  const refresh = useRefreshToday()
  const openEditor = useOpenInterestsEditor()
  const { isYou } = usePageOwner()
  const now = useNow(60_000)
  const refreshRef = useRef<HTMLButtonElement>(null)

  const reason = data?.emptyReason || ''
  const loading = isPending || (isFetching && !data)
  // An error stays on the query while a retry runs; hide it until that ends.
  const failed = error && !isFetching ? error : null
  // With no calendar, refreshing can't help until one is connected.
  const canRefresh = reason !== 'no_calendar' || isFetching || failed !== null
  const hasEvents = (data?.today.length ?? 0) > 0

  function retry() {
    refreshRef.current?.focus()
    refresh()
  }

  const missing: MissingInfo | null =
    reason === 'no_calendar'
      ? isYou
        ? { message: 'Add your calendar to see what’s on today.', action: 'Add your calendar', field: 'calendar' }
        : { message: 'Add their calendar to see what’s on their day.', action: 'Add their calendar', field: 'calendar' }
      : reason === 'calendar_gone'
        ? {
            message: isYou
              ? 'Your calendar link stopped working. It may have been reset in your calendar app.'
              : 'Their calendar link stopped working. It may have been reset in their calendar app.',
            action: 'Replace link',
            field: 'calendar',
          }
        : null

  let notice = null
  if (failed) {
    notice = <LoadError noun="today’s events" error={failed} stale={hasEvents} onRetry={retry} />
  } else if (missing) {
    notice = <MissingNudge info={missing} onOpen={openEditor} />
  }

  const showTomorrow = new Date(now).getHours() >= EVENING_HOUR
  const body = data && !reason ? <Days data={data} now={now} showTomorrow={showTomorrow} /> : null

  return (
    <WidgetCard
      title={isYou ? 'Today' : 'Their day'}
      loading={loading}
      loadingMessage={isYou ? 'Checking your calendar…' : 'Checking their calendar…'}
      notice={notice}
      footerNote={data?.provider && !reason ? `From ${isYou ? 'your' : 'their'} ${data.provider}` : undefined}
      updatedAt={data && !reason ? data.updatedAt : undefined}
      headerAction={
        <>
          {canRefresh && (
            <RefreshButton ref={refreshRef} label="Refresh calendar" refreshing={isFetching} onRefresh={refresh} />
          )}
          {/* Always mounted, so screen readers hear it when the text changes. */}
          <span role="status" className="sr-only">
            {isFetching && data ? 'Refreshing calendar…' : ''}
          </span>
        </>
      }
    >
      {body}
    </WidgetCard>
  )
}

/** Today's events, and tomorrow's under their own heading when it's evening. */
function Days({ data, now, showTomorrow }: { data: TodayData; now: number; showTomorrow: boolean }) {
  const today = dayStart(data.date)
  const tomorrow = addDays(today, 1)
  return (
    <div className="space-y-5">
      <DayList
        events={data.today}
        more={data.moreToday}
        from={today}
        to={tomorrow}
        timeZone={data.timeZone}
        now={now}
        emptyText="Nothing on the calendar today."
      />
      {showTomorrow && (
        <section aria-label="Tomorrow">
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-ink-muted">Tomorrow</h3>
          <DayList
            events={data.tomorrow}
            more={data.moreTomorrow}
            from={tomorrow}
            to={addDays(tomorrow, 1)}
            timeZone={data.timeZone}
            emptyText="Nothing on the calendar tomorrow."
          />
        </section>
      )}
    </div>
  )
}

interface DayListProps {
  events: TodayEvent[]
  more: number
  /** The day's start and end, to word events that run past either. */
  from: number
  to: number
  timeZone: string
  /** Set for today: events that have ended are dimmed. */
  now?: number
  emptyText: string
}

function DayList({ events, more, from, to, timeZone, now, emptyText }: DayListProps) {
  if (events.length === 0) {
    return <p className="rounded-2xl bg-bg px-4 py-4 text-center text-sm text-ink-muted">{emptyText}</p>
  }
  return (
    <>
      <ul className="divide-y divide-line">
        {events.map((e, i) => {
          const ended = now !== undefined && !e.allDay && Date.parse(e.end) <= now
          return (
            // Index keys are fine: the list is replaced whole, never reordered.
            <li key={i} className={`flex gap-4 py-3 first:pt-0 last:pb-0 ${ended ? 'opacity-55' : ''}`}>
              <p className="w-24 shrink-0 text-sm tabular-nums text-ink-soft">{when(e, from, to, timeZone)}</p>
              <div className="min-w-0">
                <p className={`font-medium leading-snug text-ink ${ended ? 'line-through decoration-ink-muted/50' : ''}`}>
                  {e.title || 'Busy'}
                  {ended && <span className="sr-only"> (ended)</span>}
                </p>
                {e.location && <p className="mt-0.5 truncate text-xs text-ink-muted">{e.location}</p>}
              </div>
            </li>
          )
        })}
      </ul>
      {more > 0 && <p className="mt-3 text-xs text-ink-muted">+{more} more</p>}
    </>
  )
}

/**
 * When an event happens, in words for its row: "All day", "3:30 – 4:30 PM",
 * "Until 12 PM" (started on an earlier day), or "From 5 PM" (runs into the
 * next day).
 */
function when(e: TodayEvent, from: number, to: number, timeZone: string): string {
  if (e.allDay) return 'All day'
  const start = Date.parse(e.start)
  const end = Date.parse(e.end)
  const time = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit', timeZone })
  const startsEarlier = start < from
  const endsLater = end > to
  if (startsEarlier && endsLater) return 'All day'
  if (startsEarlier) return `Until ${time.format(end)}`
  if (endsLater) return `From ${time.format(start)}`
  if (end <= start) return time.format(start)
  return time.formatRange(start, end)
}

/** Local midnight of a YYYY-MM-DD date, in milliseconds. */
function dayStart(date: string): number {
  const [y, m, d] = date.split('-').map(Number)
  return new Date(y, m - 1, d).getTime()
}

function addDays(ms: number, days: number): number {
  const d = new Date(ms)
  d.setDate(d.getDate() + days)
  return d.getTime()
}
