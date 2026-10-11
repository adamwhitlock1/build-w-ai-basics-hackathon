import { useIsMutating } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useCallback, useEffect, useId, useRef, useState } from 'react'
import { InterestsEditor, type InterestsEditorProps } from '../../account/InterestsEditor'
import { InterestsEditorContext, type ProfileField } from '../../account/interestsEditorContext'
import type { SavedWidget } from '../../api/client'
import { createWidgetKey, homepageQueryOptions, useHomepage, useMe, useUpdateProfile } from '../../api/hooks'
import { BrandMark } from '../../components/BrandMark'
import { LogoutButton } from '../../components/LogoutButton'
import { ArticlesWidget } from '../../widgets/ArticlesWidget'
import { DescribeWidgetPrompt } from '../../widgets/DescribeWidgetPrompt'
import { PlusIcon } from '../../widgets/icons'
import { LocalEventsWidget } from '../../widgets/LocalEventsWidget'
import { WidgetCard } from '../../widgets/WidgetCard'

export const Route = createFileRoute('/_authed/')({
  // Load the page's data before showing it; the component then reads it
  // from the Query cache, where new widgets are added as they're built.
  loader: ({ context }) => context.queryClient.query(homepageQueryOptions),
  component: HomePage,
})

/** How long a new widget keeps its amber "just arrived" border. */
const FRESH_MS = 6000
/** Placeholder slots shown on a sparse page, so it never looks bare. */
const EMPTY_SLOTS = 2

/**
 * "Your page": the Articles and Local Events widgets picked for you, then
 * your saved widgets, oldest first, then empty slots and the "+" that
 * describes a new one. A new widget is saved by the server, appended to the
 * cached page by the hook, and highlighted here.
 */
function HomePage() {
  const { data: page } = useHomepage()
  const { data: me } = useMe()
  const widgets = page.customWidgets
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
    // It's already in the page data (the hook added it), so the new card is
    // rendering in the next empty slot; mark it as just arrived.
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

  const emptySlots = Math.max(0, EMPTY_SLOTS - widgets.length)

  return (
    // Widgets anywhere on the page can open the interests editor through this.
    <InterestsEditorContext.Provider value={openEditor}>
      <div className="min-h-dvh">
        <main className="mx-auto max-w-6xl px-4 py-10 sm:px-6 sm:py-14 lg:px-8">
          <header className="mb-8 sm:mb-10">
            {/* The buttons wrap below the family name when both don't fit
                (on a phone, say), rather than cutting the name short. */}
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
              <BrandMark familyName={me?.family.name} />
              {/* Temporary home: these move into the family sidebar later. */}
              <div className="ml-auto flex shrink-0 items-start gap-1 sm:gap-2">
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
                <LogoutButton />
              </div>
            </div>
            <h1 className="mt-3 text-4xl leading-tight text-ink sm:text-5xl">Your page</h1>
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

          {/* Six columns on wide screens: the two picked-for-you widgets take
              half the width each, and everything after them a third. */}
          <div className="grid grid-cols-1 items-start gap-5 sm:gap-6 md:grid-cols-2 xl:grid-cols-6">
            {promptOpen && (
              <div className="col-span-full">
                <DescribeWidgetPrompt id={promptId} onCreated={handleCreated} onClose={closePrompt} />
              </div>
            )}

            <div className="xl:col-span-3">
              <ArticlesWidget />
            </div>
            <div className="xl:col-span-3">
              <LocalEventsWidget />
            </div>

            {widgets.map((widget) => (
              <div key={widget.id} className="xl:col-span-2">
                <WidgetCard widget={widget} fresh={widget.id === freshId} />
              </div>
            ))}

            {Array.from({ length: emptySlots }, (_, i) => (
              <div
                key={`empty-${i}`}
                className="flex min-h-40 items-center justify-center rounded-3xl border-2 border-dashed border-line p-6 text-center text-sm text-ink-muted xl:col-span-2"
              >
                This spot is empty — add a widget to fill it
              </div>
            ))}

            {/* The "+" at the end of the grid opens the prompt at the top of it. */}
            <button
              ref={addButtonRef}
              type="button"
              aria-expanded={promptOpen}
              aria-controls={promptId}
              disabled={building}
              onClick={() => (promptOpen ? closePrompt() : setPromptOpen(true))}
              className="group flex min-h-40 flex-col items-center justify-center gap-3 rounded-3xl border-2 border-dashed border-accent/50 p-6 text-sm font-medium text-ink-soft transition-colors hover:border-accent hover:bg-accent-soft/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent-strong disabled:cursor-wait disabled:opacity-60 disabled:hover:border-accent/50 disabled:hover:bg-transparent xl:col-span-2"
            >
              <span
                aria-hidden="true"
                className="flex size-12 items-center justify-center rounded-full bg-accent text-ink transition-colors group-hover:bg-accent-strong group-disabled:bg-accent"
              >
                <PlusIcon className="size-6" />
              </span>
              {building ? 'Building your widget…' : 'Add a widget'}
            </button>
          </div>
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
