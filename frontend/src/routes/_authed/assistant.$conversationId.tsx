import { isDemoLimit } from '../../public/demoState'
import { useQueryClient } from '@tanstack/react-query'
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import {
  assistantListQueryOptions,
  assistantQueryOptions,
  kindNames,
  sendAssistantMessage,
  useAssistantConversation,
  useDeleteAssistant,
} from '../../api/assistant'
import type { ChatMessage, ToolCall } from '../../api/chat'
import { ApiError } from '../../api/client'
import { MessageList } from '../../chat/MessageList'
import { FormError } from '../../components/FormError'
import { EmptyState } from '../../components/EmptyState'

export const Route = createFileRoute('/_authed/assistant/$conversationId')({
  component: AssistantPage,
})

/** The id the reply being written has until it's stored. */
const STREAMING_ID = -1

/** The longest message a parent can send (the server's limit too). */
const MAX_CHARS = 8000

/** One Assistant conversation: messages, the reply streaming in, and the composer. */
function AssistantPage() {
  const id = Number(Route.useParams().conversationId)
  const conversation = useAssistantConversation(id)
  const remove = useDeleteAssistant()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [text, setText] = useState('')
  const [pending, setPending] = useState<ChatMessage | null>(null)
  const [streaming, setStreaming] = useState<string | null>(null)
  const [partial, setPartial] = useState<{ id: number; text: string } | null>(null)
  const [liveTools, setLiveTools] = useState<ToolCall[]>([])
  const [error, setError] = useState<ApiError | null>(null)
  const [confirming, setConfirming] = useState(false)
  const sending = streaming !== null
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' })
  }, [conversation.data?.messages.length, streaming, pending, liveTools.length])

  async function send() {
    const message = text.trim()
    if (!message || sending) return
    setError(null)
    setLiveTools([])
    setStreaming('')
    setPartial(null)
    let replySoFar = ''
    try {
      await sendAssistantMessage(id, message, {
        onMessage: (m) => {
          setText('')
          setPending(m)
        },
        onTool: (tool, detail) =>
          setLiveTools((calls) => [...calls, { tool, detail, messageId: STREAMING_ID, createdAt: '' }]),
        onDelta: (piece) => { replySoFar += piece; setStreaming(replySoFar) },
        onDone: () => {},
      })
    } catch (err) {
      if (err instanceof ApiError && isDemoLimit(err.message)) setPartial({ id: id, text: replySoFar })
      setError(err instanceof ApiError ? err : new ApiError(0, 'Something went wrong.', 'Try again in a moment.'))
    } finally {
      await queryClient.invalidateQueries({ queryKey: assistantQueryOptions(id).queryKey })
      setPending(null)
      setStreaming(null)
      setLiveTools([])
      void queryClient.invalidateQueries({ queryKey: assistantListQueryOptions.queryKey, exact: true })
    }
  }

  async function deleteConversation() {
    await remove.mutateAsync(id)
    await navigate({ to: '/assistant' })
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    void send()
  }

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      void send()
    }
  }

  const messages = [
    ...(conversation.data?.messages ?? []),
    ...(pending ? [pending] : []),
    // Until its first words arrive it's empty, and MessageList shows the
    // breathing light and "Thinking…" in its place.
    ...(partial?.id === id && partial.text ? [{ id: STREAMING_ID - 1, role: 'agent' as const, text: `Incomplete answer (demo allowance reached):\n\n${partial.text}`, createdAt: '' }] : []),
    ...(streaming !== null && (pending || streaming || liveTools.length > 0)
      ? [{ id: STREAMING_ID, role: 'agent' as const, text: streaming, createdAt: '' }]
      : []),
  ]
  const kind = conversation.data?.conversation.kind

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-3xl flex-col px-4 py-6 sm:px-6 sm:py-10">
      <Link to="/assistant" className="text-sm font-medium text-primary underline-offset-4 hover:underline">
        ← All conversations
      </Link>
      <header className="mt-3 mb-5">
        <h1 className="text-3xl leading-tight break-words text-ink sm:text-4xl">
          {conversation.data?.conversation.title || 'New conversation'}
        </h1>
        {kind && (
          <p className="mt-2 flex flex-wrap items-center gap-2 text-sm text-ink-soft">
            <span className="rounded-full bg-primary-soft px-3 py-1 text-primary">{kindNames[kind]}</span>
            {kind === 'kids'
              ? 'Can read the kids’ chats; runs only on providers that keep nothing.'
              : 'Can’t read the kids’ chats; start “About the kids” for that.'}
          </p>
        )}
      </header>

      {conversation.isError && <FormError message={conversation.error.message} hint={conversation.error.hint} />}

      <div className="flex-1" aria-live="polite" aria-busy={sending}>
        {messages.length === 0 && conversation.data ? (
          <EmptyState>
            {kind === 'kids'
              ? 'Ask about the kids: “Anything I should know about Lily’s chats this week?”'
              : 'Ask anything: “What does everyone have tomorrow?” or “Find a good beginner robotics kit.”'}
          </EmptyState>
        ) : (
          <MessageList
            messages={messages}
            toolCalls={[...(conversation.data?.toolCalls ?? []), ...liveTools]}
            linkify
          />
        )}
        <div ref={endRef} />
      </div>

      {error && (
        <div className="mt-4">
          <FormError message={error.message} hint={error.hint} />
        </div>
      )}

      {conversation.data && (
        <>
          <form onSubmit={handleSubmit} className="lantern-glass sticky bottom-0 mt-4 -mx-2 rounded-t-3xl px-2 pt-2 pb-3">
            <label htmlFor="assistant-message" className="sr-only">
              Your message
            </label>
            <div className="flex items-end gap-2 rounded-3xl border border-control bg-surface p-2 shadow-card">
              <textarea
                id="assistant-message"
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={handleKeyDown}
                rows={2}
                maxLength={MAX_CHARS}
                placeholder="Ask your Assistant…"
                className="min-h-11 flex-1 resize-none bg-transparent px-3 py-2 text-ink outline-none placeholder:text-ink-muted"
              />
              <button
                type="submit"
                disabled={sending || text.trim() === ''}
                className="h-11 shrink-0 rounded-2xl bg-primary px-4 font-medium text-on-primary transition-colors hover:bg-primary-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-50"
              >
                Send
              </button>
            </div>
          </form>
          <div className="pb-4">
            {confirming ? (
              <div className="flex flex-wrap items-center gap-3 text-sm">
                <span className="text-ink">Delete this conversation for good?</span>
                <button
                  type="button"
                  disabled={remove.isPending}
                  onClick={() => void deleteConversation().catch(() => {})}
                  className="h-9 rounded-2xl bg-danger px-3 font-medium text-on-danger hover:opacity-90 disabled:opacity-60"
                >
                  Delete
                </button>
                <button type="button" onClick={() => setConfirming(false)} className="h-9 px-2 text-ink-soft hover:text-ink">
                  Cancel
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setConfirming(true)}
                className="text-sm text-danger underline-offset-4 hover:underline"
              >
                Delete conversation…
              </button>
            )}
            {remove.isError && <FormError message={remove.error.message} hint={remove.error.hint} />}
          </div>
        </>
      )}
    </main>
  )
}
