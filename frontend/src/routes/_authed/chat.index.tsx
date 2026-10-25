import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { modeNames, useChats, useStartChat, type ChatMode } from '../../api/chat'
import { ChatNotice } from '../../chat/ChatNotice'
import { FormError } from '../../components/FormError'
import { formatShortDate } from '../../family/people'

export const Route = createFileRoute('/_authed/chat/')({
  component: ChatListPage,
})

const modes: { mode: ChatMode; blurb: string }[] = [
  { mode: 'homework', blurb: 'Work through an assignment with hints, step by step.' },
  { mode: 'explore', blurb: 'Ask anything, brainstorm a story, or plan a project.' },
]

/** A kid's conversations, and the two ways to start a new one. */
function ChatListPage() {
  const chats = useChats()
  const start = useStartChat()
  const navigate = useNavigate()

  async function startChat(mode: ChatMode) {
    const c = await start.mutateAsync(mode)
    await navigate({ to: '/chat/$chatId', params: { chatId: String(c.id) } })
  }

  return (
    <main className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6 sm:py-12">
      <Link to="/" className="text-sm font-medium text-primary underline-offset-4 hover:underline">
        ← Back to your page
      </Link>
      <header className="mt-4 mb-6">
        <h1 className="text-4xl leading-tight text-ink sm:text-5xl">Chat</h1>
        <ChatNotice className="mt-3" />
      </header>

      {chats.isError && <FormError message={chats.error.message} hint={chats.error.hint} />}

      {chats.data?.available === false ? (
        <p className="rounded-3xl bg-surface p-6 text-ink-soft shadow-card">Chat isn’t set up yet. Ask a parent.</p>
      ) : (
        chats.data && (
          <section aria-labelledby="new-chat-heading" className="mb-8">
            <h2 id="new-chat-heading" className="sr-only">
              New chat
            </h2>
            <div className="grid gap-3 sm:grid-cols-2">
              {modes.map(({ mode, blurb }) => (
                <button
                  key={mode}
                  type="button"
                  disabled={start.isPending}
                  onClick={() => void startChat(mode).catch(() => {})}
                  className="rounded-3xl border-2 border-accent/50 bg-surface p-5 text-left transition-colors hover:border-accent hover:bg-accent-soft/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent-strong disabled:cursor-wait disabled:opacity-60"
                >
                  <span className="block font-display text-2xl text-ink">{modeNames[mode]}</span>
                  <span className="mt-1 block text-sm text-ink-soft">{blurb}</span>
                </button>
              ))}
            </div>
            {start.isError && <FormError message={start.error.message} hint={start.error.hint} />}
          </section>
        )
      )}

      {chats.data && chats.data.conversations.length > 0 && (
        <section aria-labelledby="past-chats-heading">
          <h2 id="past-chats-heading" className="mb-2 text-2xl text-ink">
            Your chats
          </h2>
          <ul className="divide-y divide-line rounded-3xl bg-surface px-5 shadow-card">
            {chats.data.conversations.map((c) => (
              <li key={c.id}>
                <Link
                  to="/chat/$chatId"
                  params={{ chatId: String(c.id) }}
                  className="flex items-baseline justify-between gap-3 py-3 text-ink hover:text-primary focus-visible:outline-2 focus-visible:outline-primary"
                >
                  <span className="truncate">{c.title || 'New chat'}</span>
                  <span className="shrink-0 text-xs text-ink-muted">
                    {modeNames[c.mode]} · {formatShortDate(c.updatedAt)}
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
