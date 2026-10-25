// Kid chat and parent review: the API's shapes, the TanStack Query hooks,
// and the streaming request that sends a kid's message and reads the reply
// as it's written.

import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, apiFetch } from './client'

export type ChatMode = 'homework' | 'explore'

export interface Conversation {
  id: number
  mode: ChatMode
  /** "" until the first message has been sent. */
  title: string
  createdAt: string
  updatedAt: string
  /** Parents only: how many of its flags no parent has reviewed. */
  unreviewedFlags?: number
}

export interface ChatMessage {
  id: number
  role: 'kid' | 'agent'
  text: string
  createdAt: string
}

export interface ChatFlag {
  id: number
  messageId: number
  conversationId: number
  memberId: number
  category: string
  severity: 'low' | 'medium' | 'high'
  crisis: boolean
  reason: string
  createdAt: string
  reviewedAt: string | null
  reviewedBy: number | null
  /** In the review queue only: the flagged message and its conversation's title. */
  messageText?: string
  conversationTitle?: string
}

/** A tool the agent used while replying, shown as a note above the reply. */
export interface ToolCall {
  tool: 'weather' | 'my_day' | 'my_picks' | 'search' | string
  /** A search's query; "" for the other tools. */
  detail: string
  /** The reply it informed; null if that reply failed. */
  messageId: number | null
  createdAt: string
}

/** What a tool note says, for kids and parents alike. */
export function toolNoteText(tool: string, detail: string): string {
  switch (tool) {
    case 'weather':
      return 'Checked the weather'
    case 'my_day':
      return 'Looked at your day'
    case 'my_picks':
      return 'Looked at your picks'
    case 'search':
      return `Searched: ${detail}`
    default:
      return `Used ${tool}`
  }
}

export interface Transcript {
  conversation: Conversation
  messages: ChatMessage[]
  /** The tools the agent used, in order. */
  toolCalls: ToolCall[]
  /** Parents only. */
  flags?: ChatFlag[]
}

/** GET /api/me/chats: whether chat is on, and the kid's conversations. */
export interface ChatList {
  available: boolean
  conversations: Conversation[]
}

export interface Alerts {
  unreviewed: number
  crises: { flagId: number; memberId: number; conversationId: number; createdAt: string }[]
}

/** What each flag category is called on screen. */
export const categoryNames: Record<string, string> = {
  self_harm: 'Self-harm or suicide',
  abuse: 'Abuse or danger at home',
  bullying: 'Bullying',
  sexual: 'Sexual content',
  violence: 'Violence or weapons',
  drugs: 'Drugs or alcohol',
  strangers: 'Strangers or personal info',
  hate: 'Hate',
  other: 'Other concern',
  couldnt_check: 'Couldn’t check',
  blocked: 'Refused by the AI’s filter',
}

export const modeNames: Record<ChatMode, string> = { homework: 'Homework help', explore: 'Explore' }

// --- Kids ---

export const chatsQueryOptions = queryOptions<ChatList, ApiError>({
  queryKey: ['chats'],
  queryFn: ({ signal }) => apiFetch<ChatList>('/api/me/chats', { signal }),
})

/** The kid's conversations, and whether chat is on. */
export function useChats() {
  return useQuery(chatsQueryOptions)
}

export function chatQueryOptions(chatId: number) {
  return queryOptions<Transcript, ApiError>({
    queryKey: ['chats', chatId],
    queryFn: ({ signal }) => apiFetch<Transcript>(`/api/me/chats/${chatId}`, { signal }),
  })
}

/** One of the kid's conversations, with its messages. */
export function useChat(chatId: number) {
  return useQuery(chatQueryOptions(chatId))
}

/** Starts a conversation; resolves with it. */
export function useStartChat() {
  const queryClient = useQueryClient()
  return useMutation<Conversation, ApiError, ChatMode>({
    mutationFn: (mode) => apiFetch<Conversation>('/api/me/chats', { method: 'POST', body: { mode } }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: chatsQueryOptions.queryKey, exact: true }),
  })
}

/** What sendChatMessage reports as the reply streams in. */
export interface StreamHandlers {
  /** The kid's message, as stored. */
  onMessage: (message: ChatMessage) => void
  /** A tool the agent used, as it's used. */
  onTool: (tool: string, detail: string) => void
  /** A piece of the reply. */
  onDelta: (text: string) => void
  /** The whole reply, as stored. */
  onDone: (message: ChatMessage) => void
}

/**
 * Sends a kid's message and streams the reply. The server answers with
 * server-sent events, read here from the fetch body (EventSource can't
 * POST). A request refused before the message is stored rejects with an
 * ApiError; a reply that fails partway rejects with one too, after the
 * pieces that did arrive were reported.
 */
export async function sendChatMessage(chatId: number, text: string, handlers: StreamHandlers, signal?: AbortSignal) {
  let res: Response
  try {
    res = await fetch(`/api/me/chats/${chatId}/messages`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
      // The kid's time zone, so "today" in their calendar is their today.
      body: JSON.stringify({ text, timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone }),
      signal,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, "Couldn't reach the Porchlight server.", 'Check your connection, then try again.')
  }
  if (!res.ok || !res.headers.get('Content-Type')?.startsWith('text/event-stream')) {
    const body = (await res.json().catch(() => null)) as { error?: string; hint?: string } | null
    throw new ApiError(res.status, body?.error ?? 'Something went wrong.', body?.hint ?? 'Try again in a moment.')
  }
  if (!res.body) throw new ApiError(0, "The server's answer was empty.", 'Try again in a moment.')

  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
  let buffer = ''
  let finished = false
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += value
    // Events end with a blank line; keep any partial one for next time.
    let end: number
    while ((end = buffer.indexOf('\n\n')) >= 0) {
      const raw = buffer.slice(0, end)
      buffer = buffer.slice(end + 2)
      const name = /^event: (.*)$/m.exec(raw)?.[1]
      const data = /^data: (.*)$/m.exec(raw)?.[1]
      if (!name || data === undefined) continue
      const payload: unknown = JSON.parse(data)
      switch (name) {
        case 'message':
          handlers.onMessage(payload as ChatMessage)
          break
        case 'tool': {
          const note = payload as { tool: string; detail: string }
          handlers.onTool(note.tool, note.detail)
          break
        }
        case 'delta':
          handlers.onDelta((payload as { text: string }).text)
          break
        case 'done':
          finished = true
          handlers.onDone(payload as ChatMessage)
          break
        case 'error': {
          const err = payload as { error: string; hint?: string }
          throw new ApiError(200, err.error, err.hint ?? '')
        }
      }
    }
  }
  if (!finished) throw new ApiError(0, 'The answer stopped partway.', 'Try sending your message again.')
}

// --- Parents ---

export function memberChatsQueryOptions(memberId: number) {
  return queryOptions<{ conversations: Conversation[] }, ApiError>({
    queryKey: ['members', memberId, 'chats'],
    queryFn: ({ signal }) => apiFetch(`/api/members/${memberId}/chats`, { signal }),
  })
}

/** A kid's conversations, for a parent. */
export function useMemberChats(memberId: number) {
  return useQuery(memberChatsQueryOptions(memberId))
}

/** One of a kid's conversations, with messages and flags, for a parent. */
export function useMemberTranscript(memberId: number, chatId: number) {
  return useQuery(
    queryOptions<Transcript, ApiError>({
      queryKey: ['members', memberId, 'chats', chatId],
      queryFn: ({ signal }) => apiFetch<Transcript>(`/api/members/${memberId}/chats/${chatId}`, { signal }),
    }),
  )
}

/** Deletes one of a kid's conversations (a parent, after confirming). */
export function useDeleteChat(memberId: number) {
  const queryClient = useQueryClient()
  return useMutation<void, ApiError, number>({
    mutationFn: (chatId) => apiFetch<void>(`/api/members/${memberId}/chats/${chatId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['members', memberId, 'chats'] })
      void queryClient.invalidateQueries({ queryKey: reviewsQueryOptions.queryKey })
      void queryClient.invalidateQueries({ queryKey: alertsQueryOptions.queryKey })
    },
  })
}

export const reviewsQueryOptions = queryOptions<{ flags: ChatFlag[] }, ApiError>({
  queryKey: ['reviews'],
  queryFn: ({ signal }) => apiFetch('/api/family/reviews', { signal }),
  refetchOnWindowFocus: true,
})

/** The family's review queue: unreviewed flags first, crises first among them. */
export function useReviews() {
  return useQuery(reviewsQueryOptions)
}

/** Marks a flag as reviewed. */
export function useMarkReviewed() {
  const queryClient = useQueryClient()
  return useMutation<ChatFlag, ApiError, number>({
    mutationFn: (flagId) => apiFetch<ChatFlag>(`/api/family/reviews/${flagId}/reviewed`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: reviewsQueryOptions.queryKey })
      void queryClient.invalidateQueries({ queryKey: alertsQueryOptions.queryKey })
      void queryClient.invalidateQueries({ queryKey: ['members'], predicate: (q) => q.queryKey.includes('chats') })
    },
  })
}

export const alertsQueryOptions = queryOptions<Alerts, ApiError>({
  queryKey: ['alerts'],
  queryFn: ({ signal }) => apiFetch<Alerts>('/api/family/alerts', { signal }),
  // A possible crisis must not wait for a reload: ask every minute, and
  // whenever the parent comes back to the tab. It's a cheap request.
  refetchInterval: 60_000,
  refetchOnWindowFocus: true,
})

/** The family's alerts; only for parents (pass enabled: false for kids). */
export function useAlerts(enabled: boolean) {
  return useQuery({ ...alertsQueryOptions, enabled })
}

/** A kid's birth year, for a parent: null when it isn't set. */
export function useBirthYear(memberId: number) {
  return useQuery(
    queryOptions<{ birthYear: number | null }, ApiError>({
      queryKey: ['members', memberId, 'birth-year'],
      queryFn: ({ signal }) => apiFetch(`/api/members/${memberId}/birth-year`, { signal }),
    }),
  )
}

/** Sets a kid's birth year (null clears it). */
export function useSetBirthYear(memberId: number) {
  const queryClient = useQueryClient()
  return useMutation<{ birthYear: number | null }, ApiError, number | null>({
    mutationFn: (birthYear) =>
      apiFetch(`/api/members/${memberId}/birth-year`, { method: 'PUT', body: { birthYear } }),
    onSuccess: (data) => queryClient.setQueryData(['members', memberId, 'birth-year'], data),
  })
}
