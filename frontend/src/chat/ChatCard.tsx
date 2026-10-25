import { Link } from '@tanstack/react-router'
import { modeNames, useChats } from '../api/chat'
import { ChatNotice } from './ChatNotice'

/**
 * The Chat card at the top of a kid's page: the notice, a way to start
 * chatting, and the last few conversations. It sits outside the arranged
 * cards, so it can't be moved or removed.
 */
export function ChatCard() {
  const chats = useChats()
  const recent = chats.data?.conversations.slice(0, 3) ?? []

  return (
    <section aria-labelledby="chat-card-heading" className="mb-6 rounded-3xl bg-surface p-5 shadow-card sm:mb-8 sm:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 id="chat-card-heading" className="text-2xl text-ink">
            Ask Porchlight
          </h2>
          <p className="mt-1 text-sm text-ink-soft">Help with homework, or explore anything you’re curious about.</p>
        </div>
        {chats.data?.available !== false && (
          <Link
            to="/chat"
            className="inline-flex h-11 items-center rounded-2xl bg-primary px-4 font-medium text-on-primary transition-colors hover:bg-primary-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          >
            Start a chat
          </Link>
        )}
      </div>
      <ChatNotice className="mt-3" />
      {chats.data?.available === false && (
        <p className="mt-3 text-sm text-ink-soft">Chat isn’t set up yet. Ask a parent.</p>
      )}
      {recent.length > 0 && (
        <ul className="mt-4 divide-y divide-line border-t border-line">
          {recent.map((c) => (
            <li key={c.id}>
              <Link
                to="/chat/$chatId"
                params={{ chatId: String(c.id) }}
                className="flex items-baseline justify-between gap-3 py-2.5 text-ink hover:text-primary focus-visible:outline-2 focus-visible:outline-primary"
              >
                <span className="truncate">{c.title || 'New chat'}</span>
                <span className="shrink-0 text-xs text-ink-muted">{modeNames[c.mode]}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
