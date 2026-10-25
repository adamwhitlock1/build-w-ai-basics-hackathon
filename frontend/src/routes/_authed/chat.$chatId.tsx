import { useQueryClient } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { ApiError } from '../../api/client'
import {
  chatQueryOptions,
  chatsQueryOptions,
  modeNames,
  sendChatMessage,
  useChat,
  type ChatMessage,
  type ToolCall,
} from '../../api/chat'
import { ChatNotice } from '../../chat/ChatNotice'
import { MessageList } from '../../chat/MessageList'
import { FormError } from '../../components/FormError'

export const Route = createFileRoute('/_authed/chat/$chatId')({
  component: ChatPage,
})

/** The id the reply being written has until it's stored. */
const STREAMING_ID = -1

/** The longest message a kid can send (the server's limit too). */
const MAX_CHARS = 4000

/**
 * One conversation: its messages, the reply streaming in as it's written,
 * and the box to type the next message.
 */
function ChatPage() {
  const chatId = Number(Route.useParams().chatId)
  const chat = useChat(chatId)
  const queryClient = useQueryClient()
  const [text, setText] = useState('')
  // While a reply streams: the kid's message as stored, and the reply so far.
  const [pendingKid, setPendingKid] = useState<ChatMessage | null>(null)
  const [streaming, setStreaming] = useState<string | null>(null)
  // The tools the agent uses while this reply is being written.
  const [liveTools, setLiveTools] = useState<ToolCall[]>([])
  const [error, setError] = useState<ApiError | null>(null)
  const sending = streaming !== null
  const endRef = useRef<HTMLDivElement>(null)

  // Keep the newest message in view as the reply grows.
  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' })
  }, [chat.data?.messages.length, streaming, pendingKid])

  async function send() {
    const message = text.trim()
    if (!message || sending) return
    setError(null)
    setLiveTools([])
    setStreaming('')
    try {
      await sendChatMessage(chatId, message, {
        onMessage: (m) => {
          setText('')
          setPendingKid(m)
        },
        onTool: (tool, detail) =>
          setLiveTools((calls) => [...calls, { tool, detail, messageId: STREAMING_ID, createdAt: '' }]),
        onDelta: (piece) => setStreaming((so) => (so ?? '') + piece),
        onDone: () => {},
      })
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, 'Something went wrong.', 'Try again in a moment.'))
    } finally {
      // Fetch the conversation as stored (both messages, and the title a
      // first message gives it) before dropping the streamed copy, so
      // nothing flickers. A failed reply left the kid's message stored.
      await queryClient.invalidateQueries({ queryKey: chatQueryOptions(chatId).queryKey })
      setPendingKid(null)
      setStreaming(null)
      setLiveTools([])
      void queryClient.invalidateQueries({ queryKey: chatsQueryOptions.queryKey, exact: true })
    }
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    void send()
  }

  // Enter sends; Shift+Enter starts a new line.
  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      void send()
    }
  }

  const messages = [
    ...(chat.data?.messages ?? []),
    ...(pendingKid ? [pendingKid] : []),
    // The reply being written; its tool notes show above it, even before
    // any text has arrived.
    ...(streaming !== null && (streaming || liveTools.length > 0)
      ? [{ id: STREAMING_ID, role: 'agent' as const, text: streaming || '…', createdAt: '' }]
      : []),
  ]

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-3xl flex-col px-4 py-6 sm:px-6 sm:py-10">
      <Link to="/chat" className="text-sm font-medium text-primary underline-offset-4 hover:underline">
        ← All chats
      </Link>
      <header className="mt-3 mb-5">
        <h1 className="text-3xl leading-tight break-words text-ink sm:text-4xl">
          {chat.data?.conversation.title || 'New chat'}
        </h1>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          {chat.data && (
            <span className="rounded-full bg-accent-soft px-3 py-1 text-sm text-ink">
              {modeNames[chat.data.conversation.mode]}
            </span>
          )}
          <ChatNotice />
        </div>
      </header>

      {chat.isError && <FormError message={chat.error.message} hint={chat.error.hint} />}

      <div className="flex-1" aria-live="polite" aria-busy={sending}>
        {messages.length === 0 && chat.data ? (
          <p className="text-ink-soft">
            {chat.data.conversation.mode === 'homework'
              ? 'What are you working on? Share the question and what you’ve tried so far.'
              : 'What are you curious about today?'}
          </p>
        ) : (
          <MessageList messages={messages} toolCalls={[...(chat.data?.toolCalls ?? []), ...liveTools]} />
        )}
        {sending && streaming === '' && liveTools.length === 0 && (
          <p role="status" className="mt-3 text-sm text-ink-muted">
            Thinking…
          </p>
        )}
        <div ref={endRef} />
      </div>

      {error && (
        <div className="mt-4">
          <FormError message={error.message} hint={error.hint} />
        </div>
      )}

      {chat.data && (
        <form onSubmit={handleSubmit} className="sticky bottom-0 mt-4 bg-bg pt-2 pb-4">
          <label htmlFor="chat-message" className="sr-only">
            Your message
          </label>
          <div className="flex items-end gap-2 rounded-3xl bg-surface p-2 shadow-card">
            <textarea
              id="chat-message"
              value={text}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={handleKeyDown}
              rows={2}
              maxLength={MAX_CHARS}
              placeholder="Type a message…"
              className="min-h-11 flex-1 resize-none bg-transparent px-3 py-2 text-ink outline-none placeholder:text-ink-muted"
            />
            <button
              type="submit"
              disabled={sending || text.trim() === ''}
              className="h-11 shrink-0 rounded-2xl bg-accent px-4 font-medium text-ink transition-colors hover:bg-accent-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent-strong disabled:cursor-not-allowed disabled:opacity-50"
            >
              Send
            </button>
          </div>
        </form>
      )}
    </main>
  )
}
