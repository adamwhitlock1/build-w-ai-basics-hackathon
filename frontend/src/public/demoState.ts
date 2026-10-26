export interface Allowance { modelsRemaining: number; searchesRemaining: number; reset: string }
export function demoPaused(quota: Allowance | undefined, now = Date.now()): boolean {
  if (!quota || Date.parse(quota.reset) <= now) return false
  return quota.modelsRemaining === 0 || quota.searchesRemaining === 0
}
/** Whether an error is the judge demo's spent (or unreadable) allowance, by the server's code. */
export function isDemoLimit(err: { code?: string }): boolean {
  return err.code === 'demo_limit'
}
