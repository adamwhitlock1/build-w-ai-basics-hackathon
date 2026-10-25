import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { kindNames, useAssistantList, useStartAssistant, type AssistantKind } from '../../api/assistant'
import { useMe } from '../../api/hooks'
import { FormError } from '../../components/FormError'
import { formatShortDate } from '../../family/people'

export const Route = createFileRoute('/_authed/assistant/')({
  component: AssistantListPage,
})

const kinds: { kind: AssistantKind; blurb: string }[] = [
  { kind: 'anything', blurb: 'Research anything, check the family’s day, the weather or your picks.' },
  {
    kind: 'kids',
    blurb: 'Everything above, plus the kids’ chats: flags to review, what they’ve been asking. Runs only on providers that keep nothing.',
  },
]

/** A parent's Assistant conversations, and the two ways to start one. */
function AssistantListPage() {
  const { data: me } = useMe()
  const list = useAssistantList()
  const start = useStartAssistant()
  const navigate = useNavigate()

  async function startConversation(kind: AssistantKind) {
    const c = await start.mutateAsync(kind)
    await navigate({ to: '/assistant/$conversationId', params: { conversationId: String(c.id) } })
  }

  if (me && !me.user.isAdmin) {
    return (
      <main className="mx-auto w-full max-w-3xl px-4 py-12">
        <p className="rounded-3xl bg-surface p-6 text-ink-soft shadow-card">The Assistant is for parents.</p>
      </main>
    )
  }

  return (
    <main className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6 sm:py-12">
      <Link to="/" className="text-sm font-medium text-primary underline-offset-4 hover:underline">
        ← Back to your page
      </Link>
      <header className="mt-4 mb-6">
        <h1 className="text-4xl leading-tight text-ink sm:text-5xl">Assistant</h1>
        <p className="mt-2 max-w-prose text-ink-soft">
          Your private assistant: only you can see these conversations. It reads and answers, and never changes
          anything.
        </p>
      </header>

      {list.isError && <FormError message={list.error.message} hint={list.error.hint} />}

      {list.data && (
        <section aria-label="New conversation" className="mb-8 grid gap-3 sm:grid-cols-2">
          {kinds.map(({ kind, blurb }) => {
            const off = kind === 'kids' && !list.data.kidsAvailable
            return (
              <button
                key={kind}
                type="button"
                disabled={start.isPending || off}
                onClick={() => void startConversation(kind).catch(() => {})}
                className="rounded-3xl border-2 border-primary/30 bg-surface p-5 text-left transition-colors hover:border-primary hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:border-primary/30 disabled:hover:bg-surface"
              >
                <span className="block font-display text-2xl text-ink">{kindNames[kind]}</span>
                <span className="mt-1 block text-sm text-ink-soft">{blurb}</span>
                {off && (
                  <span className="mt-2 block text-sm text-danger">
                    Needs kid chat to be set up (a kids’ OpenRouter key).
                  </span>
                )}
              </button>
            )
          })}
          {start.isError && <FormError message={start.error.message} hint={start.error.hint} />}
        </section>
      )}

      {list.data && list.data.conversations.length > 0 && (
        <section aria-labelledby="past-heading">
          <h2 id="past-heading" className="mb-2 text-2xl text-ink">
            Your conversations
          </h2>
          <ul className="divide-y divide-line rounded-3xl bg-surface px-5 shadow-card">
            {list.data.conversations.map((c) => (
              <li key={c.id}>
                <Link
                  to="/assistant/$conversationId"
                  params={{ conversationId: String(c.id) }}
                  className="flex items-baseline justify-between gap-3 py-3 text-ink hover:text-primary focus-visible:outline-2 focus-visible:outline-primary"
                >
                  <span className="truncate">{c.title || 'New conversation'}</span>
                  <span className="shrink-0 text-xs text-ink-muted">
                    {kindNames[c.kind]} · {formatShortDate(c.updatedAt)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}
    </main>
  )
}
