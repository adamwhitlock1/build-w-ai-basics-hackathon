import { queryOptions, useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'

export interface PublicConfig {
  demo: boolean
  demoUrl: string
  videoUrl: string
  quota?: { modelsRemaining: number; searchesRemaining: number; reset: string }
}
export const publicConfigOptions = queryOptions({
  queryKey: ['public-config'],
  queryFn: () => apiFetch<PublicConfig>('/api/public-config'),
  staleTime: 15_000,
  retry: false,
})
export function usePublicConfig() {
  return useQuery({ ...publicConfigOptions, refetchInterval: (q) => q.state.data?.demo ? 15_000 : false })
}
