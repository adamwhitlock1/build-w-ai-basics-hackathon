// Typed API client for the Porchlight Go backend. Every request goes through
// apiFetch so headers, credentials and error handling live in one place.

/** How a widget lays out its items. Mirrors the backend contract. */
export type Archetype = 'list' | 'cards' | 'hero' | 'key_value' | 'table'

/** Which search vertical the backend used to fill the widget. */
export type SourceName = 'news_search' | 'events_search' | 'web_search'

/** One row of widget content. Every field is always a string ("" when unknown). */
export interface WidgetItem {
  title: string
  url: string
  snippet: string
  date: string
  label: string
  value: string
  thumbnail: string
}

/** Response of POST /api/widgets/from-prompt (`widgets.Widget` in the Go backend). */
export interface Widget {
  title: string
  source: SourceName
  query: string
  archetype: Archetype
  /** May be empty when the search found nothing. */
  items: WidgetItem[]
}

/**
 * Every failure from apiFetch (other than a caller's own abort) is an
 * ApiError, so callers can rely on `message` (what went wrong) and `hint`
 * (what to do next). `status` is the HTTP status, or 0 when the server
 * couldn't be reached.
 */
export class ApiError extends Error {
  readonly status: number
  readonly hint: string

  constructor(status: number, message: string, hint = '') {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.hint = hint
  }
}

/** Like RequestInit, but `body` is any JSON-serialisable value. */
export type ApiRequestInit = Omit<RequestInit, 'body'> & { body?: unknown }

/** Sends a JSON request to the API and returns the parsed JSON response. */
export async function apiFetch<T>(path: string, init: ApiRequestInit = {}): Promise<T> {
  const { body, headers: callerHeaders, ...rest } = init
  const hasBody = body !== undefined

  // new Headers() accepts every HeadersInit shape (object, array, Headers).
  const headers = new Headers(callerHeaders)
  if (!headers.has('Accept')) headers.set('Accept', 'application/json')
  if (hasBody && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')

  let res: Response
  try {
    res = await fetch(path, {
      ...rest,
      credentials: 'same-origin',
      headers,
      body: hasBody ? JSON.stringify(body) : undefined,
    })
  } catch (err) {
    // A caller-initiated abort (e.g. TanStack Query cancelling) isn't a failure.
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    // Otherwise fetch only rejects on network failure (server down, offline).
    throw new ApiError(
      0,
      "Couldn't reach the Porchlight server.",
      'Check that the backend is running, then try again.',
    )
  }

  // Slice 2: a 401 here will redirect to /login.

  if (!res.ok) {
    throw await toApiError(res)
  }

  if (res.status === 204) {
    return undefined as T
  }

  try {
    return (await res.json()) as T
  } catch {
    throw new ApiError(res.status, "The server sent a response we couldn't read.", 'Try again in a moment.')
  }
}

/** Turns a non-2xx response into an ApiError, preferring the server's {error, hint}. */
async function toApiError(res: Response): Promise<ApiError> {
  const text = await res.text().catch(() => '')
  try {
    const parsed: unknown = JSON.parse(text)
    if (isErrorBody(parsed)) {
      return new ApiError(res.status, parsed.error, parsed.hint ?? '')
    }
  } catch {
    // Not JSON (e.g. the dev proxy's own error page) — fall through.
  }

  if (res.status >= 500) {
    return new ApiError(
      res.status,
      'The server had trouble with that request.',
      'Try again in a moment. If it keeps happening, check that the backend is running.',
    )
  }
  return new ApiError(res.status, `Something went wrong (HTTP ${res.status}).`, 'Try again in a moment.')
}

function isErrorBody(v: unknown): v is { error: string; hint?: string } {
  if (typeof v !== 'object' || v === null) return false
  const body = v as Record<string, unknown>
  return (
    typeof body.error === 'string' &&
    body.error !== '' &&
    (body.hint === undefined || typeof body.hint === 'string')
  )
}
