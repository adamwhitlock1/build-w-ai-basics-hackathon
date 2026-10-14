import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  useSuspenseQuery,
  type Query,
  type QueryClient,
} from '@tanstack/react-query'
import { useRouter, type RegisteredRouter } from '@tanstack/react-router'
import { useContext, useEffect } from 'react'
import { layoutOf, withCustomBlock, withoutCustomBlock } from '../puck/layout'
import { useNow } from '../widgets/updatedAgo'
import {
  ApiError,
  apiFetch,
  type AcceptInviteInput,
  type BriefingWidget,
  type CalendarResponse,
  type CreateFamilyInput,
  type CreateInviteInput,
  type Credentials,
  type CuratedWidget,
  type FamilyOverview,
  type Homepage,
  type InviteDetails,
  type InviteResponse,
  type Layout,
  type LayoutResponse,
  type Me,
  type MemberProfile,
  type PendingInvite,
  type Profile,
  type ProfileResponse,
  type SavedWidget,
  type TodayWidget,
  type Widget,
} from './client'
import { ViewedMemberContext, type PageOwner } from './pageOwner'

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

/**
 * Whether a cached query holds the logged-in person's data (nearly all do).
 * An invite's details are the exception: they're the same for anyone holding
 * the link, and the invite page stays on screen while its visitor logs out
 * or accepts, so they're kept when the session changes.
 */
function belongsToSession(query: Query) {
  return query.queryKey[0] !== 'invite'
}

/** Starts from a clean cache for the person who just logged in, signed up or accepted an invite. */
function startSession(queryClient: QueryClient, me: Me) {
  // Nothing cached for someone else may show up on this person's pages.
  queryClient.removeQueries({ predicate: belongsToSession })
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

/**
 * Logs out on the server, then on this device, and shows the login screen.
 * With `stayHere`, the current page stays on screen instead: the invite page
 * uses it so a logged-in visitor can carry on as the person invited.
 */
export function useLogout({ stayHere = false } = {}) {
  const queryClient = useQueryClient()
  const router = useRouter()
  return useMutation<void, ApiError, void>({
    mutationFn: () => apiFetch<void>('/api/logout', { method: 'POST' }),
    onSuccess: () => {
      if (!stayHere) return endSession(queryClient, router)
      queryClient.setQueryData(meQueryOptions.queryKey, null)
      queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'me' && belongsToSession(query) })
    },
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

/**
 * Your family as the sidebar shows it: its members and, for parents, the
 * invites nobody has accepted yet. It's a cheap request (no search credit),
 * so unlike the widgets it's asked again whenever you come back to the tab:
 * someone may have accepted an invite in another window meanwhile. (The type
 * arguments say failures are ApiErrors, apiFetch's only kind, so the sidebar
 * can show their message and hint.)
 */
export const familyQueryOptions = queryOptions<FamilyOverview, ApiError, FamilyOverview, readonly ['family']>({
  queryKey: ['family'],
  queryFn: ({ signal }) => apiFetch<FamilyOverview>('/api/me/family', { signal }),
  refetchOnWindowFocus: true,
})

/** Your family's members and pending invites (see familyQueryOptions). */
export function useFamily() {
  return useQuery(familyQueryOptions)
}

/**
 * One member of your family, from the list the sidebar loads (undefined
 * until it has, or if they aren't in it). Reading it here doesn't ask the
 * server for the list again; the sidebar keeps it fresh.
 */
export function useFamilyMember(memberId: number) {
  const { data } = useQuery({
    ...familyQueryOptions,
    refetchOnMount: false,
    // Only the member is handed back, so changes to anyone else (or to the
    // invites) don't re-render the caller.
    select: (family) => family.members.find((member) => member.id === memberId),
  })
  return data
}

/**
 * Creates an invite link for someone to join the family (parents only).
 * Call `mutate({ email, role })`; it resolves with the invite. Inviting an
 * email that already has a pending invite gives back that same invite, with
 * the new role; inviting a member, or anyone with an account, fails with a
 * 409 ApiError ("Already a member.").
 */
export function useCreateInvite(familyId: number) {
  const queryClient = useQueryClient()
  return useMutation<PendingInvite, ApiError, CreateInviteInput>({
    mutationFn: async (input) => {
      const res = await apiFetch<InviteResponse>(`/api/families/${familyId}/invites`, { method: 'POST', body: input })
      return res.invite
    },
    onSuccess: (invite) => {
      // Show it in the pending list straight away: in place of the old copy
      // if the email was invited before, otherwise at the end.
      queryClient.setQueryData(familyQueryOptions.queryKey, (family) => {
        if (!family) return family
        const known = family.pendingInvites.some((i) => i.id === invite.id)
        const pendingInvites = known
          ? family.pendingInvites.map((i) => (i.id === invite.id ? invite : i))
          : [...family.pendingInvites, invite]
        return { ...family, pendingInvites }
      })
    },
    // Then ask the server again, success or not: "Already a member" may mean
    // they accepted an invite since the list was loaded.
    onSettled: () => void queryClient.invalidateQueries({ queryKey: familyQueryOptions.queryKey }),
  })
}

/** Revokes a pending invite, so its link stops working (parents only). Call `mutate(inviteId)`. */
export function useRevokeInvite(familyId: number) {
  const queryClient = useQueryClient()
  return useMutation<void, ApiError, number>({
    mutationFn: (inviteId) =>
      apiFetch<void>(`/api/families/${familyId}/invites/${inviteId}`, { method: 'DELETE' }),
    onSuccess: (_nothing, inviteId) => {
      queryClient.setQueryData(familyQueryOptions.queryKey, (family) =>
        family ? { ...family, pendingInvites: family.pendingInvites.filter((i) => i.id !== inviteId) } : family,
      )
    },
    // A 404 means it was accepted or revoked elsewhere meanwhile; the fresh
    // list then drops it either way.
    onSettled: () => void queryClient.invalidateQueries({ queryKey: familyQueryOptions.queryKey }),
  })
}

// ---------------------------------------------------------------------------
// Accepting an invite
// ---------------------------------------------------------------------------

/**
 * Who an invite link is for, and from which family. Anyone holding the link
 * can ask, logged in or not. An unknown or revoked link fails with a 404
 * ApiError, and an expired or used one with a 410.
 */
export function inviteQueryOptions(token: string) {
  return queryOptions<InviteDetails, ApiError, InviteDetails, readonly ['invite', string]>({
    queryKey: ['invite', token],
    queryFn: ({ signal }) => apiFetch<InviteDetails>(`/api/invites/${encodeURIComponent(token)}`, { signal }),
    // The details don't change while the page is open (its loader asks
    // afresh on each visit). A link that didn't work isn't asked about again
    // just because the page re-renders.
    staleTime: Infinity,
    retryOnMount: false,
  })
}

/** The invite behind a link (see inviteQueryOptions). */
export function useInvite(token: string) {
  return useQuery(inviteQueryOptions(token))
}

/**
 * Accepts an invite: creates the account with the chosen password and logs
 * it in. Call `mutate({ password })`; it resolves with the new member's Me.
 * A link that was used or has expired meanwhile fails with a 410 ApiError,
 * and an email that has signed up meanwhile with a 409.
 */
export function useAcceptInvite(token: string) {
  const queryClient = useQueryClient()
  return useMutation<Me, ApiError, AcceptInviteInput>({
    mutationFn: (input) =>
      apiFetch<Me>(`/api/invites/${encodeURIComponent(token)}/accept`, { method: 'POST', body: input }),
    onSuccess: (me) => startSession(queryClient, me),
  })
}

// ---------------------------------------------------------------------------
// Pages: yours, and (for parents) family members'
// ---------------------------------------------------------------------------

/**
 * Query key of someone's page: ['homepage'] for yours, and ['members',
 * memberId, 'homepage'] for a family member's. (One type for both, so a card
 * can read either one through the same hook: see usePageHomepage.)
 */
type HomepageKey = readonly ['homepage'] | readonly ['members', number, 'homepage']

/**
 * The logged-in person's page: their profile, the layout of their cards, and
 * their saved widgets, oldest first. The layout and widgets only change
 * through this app, which updates the cache as it goes, so it's never
 * refetched in the background (but can still be invalidated). The profile
 * can also be changed by a parent, so the interests editor asks for a fresh
 * copy as it opens (see useFreshPageProfile).
 */
export const homepageQueryOptions = queryOptions<Homepage, ApiError, Homepage, HomepageKey>({
  queryKey: ['homepage'],
  queryFn: ({ client, queryKey, signal }) => {
    const owner: PageOwner = { isYou: true, userId: client.getQueryData(meQueryOptions.queryKey)?.user.id }
    return fetchPage(client, queryKey, '/api/me/homepage', owner, signal)
  },
  staleTime: Infinity,
  // Kept for the whole visit, like the cards picked from it, so a fresh copy
  // can always be checked against the profile they were picked for.
  gcTime: Infinity,
})

/**
 * The homepage data. Use it under a route whose loader has already loaded
 * `homepageQueryOptions`; otherwise it suspends until the data arrives.
 */
export function useHomepage() {
  return useSuspenseQuery(homepageQueryOptions)
}

/**
 * A family member's page, as a parent sees it: their profile, the layout of
 * their cards and their saved widgets (the same shape as yours). Parents
 * only: the server answers anyone else with a 403, and an id that isn't
 * someone in your family with a 404.
 *
 * It's a cheap request (no search credit), and they may have changed their
 * page since you last looked, so it's asked again each time you switch to
 * them, while the copy you already have shows.
 */
export function memberHomepageQueryOptions(memberId: number) {
  return queryOptions<Homepage, ApiError, Homepage, HomepageKey>({
    queryKey: ['members', memberId, 'homepage'],
    // No AbortSignal: if you switch away mid-load, the answer is still worth
    // keeping for when you come back. (With one, React's StrictMode, which
    // mounts everything twice in development, would cancel and resend it.)
    queryFn: ({ client, queryKey }) =>
      fetchPage(client, queryKey, `/api/members/${memberId}/homepage`, { isYou: false, userId: memberId }),
    // Kept for the whole visit, like their picks (see homepageQueryOptions).
    gcTime: Infinity,
  })
}

/**
 * Fetches someone's page from `path`, and if its profile isn't the one in
 * the cache, reloads that person's curated widgets to match.
 *
 * A profile can change without this tab knowing: a parent can set a kid's
 * interests, and the kid (or another parent) can change them while a parent
 * isn't looking. The cards would then show picks for the old interests under
 * the new ones. Nothing is reloaded when the profile is unchanged, so the
 * curated widgets' cost protections hold.
 */
async function fetchPage(
  client: QueryClient,
  queryKey: HomepageKey,
  path: string,
  owner: PageOwner,
  signal?: AbortSignal,
): Promise<Homepage> {
  const before = client.getQueryData<Homepage>(queryKey)?.profile
  const page = await apiFetch<Homepage>(path, { signal })
  // Skipped on a first load (there are no picks to correct yet), and when a
  // profile save landed meanwhile: the save has reloaded them already, and
  // this copy is older than it.
  if (before && client.getQueryData<Homepage>(queryKey)?.profile === before) {
    void reseedCuratedWidgets(client, kindsAffected(before, page.profile), owner)
  }
  return page
}

/** A family member's page (see memberHomepageQueryOptions), with its loading and error states. */
export function useMemberHomepage(memberId: number) {
  return useQuery(memberHomepageQueryOptions(memberId))
}

/**
 * Whether a card should show its "Kid-safe" marker: only to parents, on
 * content made for a kid. A kid isn't shown it; it tells them nothing they
 * can act on.
 */
export function useShowKidSafe(widget: Pick<Widget, 'kidSafe'> | undefined): boolean {
  const { data: me } = useMe()
  return Boolean(widget?.kidSafe && me?.user.isAdmin)
}

/**
 * Whose page the calling card is on: yours, or the family member's a parent
 * is viewing (see ViewedMemberContext in pageOwner.ts).
 */
export function usePageOwner(): PageOwner {
  const { data: me } = useMe()
  const memberId = useContext(ViewedMemberContext)
  return memberId === null ? { isYou: true, userId: me?.user.id } : { isYou: false, userId: memberId }
}

/**
 * The page the calling card is on: yours, or the member's a parent is
 * viewing. Pages only show their cards once they've loaded, so this reads
 * it from the cache.
 */
export function usePageHomepage(): Homepage {
  const owner = usePageOwner()
  const options = owner.isYou ? homepageQueryOptions : memberHomepageQueryOptions(owner.userId)
  return useSuspenseQuery(options).data
}

/**
 * The profile of the page the caller is on (yours, or the member's a parent
 * is viewing), for an interests editor to start from. The page is asked for
 * again as the caller mounts: someone else may have changed the profile
 * since it loaded (a parent setting a kid's interests, say), and a form
 * filled from the old copy would save the old values back over theirs. The
 * cached profile is returned until the fresh one arrives.
 */
export function useFreshPageProfile(): MemberProfile {
  const owner = usePageOwner()
  const options = owner.isYou ? homepageQueryOptions : memberHomepageQueryOptions(owner.userId)
  return useSuspenseQuery({ ...options, refetchOnMount: 'always' }).data.profile
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
 * Gets new items for one of your saved widgets: the server runs its saved
 * search again and keeps its title and look. Call `mutate()`. On success the
 * refreshed widget replaces the old one in your cached page, so its card
 * updates in place. Like the other mutations here, the cache update runs
 * even if the card has unmounted meanwhile.
 */
export function useRefreshCustomWidget(widgetId: number) {
  const queryClient = useQueryClient()
  return useMutation<SavedWidget, ApiError, void, { userId: number | undefined }>({
    onMutate: () => ({ userId: queryClient.getQueryData(meQueryOptions.queryKey)?.user.id }),
    mutationFn: () => apiFetch<SavedWidget>(`/api/widgets/custom/${widgetId}/refresh`, { method: 'POST' }),
    onSuccess: (widget, _input, { userId }) => {
      // Someone else logged in on this device while it refreshed.
      if (queryClient.getQueryData(meQueryOptions.queryKey)?.user.id !== userId) return
      queryClient.setQueryData(homepageQueryOptions.queryKey, (page) =>
        page ? { ...page, customWidgets: page.customWidgets.map((w) => (w.id === widget.id ? widget : w)) } : page,
      )
    },
    onError: (err) => {
      // Deleted in another tab: ask for the page again, and the card goes.
      if (err.status === 404) void queryClient.invalidateQueries({ queryKey: homepageQueryOptions.queryKey })
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

/**
 * Query key of one person's curated widget: ['widgets', kind, userId] for
 * yours, and ['members', memberId, 'widgets', kind] for a family member's.
 */
type CuratedWidgetKey =
  | readonly ['widgets', CuratedKind, number | undefined]
  | readonly ['members', number, 'widgets', CuratedKind]

/**
 * The curated requests still under way, so saving new interests can stop
 * one asked with the old ones (see reseedCuratedWidgets). Keyed like the
 * queries.
 */
const curatedInFlight = new Map<string, AbortController>()

function inFlightKey(queryKey: CuratedWidgetKey) {
  return JSON.stringify(queryKey)
}

/**
 * Curated queries whose next fetch should ask the server for new picks
 * (`?refresh=1`) rather than the remembered ones. Keyed like curatedInFlight.
 */
const refreshRequested = new Set<string>()

/**
 * Stale results already sent off for a background refresh, as
 * "<key>@<updatedAt>", so each one is refreshed once: a refresh that fails
 * leaves the same stale result behind, and mustn't retry in a loop.
 */
const staleRefreshed = new Set<string>()

/**
 * One person's Articles or Local Events widget: yours, or a family member's
 * while a parent looks at their page. The server remembers each person's
 * latest picks and sends them back without searching, so it loads once per
 * visit (a full reload loads it again, cheaply) and after that only when
 * asked: the card's refresh button, a background refresh of stale picks
 * (see useCuratedWidget), or saving new interests. The person's id in the
 * key keeps one person's picks off anyone else's page.
 */
export function curatedWidgetQueryOptions(kind: CuratedKind, owner: PageOwner) {
  // Your picks come from /api/widgets/..., and a member's from the
  // parents-only /api/members/{id}/widgets/..., which answers the same way.
  const queryKey: CuratedWidgetKey = owner.isYou
    ? ['widgets', kind, owner.userId]
    : ['members', owner.userId, 'widgets', kind]
  const path = owner.isYou ? `/api/widgets/${kind}` : `/api/members/${owner.userId}/widgets/${kind}`

  // The type arguments say failures are ApiErrors (apiFetch's only kind,
  // apart from the reseed's abort, which the query never sees), so cards
  // can show their message and hint.
  return queryOptions<CuratedWidget, ApiError, CuratedWidget, CuratedWidgetKey>({
    queryKey,
    // Not TanStack's AbortSignal, on purpose. If the card unmounts mid-load
    // (StrictMode mounts everything twice in development, or someone clicks
    // away), the request finishes and its answer waits in the cache for the
    // card's return, instead of being cancelled and paid for a second time.
    // Only a reseed aborts it, through this request's own controller.
    queryFn: async () => {
      const key = inFlightKey(queryKey)
      // Asked by refreshCuratedWidget: get new picks, not the remembered ones.
      const refresh = refreshRequested.delete(key)
      const controller = new AbortController()
      curatedInFlight.set(key, controller)
      try {
        return await apiFetch<CuratedWidget>(refresh ? `${path}?refresh=1` : path, {
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
    enabled: owner.userId !== undefined,
    staleTime: Infinity,
    // Kept for the whole visit, even while no card shows it (say, while a
    // parent looks at someone else's page), so coming back costs no search.
    gcTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  })
}

/**
 * The Articles widget of the page it's on: news picked from that person's
 * interests. `refetch()` asks for fresh picks.
 */
export function useArticles() {
  return useCuratedWidget('articles')
}

/**
 * The Local Events widget of the page it's on: upcoming events near that
 * person. `refetch()` asks for fresh picks.
 */
export function useEvents() {
  return useCuratedWidget('events')
}

/**
 * One curated widget of the page it's on. When the server says its picks
 * are stale, they stay on screen while new ones load in the background,
 * once per stale result.
 */
function useCuratedWidget(kind: CuratedKind) {
  const queryClient = useQueryClient()
  const options = curatedWidgetQueryOptions(kind, usePageOwner())
  const query = useQuery(options)
  // The key's array is new each render; its JSON text only changes when the
  // page's person does, so the effect depends on that.
  const keyText = inFlightKey(options.queryKey)
  const staleSince = query.data?.stale ? query.data.updatedAt : undefined
  useEffect(() => {
    if (!staleSince) return
    const once = `${keyText}@${staleSince}`
    if (staleRefreshed.has(once)) return
    staleRefreshed.add(once)
    void refreshCuratedWidget(queryClient, JSON.parse(keyText) as CuratedWidgetKey)
  }, [queryClient, keyText, staleSince])
  return query
}

/**
 * Returns a function that gets new picks for one curated widget of the page
 * it's on, even if the remembered ones are still fresh: its refresh button.
 */
export function useRefreshCuratedWidget(kind: CuratedKind) {
  const queryClient = useQueryClient()
  const { queryKey } = curatedWidgetQueryOptions(kind, usePageOwner())
  return () => void refreshCuratedWidget(queryClient, queryKey)
}

/**
 * Loads new picks for a curated widget, keeping the current ones on screen
 * meanwhile. A click that lands while a load is already under way (a quick
 * double-click, say) joins it rather than starting a second, paid one.
 */
function refreshCuratedWidget(queryClient: QueryClient, queryKey: CuratedWidgetKey) {
  if (queryClient.isFetching({ queryKey, exact: true }) === 0) refreshRequested.add(inFlightKey(queryKey))
  return queryClient.refetchQueries({ queryKey, exact: true }, { cancelRefetch: false })
}

// ---------------------------------------------------------------------------
// Today (the calendar card)
// ---------------------------------------------------------------------------

/**
 * Query key of one person's Today card on one local day: ['widgets',
 * 'today', userId, date] for yours, ['members', memberId, 'widgets',
 * 'today', date] for a family member's. The date is part of the key, so a
 * page left open past midnight asks for the new day.
 */
type TodayKey =
  | readonly ['widgets', 'today', number | undefined, string]
  | readonly ['members', number, 'widgets', 'today', string]

/** Today queries whose next fetch should get fresh events (`?refresh=1`). */
const todayRefreshRequested = new Set<string>()

/** Stale Today results already refreshed in the background, as "<key>@<updatedAt>". */
const todayStaleRefreshed = new Set<string>()

/** The browser's time zone, e.g. "America/Denver": the person's "today". */
export function browserTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

/** The local date of `now` as YYYY-MM-DD. */
function localDate(now: number): string {
  const d = new Date(now)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function todayQueryOptions(owner: PageOwner, date: string) {
  const queryKey: TodayKey = owner.isYou
    ? ['widgets', 'today', owner.userId, date]
    : ['members', owner.userId, 'widgets', 'today', date]
  const path = owner.isYou ? '/api/widgets/today' : `/api/members/${owner.userId}/widgets/today`
  return queryOptions<TodayWidget, ApiError, TodayWidget, TodayKey>({
    queryKey,
    queryFn: () => {
      const params = new URLSearchParams({ tz: browserTimeZone() })
      if (todayRefreshRequested.delete(JSON.stringify(queryKey))) params.set('refresh', '1')
      return apiFetch<TodayWidget>(`${path}?${params}`, { cache: 'no-store' })
    },
    enabled: owner.userId !== undefined,
    // Like the curated cards: loaded once per visit, then only when asked
    // (the refresh button, a background refresh of stale events, a new day,
    // or a calendar being connected or removed).
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  })
}

/**
 * The Today card of the page it's on: that person's events for today and
 * tomorrow, in this browser's time zone. Stale events stay on screen while
 * fresh ones load in the background, once per stale result.
 */
export function useToday() {
  const queryClient = useQueryClient()
  const date = localDate(useNow(60_000))
  const options = todayQueryOptions(usePageOwner(), date)
  const query = useQuery({
    ...options,
    // Across midnight the key changes; keep yesterday's card up until
    // today's arrives, rather than flashing the loading state.
    placeholderData: (previous) => previous,
  })
  const keyText = JSON.stringify(options.queryKey)
  const staleSince = query.data?.stale && !query.isPlaceholderData ? query.data.updatedAt : undefined
  useEffect(() => {
    if (!staleSince) return
    const once = `${keyText}@${staleSince}`
    if (todayStaleRefreshed.has(once)) return
    todayStaleRefreshed.add(once)
    void refreshToday(queryClient, JSON.parse(keyText) as TodayKey)
  }, [queryClient, keyText, staleSince])
  return query
}

/** Returns a function that fetches the page's calendar again: the Today card's refresh button. */
export function useRefreshToday() {
  const queryClient = useQueryClient()
  const { queryKey } = todayQueryOptions(usePageOwner(), localDate(useNow(60_000)))
  return () => void refreshToday(queryClient, queryKey)
}

function refreshToday(queryClient: QueryClient, queryKey: TodayKey) {
  if (queryClient.isFetching({ queryKey, exact: true }) === 0) todayRefreshRequested.add(JSON.stringify(queryKey))
  return queryClient.refetchQueries({ queryKey, exact: true }, { cancelRefetch: false })
}

/** Every Today query of one person, whatever the day. */
function todayPrefix(owner: PageOwner) {
  return owner.isYou ? ['widgets', 'today', owner.userId] : ['members', owner.userId, 'widgets', 'today']
}

/**
 * Connects or removes the calendar of the page it's on: yours, or (for a
 * parent) the family member's. `mutate(url)` connects one (the server
 * fetches it first and refuses anything that isn't a calendar);
 * `mutate(null)` removes it. On success the page's profile says what's
 * connected, and the Today card loads again.
 */
export function useSetCalendar() {
  const queryClient = useQueryClient()
  const owner = usePageOwner()
  const base = owner.isYou ? '/api/me/calendar' : `/api/members/${owner.userId}/calendar`
  const pageKey = owner.isYou ? homepageQueryOptions.queryKey : memberHomepageQueryOptions(owner.userId).queryKey
  return useMutation<CalendarResponse, ApiError, string | null>({
    mutationFn: (url) =>
      url === null
        ? apiFetch<CalendarResponse>(base, { method: 'DELETE' })
        : apiFetch<CalendarResponse>(base, { method: 'PUT', body: { url } }),
    onSuccess: ({ calendar }) => {
      queryClient.setQueryData(pageKey, (page) => (page ? { ...page, profile: { ...page.profile, calendar } } : page))
      if (owner.isYou) {
        queryClient.setQueryData(meQueryOptions.queryKey, (me) => (me ? { ...me, profile: { ...me.profile, calendar } } : me))
      }
      void queryClient.invalidateQueries({ queryKey: todayPrefix(owner) })
    },
  })
}

// ---------------------------------------------------------------------------
// Morning briefing
// ---------------------------------------------------------------------------

/**
 * Query key of one person's briefing: ['widgets', 'briefing', userId, date,
 * facts] for yours, ['members', memberId, 'widgets', 'briefing', date,
 * facts] for a family member's. `facts` changes whenever the Today,
 * Articles or Local Events results on the page do, so the briefing is asked
 * for again and the server can decide whether to rewrite it.
 */
type BriefingKey =
  | readonly ['widgets', 'briefing', number | undefined, string, string]
  | readonly ['members', number, 'widgets', 'briefing', string, string]

/**
 * The morning briefing of the page it's on. It waits until the page's
 * Today, Articles and Local Events cards have loaded (or failed), so the
 * day's first briefing is written from what they found, and asks again when
 * any of them update or the day changes. The server sends back the
 * remembered briefing unless the facts behind it changed, so asking again
 * is cheap. While a new one loads, the last one stays on screen.
 */
export function useBriefing() {
  const owner = usePageOwner()
  const date = localDate(useNow(60_000))
  // The same queries the cards use, read without the cards' background
  // refreshes (useToday and useCuratedWidget run those once already).
  const today = useQuery(todayQueryOptions(owner, date))
  const articles = useQuery(curatedWidgetQueryOptions('articles', owner))
  const events = useQuery(curatedWidgetQueryOptions('events', owner))
  const settled = !today.isPending && !articles.isPending && !events.isPending
  const facts = [today.data?.updatedAt, articles.data?.updatedAt, events.data?.updatedAt].join('|')

  const queryKey: BriefingKey = owner.isYou
    ? ['widgets', 'briefing', owner.userId, date, facts]
    : ['members', owner.userId, 'widgets', 'briefing', date, facts]
  const path = owner.isYou ? '/api/widgets/briefing' : `/api/members/${owner.userId}/widgets/briefing`
  return useQuery<BriefingWidget, ApiError, BriefingWidget, BriefingKey>({
    queryKey,
    queryFn: () =>
      apiFetch<BriefingWidget>(`${path}?${new URLSearchParams({ tz: browserTimeZone() })}`, { cache: 'no-store' }),
    enabled: owner.userId !== undefined && settled,
    // A new key (fresh facts, a new day) keeps the last briefing up until
    // the server answers, rather than flashing the loading state, but only
    // the same person's: never someone else's briefing on this page.
    placeholderData: (previous, previousQuery) =>
      previousQuery && samePerson(previousQuery.queryKey, queryKey) ? previous : undefined,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  })
}

/** Whether two briefing keys belong to the same person (their first three parts match). */
function samePerson(a: BriefingKey, b: BriefingKey) {
  return a.slice(0, 3).every((part, i) => part === b[i])
}

// ---------------------------------------------------------------------------
// Interests
// ---------------------------------------------------------------------------

/** What a profile save notes before it starts (its onMutate result). */
interface ProfileSaveContext {
  /** Who was logged in. If someone else is by the time it lands, it's not their page to update. */
  userId: number | undefined
  /** The profile before saving, to tell which curated widgets the change affects. */
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
  const pageKey = homepageQueryOptions.queryKey
  return useMutation<ProfileResponse, ApiError, Profile, ProfileSaveContext>({
    onMutate: async () => {
      // A fetch of your page still under way (the one the editor starts as
      // it opens, say) holds the profile from before this save. Cancel it,
      // so it can't land afterwards and put the old interests back.
      await queryClient.cancelQueries({ queryKey: pageKey })
      return {
        userId: queryClient.getQueryData(meQueryOptions.queryKey)?.user.id,
        // What the editor started from, and what your cards were picked for.
        before: queryClient.getQueryData(pageKey)?.profile,
      }
    },
    mutationFn: (profile) => apiFetch<ProfileResponse>('/api/me/profile', { method: 'PUT', body: profile }),
    onSuccess: ({ profile }, _input, { userId, before }) => {
      // Someone else logged in on this device while it saved: not their profile.
      if (queryClient.getQueryData(meQueryOptions.queryKey)?.user.id !== userId) return
      queryClient.setQueryData(meQueryOptions.queryKey, (me) => (me ? { ...me, profile } : me))
      queryClient.setQueryData(pageKey, (page) => (page ? { ...page, profile } : page))
      // Not awaited: saving is done; the widgets reload in their own time.
      void reseedCuratedWidgets(queryClient, kindsAffected(before, profile), { isYou: true, userId })
    },
  })
}

/**
 * Saves a family member's interests and location, for a parent on their
 * page. It works like useUpdateProfile, for member `memberId`: on success
 * their cached page gets the new profile, and only their curated widgets
 * that read what changed load fresh picks. Your own cached page and widgets
 * are left alone.
 */
export function useUpdateMemberProfile(memberId: number) {
  const queryClient = useQueryClient()
  const pageKey = memberHomepageQueryOptions(memberId).queryKey
  return useMutation<ProfileResponse, ApiError, Profile, ProfileSaveContext>({
    onMutate: async () => {
      // Their page is asked for again each time you switch to them (and as
      // the editor opens). Cancel a fetch still under way, so its older
      // profile can't land after this save and replace the new one.
      await queryClient.cancelQueries({ queryKey: pageKey })
      return {
        userId: queryClient.getQueryData(meQueryOptions.queryKey)?.user.id,
        before: queryClient.getQueryData(pageKey)?.profile,
      }
    },
    mutationFn: (profile) =>
      apiFetch<ProfileResponse>(`/api/members/${memberId}/profile`, { method: 'PUT', body: profile }),
    onSuccess: ({ profile }, _input, { userId, before }) => {
      // The parent logged out (and someone else in) while it saved.
      if (queryClient.getQueryData(meQueryOptions.queryKey)?.user.id !== userId) return
      queryClient.setQueryData(pageKey, (page) => (page ? { ...page, profile } : page))
      void reseedCuratedWidgets(queryClient, kindsAffected(before, profile), { isYou: false, userId: memberId })
    },
    // Then ask for their page again: the fetch cancelled above may have had
    // other news (a card they moved since, say). It's a cheap request.
    onSettled: () => void queryClient.invalidateQueries({ queryKey: pageKey }),
  })
}

/**
 * Which curated widgets a profile change affects. Articles read only the
 * interests; events read both. Each reload spends search credit, so only
 * these are reseeded.
 */
function kindsAffected(before: Profile | undefined, after: Profile): CuratedKind[] {
  const interestsChanged = before?.interests !== after.interests
  const locationChanged = before?.location !== after.location
  const kinds: CuratedKind[] = []
  if (interestsChanged) kinds.push('articles')
  if (interestsChanged || locationChanged) kinds.push('events')
  return kinds
}

/** Drops the picks of the given curated widgets on one person's page and loads fresh ones. */
async function reseedCuratedWidgets(queryClient: QueryClient, kinds: CuratedKind[], owner: PageOwner) {
  await Promise.all(
    kinds.map(async (kind) => {
      const { queryKey } = curatedWidgetQueryOptions(kind, owner)
      // A first load still under way was asked with the old interests, and
      // invalidating alone would join it rather than start over. Cancel it
      // first so its answer is thrown away, and abort its request so the
      // server stops spending search credit on it. Cancelling comes first,
      // so the query never sees the abort as an error.
      const cancelled = queryClient.cancelQueries({ queryKey })
      curatedInFlight.get(inFlightKey(queryKey))?.abort()
      // The server picks again for new interests anyway.
      refreshRequested.delete(inFlightKey(queryKey))
      await cancelled
      // Refetches it now if a card is showing it, or on the card's next mount.
      await queryClient.invalidateQueries({ queryKey })
    }),
  )
}
