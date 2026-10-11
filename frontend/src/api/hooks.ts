import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  useSuspenseQuery,
  type QueryClient,
} from '@tanstack/react-query'
import { useRouter, type RegisteredRouter } from '@tanstack/react-router'
import {
  ApiError,
  apiFetch,
  type CreateFamilyInput,
  type Credentials,
  type Homepage,
  type Me,
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
 * The logged-in person's page: their profile and saved widgets, oldest first.
 * It only changes through this app, which updates the cache as it goes, so
 * it's never refetched in the background (but can still be invalidated).
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
      // Append it to the cached page (new widgets go last) rather than
      // refetching the whole page.
      queryClient.setQueryData(homepageQueryOptions.queryKey, (page) => {
        if (!page || page.customWidgets.some((w) => w.id === widget.id)) return page
        return { ...page, customWidgets: [...page.customWidgets, widget] }
      })
      onCreated?.(widget)
    },
  })
}
