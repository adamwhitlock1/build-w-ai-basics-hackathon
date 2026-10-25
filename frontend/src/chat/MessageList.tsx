import type { ChatFlag, ChatMessage, ToolCall } from '../api/chat'
import { categoryNames, toolNoteText } from '../api/chat'

interface MessageListProps {
  messages: ChatMessage[]
  /** Who the kid is, for a parent reading along ("Jordan"); kids see "You". */
  kidName?: string
  /** A parent's view: flags to mark on their messages. */
  flags?: ChatFlag[]
  /** A message to highlight and scroll to (from the review queue). */
  highlightId?: number
  /**
   * The tools the agent used. Each note shows above the reply it informed
   * (by messageId); a note whose reply failed shows after the last message
   * sent before it.
   */
  toolCalls?: ToolCall[]
}

/**
 * A conversation's messages as chat bubbles: the kid's on the right, the
 * agent's on the left. Line breaks in a message are kept, so lists and the
 * crisis resources read as written.
 */
export function MessageList({ messages, kidName, flags, highlightId, toolCalls }: MessageListProps) {
  const flagsByMessage = new Map<number, ChatFlag>()
  for (const f of flags ?? []) flagsByMessage.set(f.messageId, f)

  // Notes above a reply, and notes (of a failed reply) after a message.
  const before = new Map<number, ToolCall[]>()
  const after = new Map<number, ToolCall[]>()
  const ids = new Set(messages.map((m) => m.id))
  for (const call of toolCalls ?? []) {
    if (call.messageId !== null && ids.has(call.messageId)) {
      before.set(call.messageId, [...(before.get(call.messageId) ?? []), call])
      continue
    }
    const sentBefore = messages.filter((m) => m.createdAt && m.createdAt <= call.createdAt)
    const anchor = sentBefore[sentBefore.length - 1] ?? messages[messages.length - 1]
    if (anchor) after.set(anchor.id, [...(after.get(anchor.id) ?? []), call])
  }

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
            data-role={m.role}
            className={`flex flex-col ${fromKid ? 'items-end' : 'items-start'} ${highlighted ? 'scroll-mt-24' : ''}`}
          >
            {before.has(m.id) && <ToolNotes calls={before.get(m.id) ?? []} />}
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
            {after.has(m.id) && <ToolNotes calls={after.get(m.id) ?? []} />}
          </li>
        )
      })}
    </ol>
  )
}

/** The small notes saying which tools the agent used. */
function ToolNotes({ calls }: { calls: ToolCall[] }) {
  return (
    <ul className="mb-1.5 flex flex-wrap gap-1.5 self-start">
      {calls.map((c, i) => (
        <li key={i} className="rounded-full bg-accent-soft px-2.5 py-0.5 text-xs text-ink-soft">
          {toolNoteText(c.tool, c.detail)}
        </li>
      ))}
    </ul>
  )
}
