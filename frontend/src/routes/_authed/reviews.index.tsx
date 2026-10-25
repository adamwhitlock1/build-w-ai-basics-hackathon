import { Link, createFileRoute } from '@tanstack/react-router'
import { useEffect } from 'react'
import { categoryNames, useMarkReviewed, useReviews, type ChatFlag } from '../../api/chat'
import { useFamily } from '../../api/hooks'
import { ReviewsLayout } from '../../chat/ReviewsLayout'
import { FormError } from '../../components/FormError'
import { formatWhen, nameFor } from '../../family/people'

export const Route = createFileRoute('/_authed/reviews/')({
  component: ReviewsPage,
})

/**
 * The parents' review queue: flagged messages from the kids' chats, the
 * unreviewed ones first (possible crises first among them), each with a
 * link to the whole conversation; and each kid's chats.
 */
function ReviewsPage() {
  const reviews = useReviews()
  const family = useFamily()
  const kids = family.data?.members.filter((m) => !m.isAdmin) ?? []
  const nameOf = (id: number) => {
    const member = family.data?.members.find((m) => m.id === id)
    return member ? nameFor(member.email) : 'A kid'
  }
  const flags = reviews.data?.flags ?? []
  const waiting = flags.filter((f) => !f.reviewedAt)
  const reviewed = flags.filter((f) => f.reviewedAt)

  // Coming from the crisis alert (#flag-12): bring that flag into view.
  useEffect(() => {
    if (!reviews.data || !window.location.hash) return
    document.getElementById(window.location.hash.slice(1))?.scrollIntoView({ block: 'center' })
  }, [reviews.data])

  return (
    <ReviewsLayout back={{ to: '/', label: 'Back to your page' }}>
      <header className="mb-6">
        <h1 className="text-4xl leading-tight text-ink sm:text-5xl">Reviews</h1>
        <p className="mt-2 max-w-prose text-ink-soft">
          Messages from your kids’ chats that the safety check flagged. Nothing was blocked: the kids got an answer, and
          anything that looked like a crisis came with help numbers.
        </p>
      </header>

      {reviews.isError && <FormError message={reviews.error.message} hint={reviews.error.hint} />}

      <section aria-labelledby="waiting-heading" className="mb-10">
        <h2 id="waiting-heading" className="mb-3 text-2xl text-ink">
          To review {waiting.length > 0 && <span className="text-ink-muted">({waiting.length})</span>}
        </h2>
        {reviews.data && waiting.length === 0 && <p className="text-ink-soft">Nothing waiting. All caught up.</p>}
        <ul className="flex flex-col gap-3">
          {waiting.map((f) => (
            <FlagItem key={f.id} flag={f} kidName={nameOf(f.memberId)} />
          ))}
        </ul>
      </section>

      {kids.length > 0 && (
        <section aria-labelledby="kids-heading" className="mb-10">
          <h2 id="kids-heading" className="mb-3 text-2xl text-ink">
            Kids’ chats
          </h2>
          <ul className="flex flex-wrap gap-2">
            {kids.map((k) => (
              <li key={k.id}>
                <Link
                  to="/reviews/$memberId"
                  params={{ memberId: String(k.id) }}
                  className="inline-flex h-10 items-center rounded-2xl border border-primary/30 bg-surface px-4 text-sm font-medium text-primary hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-primary"
                >
                  {nameFor(k.email)}’s chats
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {reviewed.length > 0 && (
        <section aria-labelledby="reviewed-heading">
          <h2 id="reviewed-heading" className="mb-3 text-2xl text-ink">
            Reviewed
          </h2>
          <ul className="flex flex-col gap-3">
            {reviewed.map((f) => (
              <FlagItem key={f.id} flag={f} kidName={nameOf(f.memberId)} reviewerName={nameOf(f.reviewedBy ?? 0)} />
            ))}
          </ul>
        </section>
      )}
    </ReviewsLayout>
  )
}

/** One flagged message in the queue. */
function FlagItem({ flag, kidName, reviewerName }: { flag: ChatFlag; kidName: string; reviewerName?: string }) {
  const mark = useMarkReviewed()
  return (
    <li
      id={`flag-${flag.id}`}
      className={`scroll-mt-24 rounded-3xl bg-surface p-5 shadow-card ${flag.crisis && !flag.reviewedAt ? 'ring-2 ring-danger' : ''}`}
    >
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="font-medium text-ink">{kidName}</span>
        <span className="text-ink-muted">{formatWhen(flag.createdAt)}</span>
        <span
          className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
            flag.crisis ? 'bg-danger text-on-danger' : 'bg-danger-soft text-danger'
          }`}
        >
          {flag.crisis ? 'Possible crisis' : (categoryNames[flag.category] ?? flag.category)} · {flag.severity}
        </span>
      </div>
      {flag.reason && <p className="mt-2 text-ink">{flag.reason}</p>}
      {flag.messageText && (
        <blockquote className="mt-2 border-l-4 border-line pl-3 text-ink-soft whitespace-pre-wrap">
          {flag.messageText}
        </blockquote>
      )}
      <div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
        <Link
          to="/reviews/$memberId/$chatId"
          params={{ memberId: String(flag.memberId), chatId: String(flag.conversationId) }}
          search={{ message: flag.messageId }}
          className="font-medium text-primary underline-offset-4 hover:underline"
        >
          Read the conversation{flag.conversationTitle ? `: ${flag.conversationTitle}` : ''}
        </Link>
        {flag.reviewedAt ? (
          <span className="text-ink-muted">
            Reviewed by {reviewerName} · {formatWhen(flag.reviewedAt)}
          </span>
        ) : (
          <button
            type="button"
            disabled={mark.isPending}
            onClick={() => mark.mutate(flag.id)}
            className="ml-auto h-9 rounded-2xl bg-ink px-4 font-medium text-bg hover:bg-ink-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-60"
          >
            Mark reviewed
          </button>
        )}
      </div>
      {mark.isError && <FormError message={mark.error.message} hint={mark.error.hint} />}
    </li>
  )
}
