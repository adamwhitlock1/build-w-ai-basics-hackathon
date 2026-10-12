// Typed API client for the Porchlight Go backend. Every request goes through
// apiFetch so headers, credentials and error handling live in one place.

/** How a widget lays out its items. Mirrors the backend contract. */
export type Archetype = 'list' | 'cards' | 'hero' | 'key_value' | 'table'

/** Which source the backend used to fill the widget: a search vertical, or the weather forecast. */
export type SourceName = 'news_search' | 'events_search' | 'web_search' | 'weather_forecast'

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
  /**
   * Searched with strict safe-search and chosen for someone under 18,
   * because the member it's for is a kid. Widgets saved before this existed
   * say false.
   */
  kidSafe: boolean
}

/**
 * When a widget's items were last updated, and whether they're old enough
 * to fetch new ones (`widgets.Freshness` in the Go backend). The server
 * decides staleness, so freshness windows live in one place.
 */
export interface Freshness {
  /** RFC 3339 timestamp, e.g. "2026-10-09T21:14:03Z". */
  updatedAt: string
  /** Past its freshness window: still shown, but due for a refresh. */
  stale: boolean
}

/**
 * A widget saved to the signed-in person's page: what POST
 * /api/widgets/from-prompt, GET /api/widgets/custom/{id} and POST
 * /api/widgets/custom/{id}/refresh return, and each entry of
 * Homepage.customWidgets.
 */
export interface SavedWidget extends Widget, Freshness {
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
 * GET /api/widgets/articles and GET /api/widgets/events return. The server
 * remembers each person's latest one (so it has no id of its own) and picks
 * again when asked with `?refresh=1` or when their interests change.
 */
export interface CuratedWidget extends Widget, Freshness {
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

/** Response of GET /api/me (and of signup, login, create-family and accepting an invite). */
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

/** Someone in the family, as GET /api/me/family lists them. */
export interface FamilyMember {
  id: number
  email: string
  /** A parent: can invite and manage the family. */
  isAdmin: boolean
  /** True for the logged-in person's own entry. */
  isYou: boolean
}

/** An invite nobody has accepted yet (and that hasn't expired). */
export interface PendingInvite {
  id: number
  email: string
  /** Invited as a parent (true) or as a kid (false). */
  isAdmin: boolean
  /** The invite link as a path, "/invite/<token>"; put the site's origin in front to share it. */
  url: string
  /** RFC 3339 timestamp; the link stops working after this. */
  expiresAt: string
}

/**
 * Response of GET /api/me/family. Parents see everyone and the pending
 * invites; kids see only themselves, and no invites.
 */
export interface FamilyOverview {
  family: { id: number; name: string }
  /** Parents first, then in the order they joined. */
  members: FamilyMember[]
  pendingInvites: PendingInvite[]
}

/** Who an invite is for: a parent can invite and manage, a kid has a page of their own. */
export type InviteRole = 'parent' | 'kid'

/** Body of POST /api/families/{id}/invites. */
export interface CreateInviteInput {
  email: string
  role: InviteRole
}

/**
 * Response of POST /api/families/{id}/invites: a new invite (201), or the
 * one that email already had (200, with the role updated).
 */
export interface InviteResponse {
  invite: PendingInvite
}

/** Response of GET /api/invites/{token}: what the invite page needs to greet someone. */
export interface InviteDetails {
  /** The invited email; it becomes their login. */
  email: string
  familyName: string
  /** Invited as a parent. */
  isAdmin: boolean
}

/** Body of POST /api/invites/{token}/accept, which responds with Me. */
export interface AcceptInviteInput {
  /** At least 8 characters, like at sign-up. */
  password: string
}

/** How many of the page's four columns a card spans: 1/4 up to 4/4. */
export type Span = 1 | 2 | 3 | 4

/** What every card on the page carries. */
interface BlockProps {
  /** Stable and unique on the page: "articles", "events" or "custom-<widgetId>". */
  id: string
  span: Span
}

/**
 * One card on the page. The type decides what it shows; a Custom card also
 * says which saved widget (SavedWidget.id) it is.
 */
export type LayoutBlock =
  | { type: 'Articles'; props: BlockProps }
  | { type: 'LocalEvents'; props: BlockProps }
  | { type: 'Custom'; props: BlockProps & { widgetId: number } }

/** The kinds of card a page can hold: 'Articles', 'LocalEvents' or 'Custom'. */
export type BlockType = LayoutBlock['type']

/**
 * A page layout, stored by the server exactly as Puck (the drag-and-drop
 * editor) reads and writes it: the cards live in the root's `blocks` slot,
 * and `content` and `zones` stay empty.
 */
export interface Layout {
  root: { props: { blocks: LayoutBlock[] } }
  content: []
  zones: Record<string, never>
}

/** Response of GET /api/me/homepage. */
export interface Homepage {
  profile: Profile
  /**
   * The cards and their order and widths. Never null: the server builds a
   * default for a page that hasn't been arranged yet.
   */
  layout: Layout
  /** Oldest first. */
  customWidgets: SavedWidget[]
}

/** Body of PUT /api/me/homepage, and its response (the layout as the server stored it). */
export interface LayoutResponse {
  layout: Layout
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
