import { useIsMutating } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { Suspense, lazy, useCallback, useEffect, useId, useRef, useState, type RefObject } from 'react'
import { InterestsEditor, type InterestsEditorProps } from '../../account/InterestsEditor'
import { InterestsEditorContext } from '../../account/interestsEditorContext'
import { useInterestsEditorPanel } from '../../account/useInterestsEditorPanel'
import type { Layout, SavedWidget } from '../../api/client'
import {
  createWidgetKey,
  homepageQueryOptions,
  useFreshPageProfile,
  useHomepage,
  useMe,
  useUpdateProfile,
} from '../../api/hooks'
import { ChatCard } from '../../chat/ChatCard'
import { CrisisAlert } from '../../chat/CrisisAlert'
import { FamilySidebar } from '../../family/FamilySidebar'
import { MemberPage } from '../../family/MemberPage'
import { layoutOf } from '../../puck/layout'
import { LayoutView } from '../../puck/LayoutView'
import { useLayoutAutosave } from '../../puck/useLayoutAutosave'
import { DescribeWidgetPrompt } from '../../widgets/DescribeWidgetPrompt'
import { PlusIcon } from '../../widgets/icons'

/** The homepage's search params, which say whose page to show. */
interface HomeSearch {
  /**
   * A family member's user id, when a parent is looking at their page
   * (/?as=3). Left out for your own page.
   */
  as?: number
}

export const Route = createFileRoute('/_authed/')({
  // Anything in ?as= that isn't a user id is ignored, so a mangled link just
  // shows your own page. (`as` is always set, even to undefined: the router
  // keeps whatever the URL had for any key left out.) Whether you may see
  // that member's page is up to the server, which says no with a 403 or 404
  // (see family/MemberPage.tsx).
  validateSearch: (search: Record<string, unknown>): HomeSearch => ({ as: userIdFrom(search.as) }),
  // Load your page's data before showing it; the component then reads it
  // from the Query cache, where new widgets are added as they're built. (A
  // member's page loads in place instead, with the sidebar still showing.)
  loader: ({ context }) => context.queryClient.query(homepageQueryOptions),
  component: HomePage,
})

/**
 * A user id from the URL, or undefined if it isn't one. The router reads
 * "?as=3" as the number 3; a string of digits is accepted too.
 */
function userIdFrom(value: unknown): number | undefined {
  const id = typeof value === 'string' && /^\d+$/.test(value) ? Number(value) : value
  return typeof id === 'number' && Number.isSafeInteger(id) && id > 0 ? id : undefined
}

/**
 * The homepage: the family sidebar, and beside it a page. Normally that's
 * your own (YourPage). A parent can pick anyone in the sidebar to see their
 * page instead (MemberPage, at /?as=<their id>); picking yourself, or "Back
 * to your page", comes back.
 */
function HomePage() {
  const { as } = Route.useSearch()
  const { data: me } = useMe()
  // True while you arrange your page; the sidebar's buttons step aside meanwhile.
  const [arranging, setArranging] = useState(false)
  // True while a widget you described is being built. Its prompt is on your
  // page, and so is the error and "Try again" if the build fails, so the
  // sidebar doesn't switch pages until it's done (as while arranging).
  const building = useIsMutating({ mutationKey: createWidgetKey(me?.user.id) }) > 0
  // Whose page to show: a family member's id, or null for your own. Your
  // own id in ?as= is simply your page. And while you're arranging, your
  // page stays until "Done": the sidebar can't switch then, but the
  // browser's Back button could, which would close the editor with a
  // change still unsaved.
  const memberId = as === undefined || as === me?.user.id || arranging ? null : as

  // Switching pages can take away the button that had focus ("Back to your
  // page", say). Then carry on from the new page's heading, rather than from
  // the top of the document.
  const headingRef = useRef<HTMLHeadingElement>(null)
  const shownRef = useRef(memberId)
  useEffect(() => {
    if (shownRef.current === memberId) return
    shownRef.current = memberId
    const active = document.activeElement
    if (!active || active === document.body) headingRef.current?.focus()
  }, [memberId])

  return (
    // From 1024px wide, the family sidebar is a column on the left and the
    // page fills the rest; narrower, the sidebar is a strip on top.
    <div className="min-h-dvh lg:flex">
      <FamilySidebar arranging={arranging} building={building} shownMemberId={memberId} />
      <main className="mx-auto w-full max-w-6xl min-w-0 flex-1 px-4 py-8 sm:px-6 sm:py-12 lg:px-8 lg:py-14">
        {/* Parents only, and only while a possible crisis is unreviewed. */}
        <CrisisAlert />
        {memberId === null ? (
          <YourPage headingRef={headingRef} building={building} onArrangingChange={setArranging} />
        ) : (
          // A new key for each member, so nothing (an open editor, say)
          // carries over from one member's page to the next.
          <MemberPage key={memberId} memberId={memberId} headingRef={headingRef} />
        )}
      </main>
    </div>
  )
}

// The Arrange editor brings Puck (a large library) with it, so it's only
// downloaded when needed: as the pointer or focus reaches "Arrange", or at
// the latest when it's tapped. React's lazy() turns the import into a
// component that shows a fallback (<Suspense>) until it has loaded.
const loadArrangeCanvas = () => import('../../puck/ArrangeCanvas')
const ArrangeCanvas = lazy(() => loadArrangeCanvas().then((m) => ({ default: m.ArrangeCanvas })))

/** How long a new widget keeps its amber "just arrived" border. */
const FRESH_MS = 6000

interface YourPageProps {
  /** For the page's heading, so the homepage can move focus there. */
  headingRef: RefObject<HTMLHeadingElement>
  /** A widget you described is being built (see HomePage). */
  building: boolean
  /** Told when arranging starts and ends, so the sidebar can step aside meanwhile. */
  onArrangingChange: (arranging: boolean) => void
}

/**
 * "Your page": your cards in the order and widths you've arranged them (at
 * first, the Articles and Local Events widgets picked for you, then your
 * saved widgets), then empty spots and the "+" that describes a new one. A
 * new widget is saved by the server, added to the end of the cached page by
 * the hook, and highlighted here.
 *
 * "Arrange" swaps the page for the Puck editor, where cards can be dragged
 * and resized; changes save as you go, and "Done" (or Escape) swaps back.
 */
function YourPage({ headingRef, building, onArrangingChange }: YourPageProps) {
  const { data: page } = useHomepage()
  const { data: me } = useMe()
  const layout = layoutOf(page)
  const [freshId, setFreshId] = useState<number | null>(null)
  const [promptOpen, setPromptOpen] = useState(false)
  const [announcement, setAnnouncement] = useState('')
  const addButtonRef = useRef<HTMLButtonElement>(null)
  const promptId = useId()

  // The interests editor: whether it's open, and where focus goes around it.
  const interestsButtonRef = useRef<HTMLButtonElement>(null)
  const editor = useInterestsEditorPanel({
    announce: setAnnouncement,
    savedMessage: 'Saved. Fresh picks are on their way.',
    buttonRef: interestsButtonRef,
  })

  function handleCreated(widget: SavedWidget) {
    // It's already in the page data (the hook added it, with a card at the
    // end of the layout), so the new card is rendering; mark it as just arrived.
    setFreshId(widget.id)
    // Collapse the prompt; the new card takes focus as it arrives.
    setPromptOpen(false)
    setAnnouncement(`Added “${widget.title}” to your page.`)
  }

  function closePrompt() {
    setPromptOpen(false)
    addButtonRef.current?.focus()
  }

  // Let the amber highlight fade once the new widget has had its moment.
  useEffect(() => {
    if (freshId === null) return
    const timer = window.setTimeout(() => setFreshId(null), FRESH_MS)
    return () => window.clearTimeout(timer)
  }, [freshId])

  // Arranging. `arrangeFrom` is the layout the editor opened with, and null
  // while the page is shown normally.
  const [arrangeFrom, setArrangeFrom] = useState<Layout | null>(null)
  const arranging = arrangeFrom !== null
  const { state: saveState, start: startSaving, change: saveChange, finish: finishSaving } = useLayoutAutosave()
  const [finishing, setFinishing] = useState(false)
  // The same, for event handlers that may run twice before a re-render (a
  // quick Escape and click).
  const finishingRef = useRef(false)

  function startArranging() {
    // The "+" prompt and the interests editor step aside while arranging,
    // and a just-built widget loses its highlight (so it doesn't grab focus
    // again when the page comes back).
    setPromptOpen(false)
    editor.close()
    setFreshId(null)
    // A copy, so nothing the editor does can touch the cached page.
    const from = structuredClone(layout)
    startSaving(from)
    setArrangeFrom(from)
    onArrangingChange(true)
    setAnnouncement('Arranging your page. Drag cards to reorder, or drag a card’s right edge to resize.')
  }

  const finishArranging = useCallback(async () => {
    if (finishingRef.current) return
    finishingRef.current = true
    setFinishing(true)
    // Sends any change still waiting, and puts the final layout in the
    // cached page, which the normal page then shows.
    const saved = await finishSaving()
    finishingRef.current = false
    setFinishing(false)
    // If it didn't save, stay here: the toolbar says why, and it's retried.
    if (!saved) return
    setArrangeFrom(null)
    onArrangingChange(false)
    setAnnouncement('Your page is saved.')
  }, [finishSaving, onArrangingChange])

  // Escape also finishes arranging, unless something else used it first
  // (cancelling a drag or a resize marks it with preventDefault).
  useEffect(() => {
    if (!arranging) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented) void finishArranging()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [arranging, finishArranging])

  return (
    // Widgets anywhere on the page can open the interests editor through this.
    <InterestsEditorContext.Provider value={editor.open}>
      <header className="mb-8 sm:mb-10">
        {/* The buttons share the heading's line, and wrap below it when
            there isn't room. */}
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
          {/* tabIndex -1: focus can be put here (see HomePage), but Tab skips it. */}
          <h1 ref={headingRef} tabIndex={-1} className="text-4xl leading-tight text-ink outline-none sm:text-5xl">
            Your page
          </h1>
          <div className="ml-auto flex shrink-0 items-center gap-1 sm:gap-2">
            {/* One button that reads "Arrange", then "Done" while
                arranging, so keyboard focus stays on it either way.
                Arranging uses the wide-screen grid of four columns,
                which is far too cramped on a phone, so "Arrange" only
                appears from tablet width up ("Done" always shows). */}
            <button
              type="button"
              // Can't start arranging while a new widget is on its way:
              // its card is added to the page when it arrives.
              disabled={building && !arranging}
              title={building && !arranging ? 'Your new widget is still being built' : undefined}
              aria-disabled={finishing || undefined}
              onClick={() => (arranging ? void finishArranging() : startArranging())}
              // Start downloading the editor as soon as it looks likely to be wanted.
              onPointerEnter={() => void loadArrangeCanvas()}
              onFocus={() => void loadArrangeCanvas()}
              className={`rounded-lg px-2.5 py-1 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-wait disabled:opacity-60 aria-disabled:cursor-wait ${
                arranging
                  ? 'bg-ink text-bg hover:bg-ink-soft'
                  : 'text-ink-soft underline-offset-4 hover:text-primary hover:underline disabled:hover:text-ink-soft disabled:hover:no-underline max-md:hidden'
              }`}
            >
              {arranging ? 'Done' : 'Arrange'}
            </button>
            {/* "Your interests" steps aside while arranging (as do the
                sidebar's buttons, "Log out" among them): "Done" comes
                first, so the last change is saved. */}
            {!arranging && (
              <button
                ref={interestsButtonRef}
                type="button"
                aria-expanded={editor.isOpen}
                aria-controls={editor.id}
                // Opens and closes the editor; focus stays on this button.
                onClick={editor.toggle}
                className="rounded-lg px-2 py-1 text-sm font-medium text-ink-soft underline-offset-4 transition-colors hover:text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
              >
                Your interests
              </button>
            )}
          </div>
        </div>
        <p className="mt-2 max-w-prose text-ink-soft">
          Describe anything you’d like to keep an eye on, and we’ll build a widget for it.
        </p>
      </header>

      {/* Always mounted, so changes to its text are announced reliably. */}
      <p role="status" className="sr-only">
        {announcement}
      </p>

      {/* Kids get the Chat card above their cards. It isn't one of the
          arranged cards, so it steps aside while arranging. */}
      {me && !me.user.isAdmin && !arranging && <ChatCard />}

      {editor.focusRequest && (
        <div className="mb-6 max-w-2xl sm:mb-8">
          <OwnInterestsEditor
            id={editor.id}
            focusRequest={editor.focusRequest}
            onSaved={editor.handleSaved}
            onCancel={editor.handleCancel}
          />
        </div>
      )}

      {arranging ? (
        // While the editor downloads (only the first time), show the
        // cards as they are, so nothing jumps.
        <Suspense fallback={<LayoutView layout={arrangeFrom} />}>
          {/* A new key per person: Puck reads its starting layout only
              when it mounts, so another person's page must start afresh. */}
          <ArrangeCanvas key={me?.user.id} initial={arrangeFrom} onChange={saveChange} save={saveState} />
        </Suspense>
      ) : (
        <LayoutView
          layout={layout}
          freshWidgetId={freshId}
          top={promptOpen && <DescribeWidgetPrompt id={promptId} onCreated={handleCreated} onClose={closePrompt} />}
          addTile={
            // The "+" at the end of the grid opens the prompt at the top of it.
            <button
              ref={addButtonRef}
              type="button"
              aria-expanded={promptOpen}
              aria-controls={promptId}
              disabled={building}
              onClick={() => (promptOpen ? closePrompt() : setPromptOpen(true))}
              className="group flex min-h-40 flex-col items-center justify-center gap-3 rounded-3xl border-2 border-dashed border-primary/40 p-6 text-sm font-medium text-ink-soft transition-colors hover:border-primary hover:bg-primary-soft/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-wait disabled:opacity-60 disabled:hover:border-primary/40 disabled:hover:bg-transparent"
            >
              <span
                aria-hidden="true"
                className="flex size-12 items-center justify-center rounded-full bg-primary text-on-primary transition-colors group-hover:bg-primary-strong group-disabled:bg-primary"
              >
                <PlusIcon className="size-6" />
              </span>
              {building ? 'Building your widget…' : 'Add a widget'}
            </button>
          }
        />
      )}
    </InterestsEditorContext.Provider>
  )
}

/**
 * The editor for your own interests and location. It's only mounted while
 * open, so each opening starts with a fresh save (no error left from before),
 * and from a fresh copy of your profile: a parent may have changed it since
 * your page loaded.
 */
function OwnInterestsEditor(props: Omit<InterestsEditorProps, 'save' | 'initial'>) {
  const updateProfile = useUpdateProfile()
  const initial = useFreshPageProfile()
  return <InterestsEditor {...props} initial={initial} save={updateProfile} />
}
