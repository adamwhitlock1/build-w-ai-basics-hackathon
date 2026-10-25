import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch, type ApiError } from './client'
import {
  choreKey,
  midnightDelay,
  completionBody,
  type ChoreDay,
  type Checklist,
  type ChoreSummary,
} from '../chores/model'

type Received<T> = T & { receivedAt: number }
function useDailyQuery<T extends ChoreDay>(viewer: number, subject: number | 'family', path: string) {
  const query = useQuery<Received<T>, ApiError>({
    queryKey: choreKey(viewer, subject),
    queryFn: async ({ signal }) => {
      const startedAt = performance.now()
      const data = await apiFetch<T>(path, { signal })
      // Counting request time too errs toward reloading early, never toward stale writes.
      return { ...data, receivedAt: startedAt }
    },
    networkMode: 'always',
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: 'always',
    retry: false,
    staleTime: 0,
  })
  const [expiredAt, setExpiredAt] = useState<number | null>(null)
  const { data, refetch } = query
  useEffect(() => {
    if (!data) return
    const remaining = () => midnightDelay(data, performance.now() - data.receivedAt)
    const timer = window.setTimeout(() => {
      setExpiredAt(data.receivedAt)
      if (document.visibilityState === 'visible') void refetch()
    }, remaining() + 50)
    const resume = () => {
      if (document.visibilityState !== 'visible') return
      if (remaining() <= 0) setExpiredAt(data.receivedAt)
      void refetch()
    }
    document.addEventListener('visibilitychange', resume)
    window.addEventListener('focus', resume)
    return () => {
      clearTimeout(timer)
      document.removeEventListener('visibilitychange', resume)
      window.removeEventListener('focus', resume)
    }
  }, [data, refetch])
  return { ...query, expired: Boolean(data && expiredAt === data.receivedAt) }
}
export function useChores(viewer: number, subject: number) {
  const own = viewer === subject
  return useDailyQuery<Checklist>(viewer, subject, own ? '/api/me/chores' : `/api/members/${subject}/chores`)
}
export function useChoreSummary(viewer: number) {
  return useDailyQuery<ChoreSummary>(viewer, 'family', '/api/family/chores')
}
export type ChoreWrite = {
  operation: 'create' | 'rename' | 'remove' | 'completion'
  id?: number
  title?: string
  date?: string
  completed?: boolean
}
export function useSaveChore(viewer: number, subject: number) {
  const client = useQueryClient()
  return useMutation<Received<Checklist>, ApiError, ChoreWrite>({
    retry: false,
    networkMode: 'always',
    mutationFn: async ({ operation, id, title, date, completed }) => {
      const startedAt = performance.now()
      const base =
        operation === 'completion' && viewer === subject ? '/api/me/chores' : `/api/members/${subject}/chores`
      const path =
        operation === 'create' ? base : `${base}/${id}${operation === 'completion' ? '/completion' : ''}`
      const data = await apiFetch<Checklist>(path, {
        method: operation === 'create' ? 'POST' : operation === 'remove' ? 'DELETE' : 'PUT',
        body:
          operation === 'completion'
            ? completionBody(date!, completed!)
            : operation === 'remove'
              ? undefined
              : { title },
      })
      return { ...data, receivedAt: startedAt }
    },
    onSuccess: async (data) => {
      await client.cancelQueries({ queryKey: choreKey(viewer, subject) })
      client.setQueryData(choreKey(viewer, subject), data)
    },
    onSettled: async () => {
      // An ambiguous network failure may have committed. Re-read, never replay it.
      await client.invalidateQueries({ queryKey: ['chores', viewer] })
    },
  })
}
