/**
 * Returns the URL if it is an absolute http(s) URL, otherwise undefined.
 * Widget content comes from search results shaped by an LLM, so anything
 * else (javascript:, data:, relative paths) is never rendered as a link or image.
 */
export function safeUrl(raw: string): string | undefined {
  if (!raw) return undefined
  try {
    const url = new URL(raw)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.href : undefined
  } catch {
    return undefined
  }
}
