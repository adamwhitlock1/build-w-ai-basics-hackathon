import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  useSuspenseQuery,
  type QueryClient,
} from '@tanstack/react-query'
import { useRouter, type RegisteredRouter } from '@tanstack/react-router'
import { layoutOf, withCustomBlock, withoutCustomBlock } from '../puck/layout'
import {
  ApiError,
  apiFetch,
  type CreateFamilyInput,
  type Credentials,
  type CuratedWidget,
  type Homepage,
  type Layout,
  type LayoutResponse,
  type Me,
  type Profile,
  type ProfileResponse,
  type SavedWidget,
} from './client'

// ---------------------------------------------------------------------------
// Who's logged in
// ---------------------------------------------------------------------------

/**
 * The logged-in person, or null when nobody is.
 *
 * `staleTime: 'static'` means "never refetch this on your own": the session
 * only changes when this app changes it, and every hook below that does so
 * writes the new answer straight into the cache. Route guards read it with
 * `queryClient.query(meQueryOptions)` — with 'static', that is TanStack Query
 * v5's replacement for the deprecated `ensureQueryData`: the cached answer if
 * there is one, otherwise a single request.
 */
export const meQueryOptions = queryOptions({
  queryKey: ['me'],
  queryFn: async ({ signal }): Promise<Me | null> => {
    try {
      return await apiFetch<Me>('/api/me', { signal })
    } catch (err) {
      // From /api/me, a 401 is an answer ("nobody is logged in"), not a failure.
      if (err instanceof ApiError && err.status === 401) return null
      throw err
    }
  },
  staleTime: 'static',
})

/** The logged-in person (null when nobody is). Route guards load it first. */
export function useMe() {
  return useQuery(meQueryOptions)
}

/** Starts from a clean cache for the person who just logged in or signed up. */
function startSession(queryClient: QueryClient, me: Me) {
  // Nothing cached for someone else may show up on this person's pages.
  queryClient.removeQueries()
  queryClient.setQueryData(meQueryOptions.queryKey, me)
}

/**
 * Forgets the logged-in person on this device and shows the login screen.
 * Runs after logging out, and whenever the server says the session expired.
 */
export async function endSession(queryClient: QueryClient, router: RegisteredRouter) {
  // Record "nobody is logged in" first, so /login doesn't bounce back to /.
  queryClient.setQueryData(meQueryOptions.queryKey, null)
  await router.navigate({ to: '/login' })
  // Now that no page is showing them, drop everything cached for that person.
  queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'me' })
}

/** Creates an account (and its family) and logs it in. Call `mutate({ email, password })`. */
export function useSignup() {
  return useStartSession('/api/signup')
}

/** Logs in. A wrong email or password fails with a 401 ApiError. */
export function useLogin() {
  return useStartSession('/api/login')
}

function useStartSession(path: '/api/signup' | '/api/login') {
  const queryClient = useQueryClient()
  return useMutation<Me, ApiError, Credentials>({
    mutationFn: (credentials) => apiFetch<Me>(path, { method: 'POST', body: credentials }),
    onSuccess: (me) => startSession(queryClient, me),
  })
}

/** Logs out on the server, then on this device, and shows the login screen. */
export function useLogout() {
  const queryClient = useQueryClient()
  const router = useRouter()
  return useMutation<void, ApiError, void>({
    mutationFn: () => apiFetch<void>('/api/logout', { method: 'POST' }),
    onSuccess: () => endSession(queryClient, router),
  })
}

// ---------------------------------------------------------------------------
// Family
// ---------------------------------------------------------------------------

/** Completes the create-family intro. Resolves with the updated Me. */
export function useCreateFamily() {
  const queryClient = useQueryClient()
  return useMutation<Me, ApiError, CreateFamilyInput>({
    mutationFn: (input) => apiFetch<Me>('/api/families', { method: 'POST', body: input }),
    onSuccess: (me) => {
      queryClient.setQueryData(meQueryOptions.queryKey, me)
      // The profile shown with the homepage just changed.
      void queryClient.invalidateQueries({ queryKey: homepageQueryOptions.queryKey })
    },
    onError: (err) => {
      // 409: the family is already set up (say, from another tab), so the
      // cached Me is out of date. Drop it; the next page asks the server.
      if (err.status === 409) void queryClient.resetQueries({ queryKey: meQueryOptions.queryKey })
    },
  })
}

// ---------------------------------------------------------------------------
// Your page
// ---------------------------------------------------------------------------

/**
 * The logged-in person's page: their profile, the layout of their cards, and
 * their saved widgets, oldest first. It only changes through this app, which
 * updates the cache as it goes, so it's never refetched in the background
 * (but can still be invalidated).
 */
export const homepageQueryOptions = queryOptions({
  queryKey: ['homepage'],
  queryFn: ({ signal }) => apiFetch<Homepage>('/api/me/homepage', { signal }),
  staleTime: Infinity,
})

/**
 * The homepage data. Use it under a route whose loader has already loaded
 * `homepageQueryOptions`; otherwise it suspends until the data arrives.
 */
export function useHomepage() {
  return useSuspenseQuery(homepageQueryOptions)
}

/**
 * Identifies one person's widget-creation mutations, e.g. for `useIsMutating`,
 * so a build someone else started on this device doesn't show as yours.
 */
export function createWidgetKey(userId: number | undefined) {
  return ['widgets', 'from-prompt', userId] as const
}

interface CreateWidgetOptions {
  /**
   * Called with the new widget. Unlike a callback passed to `mutate`, this one
   * still runs if the calling component unmounts before the server answers,
   * so a widget the server built is never dropped.
   */
  onCreated?: (widget: SavedWidget) => void
}

/**
 * Turns a plain-language prompt into a live widget, saved to your page.
 * Call `mutate(prompt)`; the server trims and validates it (1–500 chars).
 */
export function useCreateWidgetFromPrompt({ onCreated }: CreateWidgetOptions = {}) {
  const queryClient = useQueryClient()
  const { data: me } = useMe()
  // apiFetch normalises every failure into an ApiError, so the error type is
  // exact. The fourth type is what onMutate returns: who asked for the widget.
  return useMutation<SavedWidget, ApiError, string, { userId: number | undefined }>({
    mutationKey: createWidgetKey(me?.user.id),
    // A build can take a while, and on a shared computer someone else may
    // log in before it finishes, so note who asked.
    onMutate: () => ({ userId: queryClient.getQueryData(meQueryOptions.queryKey)?.user.id }),
    mutationFn: (prompt) =>
      apiFetch<SavedWidget>('/api/widgets/from-prompt', {
        method: 'POST',
        body: { prompt },
      }),
    onSuccess: (widget, _prompt, { userId }) => {
      // The server saved it to the person who asked. If they've logged out
      // since, it isn't for whoever's page is showing now.
      if (queryClient.getQueryData(meQueryOptions.queryKey)?.user.id !== userId) return
      // Show it straight away: add it to the cached page, with a card at the
      // end of the layout, just as the server did.
      queryClient.setQueryData(homepageQueryOptions.queryKey, (page) => {
        if (!page) return page
        const known = page.customWidgets.some((w) => w.id === widget.id)
        return {
          ...page,
          customWidgets: known ? page.customWidgets : [...page.customWidgets, widget],
          layout: withCustomBlock(layoutOf(page), widget.id),
        }
      })
      // Then ask for the page again, so this copy matches what the server
      // saved. The new card stays on screen meanwhile.
      void queryClient.invalidateQueries({ queryKey: homepageQueryOptions.queryKey })
      onCreated?.(widget)
    },
  })
}

/**
 * Deletes one of your saved widgets for good. Call `mutateAsync(widgetId)`.
 * The server also takes its card off your saved layout; on success the
 * widget and its card leave the cached page too.
 *
 * (Leaving a card out of a saved layout isn't enough: the server gives every
 * saved widget a card, so the widget would come back on the next load.)
 */
export function useDeleteWidget() {
  const queryClient = useQueryClient()
  return useMutation<void, ApiError, number>({
    mutationFn: (widgetId) => apiFetch<void>(`/api/widgets/custom/${widgetId}`, { method: 'DELETE' }),
    onSuccess: (_nothing, widgetId) => {
      queryClient.setQueryData(homepageQueryOptions.queryKey, (page) =>
        page
          ? {
              ...page,
              customWidgets: page.customWidgets.filter((w) => w.id !== widgetId),
              layout: withoutCustomBlock(layoutOf(page), widgetId),
            }
          : page,
      )
    },
  })
}

// ---------------------------------------------------------------------------
// Arranging your page
// ---------------------------------------------------------------------------

/**
 * Saves the order and widths of your cards. Resolves with the layout as the
 * server stored it (it keeps only the fields it knows). Called by the Arrange
 * editor's autosave (puck/useLayoutAutosave.ts), which decides when to save,
 * so this is a plain function rather than a mutation hook.
 */
export async function saveLayout(layout: Layout): Promise<Layout> {
  const res = await apiFetch<LayoutResponse>('/api/me/homepage', { method: 'PUT', body: { layout } })
  return res.layout
}

/** Puts a saved layout into the cached page, so the page shows it without asking the server again. */
export function setCachedLayout(queryClient: QueryClient, layout: Layout) {
  queryClient.setQueryData(homepageQueryOptions.queryKey, (page) => (page ? { ...page, layout } : page))
}

// ---------------------------------------------------------------------------
// Widgets picked for you
// ---------------------------------------------------------------------------

/** The two widgets Porchlight curates from a member's interests and location. */
export type CuratedKind = 'articles' | 'events'

/** Query key of one person's curated widget: ['widgets', kind, userId]. */
type CuratedWidgetKey = readonly ['widgets', CuratedKind, number | undefined]

/**
 * The curated requests still under way, so saving new interests can stop
 * one asked with the old ones (see reseedCuratedWidgets). Keyed by kind and
 * user, like the queries.
 */
const curatedInFlight = new Map<string, AbortController>()

function inFlightKey(kind: CuratedKind, userId: number | undefined) {
  return `${kind}:${userId}`
}

/**
 * One person's Articles or Local Events widget. The server asks the LLM and
 * searches afresh on every request, and each one spends search credit, so it
 * loads once per page visit (a full reload loads it again) and after that
 * only when asked: the card's refresh button, or saving new interests. The
 * user id in the key keeps one person's picks off anyone else's page.
 */
export function curatedWidgetQueryOptions(kind: CuratedKind, userId: number | undefined) {
  // The type arguments say failures are ApiErrors (apiFetch's only kind,
  // apart from the reseed's abort, which the query never sees), so cards
  // can show their message and hint.
  return queryOptions<CuratedWidget, ApiError, CuratedWidget, CuratedWidgetKey>({
    queryKey: ['widgets', kind, userId],
    // Not TanStack's AbortSignal, on purpose. If the card unmounts mid-load
    // (StrictMode mounts everything twice in development, or someone clicks
    // away), the request finishes and its answer waits in the cache for the
    // card's return, instead of being cancelled and paid for a second time.
    // Only a reseed aborts it, through this request's own controller.
    queryFn: async () => {
      const key = inFlightKey(kind, userId)
      const controller = new AbortController()
      curatedInFlight.set(key, controller)
      try {
        return await apiFetch<CuratedWidget>(`/api/widgets/${kind}`, {
          signal: controller.signal,
          // Skip the HTTP cache: otherwise the browser holds a newer request
          // for the same URL until an older one still running finishes.
          cache: 'no-store',
        })
      } finally {
        // Unless a newer request has taken its place already.
        if (curatedInFlight.get(key) === controller) curatedInFlight.delete(key)
      }
    },
    enabled: userId !== undefined,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  })
}

/** Your Articles widget: news picked from your interests. `refetch()` asks for fresh picks. */
export function useArticles() {
  return useCuratedWidget('articles')
}

/** Your Local Events widget: upcoming events near you. `refetch()` asks for fresh picks. */
export function useEvents() {
  return useCuratedWidget('events')
}

function useCuratedWidget(kind: CuratedKind) {
  const { data: me } = useMe()
  return useQuery(curatedWidgetQueryOptions(kind, me?.user.id))
}

// ---------------------------------------------------------------------------
// Your interests
// ---------------------------------------------------------------------------

/** What useUpdateProfile notes before saving (its onMutate result). */
interface ProfileSaveContext {
  userId: number | undefined
  before: Profile | undefined
}

/**
 * Saves your interests and location. Call `mutate({ interests, location })`;
 * the server trims each and allows up to 500 characters. On success the
 * cached profile is updated in place, and the curated widgets that read what
 * changed load fresh picks (their old items stay up until the new ones land).
 */
export function useUpdateProfile() {
  const queryClient = useQueryClient()
  return useMutation<ProfileResponse, ApiError, Profile, ProfileSaveContext>({
    onMutate: () => {
      const me = queryClient.getQueryData(meQueryOptions.queryKey)
      return { userId: me?.user.id, before: me?.profile }
    },
    mutationFn: (profile) => apiFetch<ProfileResponse>('/api/me/profile', { method: 'PUT', body: profile }),
    onSuccess: ({ profile }, _input, { userId, before }) => {
      // Someone else logged in on this device while it saved: not their profile.
      if (queryClient.getQueryData(meQueryOptions.queryKey)?.user.id !== userId) return
      queryClient.setQueryData(meQueryOptions.queryKey, (me) => (me ? { ...me, profile } : me))
      queryClient.setQueryData(homepageQueryOptions.queryKey, (page) => (page ? { ...page, profile } : page))

      // Articles read only your interests; events read both. Each reload
      // spends search credit, so only reseed what the change affects.
      const interestsChanged = before?.interests !== profile.interests
      const locationChanged = before?.location !== profile.location
      const stale: CuratedKind[] = []
      if (interestsChanged) stale.push('articles')
      if (interestsChanged || locationChanged) stale.push('events')
      // Not awaited: saving is done; the widgets reload in their own time.
      void reseedCuratedWidgets(queryClient, stale, userId)
    },
  })
}

/** Drops the picks of the given curated widgets and loads fresh ones. */
async function reseedCuratedWidgets(queryClient: QueryClient, kinds: CuratedKind[], userId: number | undefined) {
  await Promise.all(
    kinds.map(async (kind) => {
      const { queryKey } = curatedWidgetQueryOptions(kind, userId)
      // A first load still under way was asked with the old interests, and
      // invalidating alone would join it rather than start over. Cancel it
      // first so its answer is thrown away, and abort its request so the
      // server stops spending search credit on it. Cancelling comes first,
      // so the query never sees the abort as an error.
      const cancelled = queryClient.cancelQueries({ queryKey })
      curatedInFlight.get(inFlightKey(kind, userId))?.abort()
      await cancelled
      // Refetches it now if a card is showing it, or on the card's next mount.
      await queryClient.invalidateQueries({ queryKey })
    }),
  )
}
