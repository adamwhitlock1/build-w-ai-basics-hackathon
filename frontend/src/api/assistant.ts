// The parents' Assistant: its API shapes and TanStack Query hooks. Replies
// stream through streamMessage, as in kid chat.

import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ChatMessage, StreamHandlers, ToolCall } from './chat'
import { streamMessage } from './chat'
import { ApiError, apiFetch } from './client'

export type AssistantKind = 'anything' | 'kids'

export interface AssistantConversation {
  id: number
  kind: AssistantKind
  /** "" until the first message. */
  title: string
  createdAt: string
  updatedAt: string
}

export interface AssistantList {
  conversations: AssistantConversation[]
  /** Whether "About the kids" can be started (kid chat is set up). */
  kidsAvailable: boolean
}

export interface AssistantTranscript {
  conversation: AssistantConversation
  messages: ChatMessage[]
  toolCalls: ToolCall[]
}

export const kindNames: Record<AssistantKind, string> = { anything: 'Ask anything', kids: 'About the kids' }

export const assistantListQueryOptions = queryOptions<AssistantList, ApiError>({
  queryKey: ['assistant'],
  queryFn: ({ signal }) => apiFetch<AssistantList>('/api/me/assistant', { signal }),
})

/** The parent's Assistant conversations. */
export function useAssistantList() {
  return useQuery(assistantListQueryOptions)
}

export function assistantQueryOptions(id: number) {
  return queryOptions<AssistantTranscript, ApiError>({
    queryKey: ['assistant', id],
    queryFn: ({ signal }) => apiFetch<AssistantTranscript>(`/api/me/assistant/${id}`, { signal }),
  })
}

/** One Assistant conversation. */
export function useAssistantConversation(id: number) {
  return useQuery(assistantQueryOptions(id))
}

/** Starts a conversation of kind. */
export function useStartAssistant() {
  const queryClient = useQueryClient()
  return useMutation<AssistantConversation, ApiError, AssistantKind>({
    mutationFn: (kind) => apiFetch<AssistantConversation>('/api/me/assistant', { method: 'POST', body: { kind } }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: assistantListQueryOptions.queryKey, exact: true }),
  })
}

/** Deletes one of the parent's conversations. */
export function useDeleteAssistant() {
  const queryClient = useQueryClient()
  return useMutation<void, ApiError, number>({
    mutationFn: (id) => apiFetch<void>(`/api/me/assistant/${id}`, { method: 'DELETE' }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: assistantListQueryOptions.queryKey, exact: true }),
  })
}

/** Sends a message to conversation id and streams the reply. */
export function sendAssistantMessage(id: number, text: string, handlers: StreamHandlers, signal?: AbortSignal) {
  return streamMessage(`/api/me/assistant/${id}/messages`, text, handlers, signal)
}
