import { useIsMutating } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { Suspense, lazy, useCallback, useEffect, useId, useRef, useState } from 'react'
import { InterestsEditor, type InterestsEditorProps } from '../../account/InterestsEditor'
import { InterestsEditorContext, type ProfileField } from '../../account/interestsEditorContext'
import type { Layout, SavedWidget } from '../../api/client'
import { createWidgetKey, homepageQueryOptions, useHomepage, useMe, useUpdateProfile } from '../../api/hooks'
import { FamilySidebar } from '../../family/FamilySidebar'
import { layoutOf } from '../../puck/layout'
import { LayoutView } from '../../puck/LayoutView'
import { useLayoutAutosave } from '../../puck/useLayoutAutosave'
import { DescribeWidgetPrompt } from '../../widgets/DescribeWidgetPrompt'
import { PlusIcon } from '../../widgets/icons'

export const Route = createFileRoute('/_authed/')({
  // Load the page's data before showing it; the component then reads it
  // from the Query cache, where new widgets are added as they're built.
  loader: ({ context }) => context.queryClient.query(homepageQueryOptions),
  component: HomePage,
})

// The Arrange editor brings Puck (a large library) with it, so it's only
// downloaded when needed: as the pointer or focus reaches "Arrange", or at
// the latest when it's tapped. React's lazy() turns the import into a
// component that shows a fallback (<Suspense>) until it has loaded.
const loadArrangeCanvas = () => import('../../puck/ArrangeCanvas')
const ArrangeCanvas = lazy(() => loadArrangeCanvas().then((m) => ({ default: m.ArrangeCanvas })))

/** How long a new widget keeps its amber "just arrived" border. */
const FRESH_MS = 6000

/**
 * "Your page": your cards in the order and widths you've arranged them (at
 * first, the Articles and Local Events widgets picked for you, then your
 * saved widgets), then empty spots and the "+" that describes a new one. A
 * new widget is saved by the server, added to the end of the cached page by
 * the hook, and highlighted here.
 *
 * "Arrange" swaps the page for the Puck editor, where cards can be dragged
 * and resized; changes save as you go, and "Done" (or Escape) swaps back.
 *
 * Beside it, the family sidebar lists the family's members and, for
 * parents, its invites (see family/FamilySidebar.tsx).
 */
function HomePage() {
  const { data: page } = useHomepage()
  const { data: me } = useMe()
  const layout = layoutOf(page)
  const [freshId, setFreshId] = useState<number | null>(null)
  const [promptOpen, setPromptOpen] = useState(false)
  const [announcement, setAnnouncement] = useState('')
  const addButtonRef = useRef<HTMLButtonElement>(null)
  const promptId = useId()
  const building = useIsMutating({ mutationKey: createWidgetKey(me?.user.id) }) > 0

  // The interests editor: null while closed, otherwise which field to focus.
  // Each request is a new object, so asking again while it's open (say, from
  // a widget's "Add your city" button) moves focus again.
  const [editorFocus, setEditorFocus] = useState<{ field: ProfileField } | null>(null)
  const editorId = useId()
  const interestsButtonRef = useRef<HTMLButtonElement>(null)
  // Whatever opened the editor, so focus can go back there on Cancel.
  const editorOpenerRef = useRef<HTMLElement | null>(null)

  // useCallback keeps this function the same between renders, so the
  // widgets that read it from context don't re-render for nothing.
  const openEditor = useCallback((field: ProfileField = 'interests') => {
    const active = document.activeElement
    editorOpenerRef.current = active instanceof HTMLElement && active !== document.body ? active : null
    setEditorFocus({ field })
    // Clear the last announcement, so saving again (the same words) is a
    // change screen readers hear.
    setAnnouncement('')
  }, [])

  function handleEditorSaved(changed: boolean) {
    setEditorFocus(null)
    // The widget that opened the editor may be about to reload (and lose its
    // button), so land on the header's button, which stays put.
    interestsButtonRef.current?.focus()
    if (changed) setAnnouncement('Saved. Fresh picks are on their way.')
  }

  function handleEditorCancel() {
    setEditorFocus(null)
    const opener = editorOpenerRef.current?.isConnected ? editorOpenerRef.current : interestsButtonRef.current
    opener?.focus()
  }

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
    setEditorFocus(null)
    setFreshId(null)
    // A copy, so nothing the editor does can touch the cached page.
    const from = structuredClone(layout)
    startSaving(from)
    setArrangeFrom(from)
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
    setAnnouncement('Your page is saved.')
  }, [finishSaving])

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
    <InterestsEditorContext.Provider value={openEditor}>
      {/* From 1024px wide, the family sidebar is a column on the left and
          the page fills the rest; narrower, the sidebar is a strip on top. */}
      <div className="min-h-dvh lg:flex">
        <FamilySidebar arranging={arranging} />
        <main className="mx-auto w-full max-w-6xl min-w-0 flex-1 px-4 py-8 sm:px-6 sm:py-12 lg:px-8 lg:py-14">
          <header className="mb-8 sm:mb-10">
            {/* The buttons share the heading's line, and wrap below it when
                there isn't room. */}
            <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
              <h1 className="text-4xl leading-tight text-ink sm:text-5xl">Your page</h1>
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
                      ? 'bg-ink text-white hover:bg-ink-soft'
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
                    aria-expanded={editorFocus !== null}
                    aria-controls={editorId}
                    // Opens and closes the editor; focus stays on this button.
                    onClick={() => (editorFocus ? setEditorFocus(null) : openEditor())}
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

          {editorFocus && (
            <div className="mb-6 max-w-2xl sm:mb-8">
              <OwnInterestsEditor
                id={editorId}
                initial={page.profile}
                focusRequest={editorFocus}
                onSaved={handleEditorSaved}
                onCancel={handleEditorCancel}
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
              top={
                promptOpen && <DescribeWidgetPrompt id={promptId} onCreated={handleCreated} onClose={closePrompt} />
              }
              addTile={
                // The "+" at the end of the grid opens the prompt at the top of it.
                <button
                  ref={addButtonRef}
                  type="button"
                  aria-expanded={promptOpen}
                  aria-controls={promptId}
                  disabled={building}
                  onClick={() => (promptOpen ? closePrompt() : setPromptOpen(true))}
                  className="group flex min-h-40 flex-col items-center justify-center gap-3 rounded-3xl border-2 border-dashed border-accent/50 p-6 text-sm font-medium text-ink-soft transition-colors hover:border-accent hover:bg-accent-soft/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent-strong disabled:cursor-wait disabled:opacity-60 disabled:hover:border-accent/50 disabled:hover:bg-transparent"
                >
                  <span
                    aria-hidden="true"
                    className="flex size-12 items-center justify-center rounded-full bg-accent text-ink transition-colors group-hover:bg-accent-strong group-disabled:bg-accent"
                  >
                    <PlusIcon className="size-6" />
                  </span>
                  {building ? 'Building your widget…' : 'Add a widget'}
                </button>
              }
            />
          )}
        </main>
      </div>
    </InterestsEditorContext.Provider>
  )
}

/**
 * The editor for your own interests and location. It's only mounted while
 * open, so each opening starts with a fresh save (no error left from before).
 */
function OwnInterestsEditor(props: Omit<InterestsEditorProps, 'save'>) {
  const updateProfile = useUpdateProfile()
  return <InterestsEditor {...props} save={updateProfile} />
}
