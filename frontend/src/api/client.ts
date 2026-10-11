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

/** A widget's content (`widgets.Widget` in the Go backend). */
export interface Widget {
  title: string
  source: SourceName
  query: string
  archetype: Archetype
  /** May be empty when the search found nothing. */
  items: WidgetItem[]
}

/**
 * A widget saved to the signed-in person's page: what POST
 * /api/widgets/from-prompt and GET /api/widgets/custom/{id} return, and each
 * entry of Homepage.customWidgets.
 */
export interface SavedWidget extends Widget {
  id: number
  /** RFC 3339 timestamp, e.g. "2026-10-09T21:14:03Z". */
  createdAt: string
}

/**
 * Why a curated widget came back empty without searching: the member hasn't
 * told us what it needs yet. "" (or missing) means it did search.
 */
export type EmptyReason = '' | 'no_interests' | 'no_location'

/**
 * A widget Porchlight picks for you from your interests and location: what
 * GET /api/widgets/articles and GET /api/widgets/events return. It's
 * recomputed on every request, so it has no id.
 */
export interface CuratedWidget extends Widget {
  emptyReason?: EmptyReason
}

/** What a member has told us about themselves; seeds their curated widgets. */
export interface Profile {
  /** Free text, e.g. "space launches, soccer, gardening". */
  interests: string
  /** Free text; a city is fine. */
  location: string
}

/** Response of PUT /api/me/profile (whose body is a Profile). */
export interface ProfileResponse {
  profile: Profile
}

/** Response of GET /api/me (and of signup, login and create-family). */
export interface Me {
  user: { id: number; email: string; isAdmin: boolean }
  family: { id: number; name: string }
  /** True until the create-family intro is done (the family has no name yet). */
  needsFamilySetup: boolean
  profile: Profile
}

/** Body of POST /api/signup and POST /api/login. */
export interface Credentials {
  email: string
  password: string
}

/** Body of POST /api/families: the create-family intro. */
export interface CreateFamilyInput {
  /** 1–80 characters. */
  name: string
  /** Optional; each one becomes a pending co-admin invite. */
  coAdminEmails: string[]
  /** Up to 500 characters each; may be empty. */
  interests: string
  location: string
}

/** Response of GET /api/me/homepage. */
export interface Homepage {
  profile: Profile
  /** Always null for now; the saved page layout arrives with drag-and-drop. */
  layout: null
  /** Oldest first. */
  customWidgets: SavedWidget[]
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

/**
 * Endpoints whose 401 is an answer, not an expired session: /api/me says
 * "nobody is logged in", and login/signup say "wrong email or password".
 */
const SESSION_ENDPOINTS = new Set(['/api/me', '/api/login', '/api/signup'])

let onUnauthorized: (() => void) | undefined

/**
 * Registers what to do when the server says the session is gone (a 401 from
 * any other endpoint). The app shell sends the person to /login; keeping that
 * outside this module means the client doesn't need to know about routing.
 */
export function setUnauthorizedHandler(handler: () => void) {
  onUnauthorized = handler
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

  if (res.status === 401 && !SESSION_ENDPOINTS.has(path.split('?')[0])) {
    // "Session expired" (spec > Important Failure Modes). Still throw below,
    // so the caller's own error handling runs as usual.
    onUnauthorized?.()
  }

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

  if (res.status === 404) {
    // Porchlight's own 404s are JSON (handled above). A plain-text one means
    // the backend has no such endpoint: usually an older build still running.
    return new ApiError(
      404,
      "The backend doesn't know this request.",
      'It may be an older version. Stop it and run “make run” again in backend/.',
    )
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
