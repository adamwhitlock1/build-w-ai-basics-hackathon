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
  /** Make http(s) links in the agent's replies clickable (the parents' Assistant). */
  linkify?: boolean
}

/**
 * A conversation's messages as chat bubbles: the kid's on the right, the
 * agent's on the left. Line breaks in a message are kept, so lists and the
 * crisis resources read as written.
 */
export function MessageList({ messages, kidName, flags, highlightId, toolCalls, linkify }: MessageListProps) {
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
        // The kid or the parent: whoever is talking to the agent.
        const fromKid = m.role !== 'agent'
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
            <span className="mb-1 px-1 text-xs text-ink-muted">{fromKid ? (kidName ?? 'You') : 'Kinlight'}</span>
            <div
              className={`max-w-[85%] rounded-3xl px-4 py-2.5 whitespace-pre-wrap break-words ${
                fromKid ? 'rounded-br-lg bg-primary text-on-primary' : 'rounded-bl-lg bg-surface text-ink shadow-card'
              } ${highlighted ? 'ring-4 ring-accent' : ''}`}
            >
              {linkify && !fromKid ? <Linked text={m.text} /> : m.text}
            </div>
            {flag && (
              <span
                className={`mt-1 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  flag.crisis ? 'bg-danger text-on-danger' : 'bg-danger-soft text-danger'
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

/** Matches an http(s) link, up to whitespace or a closing bracket. */
const LINK = /(https?:\/\/[^\s)<>\]]+)/g

/**
 * text with its http(s) links made clickable, opening in a new tab. Only
 * http and https become links; anything else stays text. A full stop or
 * comma right after a link is left out of it.
 */
function Linked({ text }: { text: string }) {
  const parts = text.split(LINK)
  return (
    <>
      {parts.map((part, i) => {
        if (i % 2 === 0) return part
        const trailing = /[.,;:!?]+$/.exec(part)?.[0] ?? ''
        const url = part.slice(0, part.length - trailing.length)
        return (
          <span key={i}>
            <a href={url} target="_blank" rel="noopener noreferrer" className="break-all underline underline-offset-2">
              {url}
            </a>
            {trailing}
          </span>
        )
      })}
    </>
  )
}
