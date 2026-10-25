import { useOpenInterestsEditor } from '../account/interestsEditorContext'
import type { BriefingUse } from '../api/client'
import { useBriefing, useFamilyMember, useMe, usePageOwner, useShowKidSafe } from '../api/hooks'
import { nameFor } from '../family/people'
import { LoadError, MissingNudge } from './CuratedWidgetCard'
import { isMorning } from '../motion/light'
import { useLight } from '../motion/useLight'
import { useNow } from './updatedAgo'
import { WidgetCard } from './WidgetCard'

/** How the footer names each fact the briefing drew on. */
const USE_LABELS: Record<BriefingUse, string> = {
  weather: 'weather',
  calendar: 'calendar',
  articles: 'news',
  nearby: 'local events',
}

const list = new Intl.ListFormat(undefined, { style: 'long', type: 'conjunction' })

/** "Good morning" before noon, "Good afternoon" before 6 pm, then "Good evening". */
function greetingFor(hour: number): string {
  if (hour < 12) return 'Good morning'
  if (hour < 18) return 'Good afternoon'
  return 'Good evening'
}

/**
 * The morning briefing: a greeting for whoever's page it's on, then a few
 * sentences the AI wrote about their day from their weather, calendar, news
 * and nearby events. It has no refresh button: it's asked for again when
 * the cards it draws on update, and the server rewrites it only when what
 * they hold has changed.
 */
export function BriefingWidget() {
  const { data, error, isFetching, isPending, refetch } = useBriefing()
  const owner = usePageOwner()
  const { data: me } = useMe()
  const member = useFamilyMember(owner.isYou ? -1 : owner.userId)
  const openEditor = useOpenInterestsEditor()
  const showKidSafe = useShowKidSafe(data)
  // Once a minute, so the greeting moves from morning to afternoon by itself.
  const now = useNow(60_000)

  const email = owner.isYou ? me?.user.email : member?.email
  const greeting = greetingFor(new Date(now).getHours())
  const title = email ? `${greeting}, ${nameFor(email)}` : greeting
  const your = owner.isYou ? 'your' : 'their'

  const nothingYet = data?.emptyReason === 'nothing_yet'
  // An error stays on the query while a retry runs; hide it until that ends.
  const failed = error && !isFetching && !data ? error : null

  let notice = null
  if (failed) {
    notice = <LoadError noun={`${your} briefing`} error={failed} stale={false} onRetry={() => void refetch()} />
  } else if (nothingYet) {
    notice = (
      <MissingNudge
        info={{
          message: `Add ${your} interests, location or calendar, and ${your} morning briefing will appear here.`,
          action: owner.isYou ? 'Add your interests' : 'Add their interests',
          field: 'interests',
        }}
        onOpen={openEditor}
      />
    )
  }

  const written = data && !nothingYet && data.text ? data : null
  const uses = written?.uses.map((use) => USE_LABELS[use]) ?? []
  // In the morning, a written briefing is the screen's lit thing, unless
  // the AI is working or a card just arrived.
  const lit = useLight('briefing', !!written && isMorning(new Date(now)))

  return (
    <WidgetCard
      title={title}
      showKidSafe={showKidSafe}
      loading={isPending && !failed}
      loadingMessage={owner.isYou ? 'Writing your briefing…' : 'Writing their briefing…'}
      notice={notice}
      footerNote={uses.length > 0 ? `From ${your} ${list.format(uses)}` : undefined}
      updatedAt={written?.updatedAt}
      lit={lit}
      headerAction={
        // Always mounted, so screen readers hear it when the text changes.
        <span role="status" className="sr-only">
          {isFetching && written ? 'Updating the briefing…' : ''}
        </span>
      }
    >
      {written ? <p className="max-w-3xl text-lg leading-relaxed text-ink-soft">{written.text}</p> : null}
    </WidgetCard>
  )
}
