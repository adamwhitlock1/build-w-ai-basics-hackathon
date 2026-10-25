import { Link, createFileRoute } from '@tanstack/react-router'
import { modeNames, useMemberChats } from '../../api/chat'
import { useFamilyMember } from '../../api/hooks'
import { ReviewsLayout } from '../../chat/ReviewsLayout'
import { FormError } from '../../components/FormError'
import { formatWhen, nameFor } from '../../family/people'

export const Route = createFileRoute('/_authed/reviews/$memberId/')({
  component: KidChatsPage,
})

/** Every conversation one kid has had, for a parent to read. */
function KidChatsPage() {
  const memberId = Number(Route.useParams().memberId)
  const member = useFamilyMember(memberId)
  const chats = useMemberChats(memberId)
  const name = member ? nameFor(member.email) : 'Their'

  return (
    <ReviewsLayout back={{ to: '/reviews', label: 'Reviews' }}>
      <h1 className="mb-6 text-4xl leading-tight text-ink sm:text-5xl">
        {member ? `${name}’s chats` : 'Their chats'}
      </h1>
      {chats.isError && <FormError message={chats.error.message} hint={chats.error.hint} />}
      {chats.data && chats.data.conversations.length === 0 && <p className="text-ink-soft">No chats yet.</p>}
      {chats.data && chats.data.conversations.length > 0 && (
        <ul className="divide-y divide-line rounded-3xl bg-surface px-5 shadow-card">
          {chats.data.conversations.map((c) => (
            <li key={c.id}>
              <Link
                to="/reviews/$memberId/$chatId"
                params={{ memberId: String(memberId), chatId: String(c.id) }}
                className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 py-3 text-ink hover:text-primary focus-visible:outline-2 focus-visible:outline-primary"
              >
                <span className="truncate">{c.title || 'New chat'}</span>
                <span className="flex shrink-0 items-center gap-2 text-xs text-ink-muted">
                  {(c.unreviewedFlags ?? 0) > 0 && (
                    <span className="rounded-full bg-danger-soft px-2 py-0.5 font-medium text-danger">
                      {c.unreviewedFlags} to review
                    </span>
                  )}
                  {modeNames[c.mode]} · {formatWhen(c.updatedAt)}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </ReviewsLayout>
  )
}
