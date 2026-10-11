import { useIsMutating } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState } from 'react'
import type { SavedWidget } from '../../api/client'
import { createWidgetKey, homepageQueryOptions, useHomepage, useMe } from '../../api/hooks'
import { BrandMark } from '../../components/BrandMark'
import { LogoutButton } from '../../components/LogoutButton'
import { DescribeWidgetPrompt } from '../../widgets/DescribeWidgetPrompt'
import { PlusIcon } from '../../widgets/icons'
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
 * "Your page": your saved widgets, oldest first, then empty slots and the "+"
 * that describes a new one. A new widget is saved by the server, appended to
 * the cached page by the hook, and highlighted here.
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
    <div className="min-h-dvh">
      <main className="mx-auto max-w-6xl px-4 py-10 sm:px-6 sm:py-14 lg:px-8">
        <header className="mb-8 sm:mb-10">
          <div className="flex items-center justify-between gap-4">
            <BrandMark familyName={me?.family.name} />
            {/* Temporary home: this moves into the family sidebar later. */}
            <LogoutButton />
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

        <div className="grid grid-cols-1 items-start gap-5 sm:gap-6 md:grid-cols-2 xl:grid-cols-3">
          {promptOpen && (
            <div className="col-span-full">
              <DescribeWidgetPrompt id={promptId} onCreated={handleCreated} onClose={closePrompt} />
            </div>
          )}

          {widgets.map((widget) => (
            <WidgetCard key={widget.id} widget={widget} fresh={widget.id === freshId} />
          ))}

          {Array.from({ length: emptySlots }, (_, i) => (
            <div
              key={`empty-${i}`}
              className="flex min-h-40 items-center justify-center rounded-3xl border-2 border-dashed border-line p-6 text-center text-sm text-ink-muted"
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
        </div>
      </main>
    </div>
  )
}
