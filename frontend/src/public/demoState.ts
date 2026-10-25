export interface Allowance { modelsRemaining: number; searchesRemaining: number; reset: string }
export function demoPaused(quota: Allowance | undefined, now = Date.now()): boolean {
  if (!quota || Date.parse(quota.reset) <= now) return false
  return quota.modelsRemaining === 0 || quota.searchesRemaining === 0
}
export function isDemoLimit(message: string): boolean {
  return message.includes('Demo allowance reached.') || message.includes('Demo allowance unavailable.')
}
