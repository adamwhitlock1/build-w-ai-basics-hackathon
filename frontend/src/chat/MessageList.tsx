import type { ChatFlag, ChatMessage } from '../api/chat'
import { categoryNames } from '../api/chat'

interface MessageListProps {
  messages: ChatMessage[]
  /** Who the kid is, for a parent reading along ("Jordan"); kids see "You". */
  kidName?: string
  /** A parent's view: flags to mark on their messages. */
  flags?: ChatFlag[]
  /** A message to highlight and scroll to (from the review queue). */
  highlightId?: number
}

/**
 * A conversation's messages as chat bubbles: the kid's on the right, the
 * agent's on the left. Line breaks in a message are kept, so lists and the
 * crisis resources read as written.
 */
export function MessageList({ messages, kidName, flags, highlightId }: MessageListProps) {
  const flagsByMessage = new Map<number, ChatFlag>()
  for (const f of flags ?? []) flagsByMessage.set(f.messageId, f)

  return (
    <ol className="flex flex-col gap-3">
      {messages.map((m) => {
        const fromKid = m.role === 'kid'
        const flag = flagsByMessage.get(m.id)
        const highlighted = m.id === highlightId
        return (
          <li
            key={m.id}
            id={`message-${m.id}`}
            className={`flex flex-col ${fromKid ? 'items-end' : 'items-start'} ${highlighted ? 'scroll-mt-24' : ''}`}
          >
            <span className="mb-1 px-1 text-xs text-ink-muted">{fromKid ? (kidName ?? 'You') : 'Porchlight'}</span>
            <div
              className={`max-w-[85%] rounded-3xl px-4 py-2.5 whitespace-pre-wrap break-words ${
                fromKid ? 'rounded-br-lg bg-primary text-white' : 'rounded-bl-lg bg-surface text-ink shadow-card'
              } ${highlighted ? 'ring-4 ring-accent' : ''}`}
            >
              {m.text}
            </div>
            {flag && (
              <span
                className={`mt-1 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  flag.crisis ? 'bg-danger text-white' : 'bg-danger-soft text-danger'
                }`}
              >
                {flag.crisis ? 'Possible crisis' : `Flagged: ${categoryNames[flag.category] ?? flag.category}`} ·{' '}
                {flag.severity}
                {flag.reviewedAt ? ' · reviewed' : ''}
              </span>
            )}
          </li>
        )
      })}
    </ol>
  )
}
