import { useEffect, useState } from 'react'

const MINUTE = 60_000
const HOUR = 60 * MINUTE

const shortDate = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })

/**
 * How long ago an RFC 3339 time was, in words for a card: "just now",
 * "5 min ago", "2h ago", "yesterday", or a date ("Oct 9") beyond that. A
 * time a little in the future (the browser's clock running behind the
 * server's) reads as "just now".
 */
export function formatUpdatedAgo(updatedAt: string, now: number): string {
  const then = Date.parse(updatedAt)
  if (Number.isNaN(then)) return ''
  const ago = now - then
  if (ago < MINUTE) return 'just now'
  if (ago < HOUR) return `${Math.floor(ago / MINUTE)} min ago`
  if (ago < 24 * HOUR) return `${Math.floor(ago / HOUR)}h ago`
  if (ago < 48 * HOUR) return 'yesterday'
  return shortDate.format(then)
}

/**
 * The current time in milliseconds, updated every `intervalMs`, so labels
 * like "5 min ago" age while the page stays open.
 */
export function useNow(intervalMs: number): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs)
    return () => window.clearInterval(id)
  }, [intervalMs])
  return now
}
