import { Link } from '@tanstack/react-router'
import { useAlerts } from '../api/chat'
import { useFamily, useMe } from '../api/hooks'
import { nameFor } from '../family/people'

/**
 * The alert across the top of every parent page while a possible crisis in
 * a kid's chat is unreviewed. It names the kid and links to the flag; it
 * goes away once every crisis flag has been reviewed. Kids never see it.
 */
export function CrisisAlert() {
  const { data: me } = useMe()
  const isParent = me?.user.isAdmin ?? false
  const alerts = useAlerts(isParent)
  const family = useFamily()
  const crises = alerts.data?.crises ?? []
  if (!isParent || crises.length === 0) return null

  const names = [...new Set(crises.map((c) => c.memberId))].map((id) => {
    const member = family.data?.members.find((m) => m.id === id)
    return member ? nameFor(member.email) : 'One of your kids'
  })
  const who = names.length === 1 ? names[0] : `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`

  return (
    <div role="alert" className="mb-6 rounded-3xl bg-danger px-5 py-4 text-white shadow-card sm:mb-8">
      <p className="text-lg font-medium">{who} may need help now.</p>
      <p className="mt-1 text-sm text-white/90">
        Something {names.length === 1 ? 'they' : 'one of them'} wrote in Porchlight chat was flagged as a possible
        crisis. Please check in with them, and review it.
      </p>
      <Link
        to="/reviews"
        hash={`flag-${crises[0].flagId}`}
        className="mt-3 inline-flex h-10 items-center rounded-2xl bg-white px-4 text-sm font-medium text-danger hover:bg-danger-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
      >
        Review it now
      </Link>
    </div>
  )
}
