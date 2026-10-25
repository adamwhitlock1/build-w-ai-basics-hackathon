import { Link } from '@tanstack/react-router'
import { useRef, useState, type RefObject } from 'react'
import { InterestsEditor, type InterestsEditorProps } from '../account/InterestsEditor'
import { BirthYearField } from '../chat/BirthYearField'
import { InterestsEditorContext } from '../account/interestsEditorContext'
import { useInterestsEditorPanel } from '../account/useInterestsEditorPanel'
import type { ApiError } from '../api/client'
import { useFamilyMember, useFreshPageProfile, useMemberHomepage, useUpdateMemberProfile } from '../api/hooks'
import { ViewedMemberContext } from '../api/pageOwner'
import { layoutOf } from '../puck/layout'
import { LayoutView } from '../puck/LayoutView'
import { EyeIcon } from '../widgets/icons'
import { nameFor } from './people'
import { Breathing } from '../components/Breathing'

interface MemberPageProps {
  /** Whose page: a family member's user id (never your own). */
  memberId: number
  /** For the page's heading, so the homepage can move focus there. */
  headingRef: RefObject<HTMLHeadingElement>
}

/** A quiet bordered button, for "Back to your page" and "Try again". */
const secondaryButton =
  'inline-flex shrink-0 items-center rounded-xl border border-primary/30 bg-surface px-3.5 py-2 text-sm font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary'

/**
 * A family member's page as a parent sees it (/?as=<their id>): their cards
 * in their own order and widths, the widgets they described, and articles
 * and events picked from their interests and location. It's read-only (no
 * "+", no empty spots, no Arrange, no Remove), since the page is theirs to
 * arrange; what a parent can change is their interests and location, with
 * "Their interests".
 *
 * The cards are the same ones as on your page. ViewedMemberContext tells
 * them whose page they're on, so they load this member's data, from the
 * parents-only /api/members/{id}/... endpoints and under query keys of
 * their own.
 *
 * The server decides who may look: anyone but a parent gets a 403, and an
 * id that isn't someone in the family a 404. Either way (a stale link, say)
 * this says so calmly, with the way back to your own page.
 */
export function MemberPage({ memberId, headingRef }: MemberPageProps) {
  const page = useMemberHomepage(memberId)
  // Their email comes from the family list, which the sidebar loads anyway.
  const member = useFamilyMember(memberId)
  const name = member ? nameFor(member.email) : null

  const [announcement, setAnnouncement] = useState('')
  const interestsButtonRef = useRef<HTMLButtonElement>(null)
  const editor = useInterestsEditorPanel({
    announce: setAnnouncement,
    savedMessage: 'Saved. Their fresh picks are on their way.',
    buttonRef: interestsButtonRef,
  })

  // A 403 or 404 means this page isn't one you can see (any more), even if
  // it was earlier; any other failure only matters if there's nothing to show.
  const error = page.error
  if (error && (isRefusal(error) || !page.data)) {
    return <Unavailable error={error} headingRef={headingRef} onRetry={() => void page.refetch()} />
  }
  // Until the server answers for someone the family list doesn't show (a
  // kid's stale link, say), say nothing about whose page this is.
  if (!page.data && !member) {
    return <PageLoading />
  }

  return (
    // The cards below load this member's data (see the comment above), and
    // their "Add their interests" buttons open this page's editor.
    <ViewedMemberContext.Provider value={memberId}>
      <InterestsEditorContext.Provider value={editor.open}>
        <ViewingBanner email={member?.email} />

        <header className="mb-8 sm:mb-10">
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
            {/* tabIndex -1: focus can be put here (see HomePage), but Tab skips it. */}
            <h1
              ref={headingRef}
              tabIndex={-1}
              className="min-w-0 text-4xl leading-tight break-words text-ink outline-none sm:text-5xl"
            >
              {name ? `${name}’s page` : 'Their page'}
            </h1>
            {page.data && (
              <button
                ref={interestsButtonRef}
                type="button"
                aria-expanded={editor.isOpen}
                aria-controls={editor.id}
                onClick={editor.toggle}
                className="ml-auto shrink-0 rounded-lg px-2 py-1 text-sm font-medium text-ink-soft underline-offset-4 transition-colors hover:text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
              >
                Their interests
              </button>
            )}
          </div>
          <p className="mt-2 max-w-prose text-ink-soft">
            Their articles and events are picked from their interests and location, which you can set here. Arranging
            the page and adding cards is up to them.
          </p>
          {/* Kids only: their age picks how Kinlight chat talks with them. */}
          {member && !member.isAdmin && <BirthYearField memberId={memberId} name={name ?? 'Their'} />}
        </header>

        {/* Always mounted, so changes to its text are announced reliably. */}
        <p role="status" className="sr-only">
          {announcement}
        </p>

        {page.data && editor.focusRequest && (
          <div className="mb-6 max-w-2xl sm:mb-8">
            <MemberInterestsEditor
              memberId={memberId}
              id={editor.id}
              heading={name ? `${name}’s interests` : 'Their interests'}
              whose="theirs"
              focusRequest={editor.focusRequest}
              onSaved={editor.handleSaved}
              onCancel={editor.handleCancel}
            />
          </div>
        )}

        {/* No addTile: without the "+", LayoutView shows no empty spots either. */}
        {page.data ? <LayoutView layout={layoutOf(page.data)} /> : <CardsLoading />}
      </InterestsEditorContext.Provider>
    </ViewedMemberContext.Provider>
  )
}

/** Whether the server said this page isn't yours to see: 403 (not a parent) or 404 (not in your family). */
function isRefusal(error: ApiError) {
  return error.status === 403 || error.status === 404
}

/**
 * The editor for a family member's interests and location. Like the one for
 * your own, it's only mounted while open, so each opening starts afresh, from
 * a fresh copy of their profile: they may have changed it since you switched
 * to their page.
 */
function MemberInterestsEditor({
  memberId,
  ...props
}: Omit<InterestsEditorProps, 'save' | 'initial'> & { memberId: number }) {
  const save = useUpdateMemberProfile(memberId)
  const initial = useFreshPageProfile()
  return <InterestsEditor {...props} initial={initial} save={save} />
}

/** The calm note across the top of someone else's page: whose it is, who can see it, and the way back. */
function ViewingBanner({ email }: { email?: string }) {
  return (
    <div className="mb-6 flex flex-col gap-3 rounded-2xl border border-primary/15 bg-primary-soft/70 px-4 py-3 sm:mb-8 sm:flex-row sm:items-center sm:justify-between sm:px-5">
      <p className="flex min-w-0 items-start gap-2.5 text-sm leading-relaxed text-ink-soft">
        <EyeIcon className="mt-0.5 size-4 shrink-0 text-primary" />
        <span className="min-w-0">
          You’re viewing <span className="font-medium break-words text-ink">{email ?? 'this family member'}</span>’s
          page. Only you and other parents can see this.
        </span>
      </p>
      {/* exact: otherwise the router marks this link as the current page
          too (its "/" with no ?as= counts as part of "/?as=3"). */}
      <Link to="/" activeOptions={{ exact: true }} className={`self-start sm:self-auto ${secondaryButton}`}>
        Back to your page
      </Link>
    </div>
  )
}

interface UnavailableProps {
  error: ApiError
  headingRef: RefObject<HTMLHeadingElement>
  onRetry: () => void
}

/**
 * Shown instead of the page when it can't be shown: the server's own words
 * for why, and the way back to your page (plus "Try again" when trying again
 * could help).
 */
function Unavailable({ error, headingRef, onRetry }: UnavailableProps) {
  const refused = isRefusal(error)
  return (
    <div className="max-w-xl">
      <h1 ref={headingRef} tabIndex={-1} className="text-4xl leading-tight text-ink outline-none sm:text-5xl">
        {refused ? 'This page isn’t available' : 'Their page didn’t load'}
      </h1>
      <div className="mt-6 rounded-3xl border border-line bg-surface p-6 shadow-card sm:p-8">
        <p className="font-medium text-ink">{error.message}</p>
        <p className="mt-1 text-ink-soft">
          {error.hint || (refused ? 'Your own page is just a click away.' : 'Try again in a moment.')}
        </p>
        <div className="mt-5 flex flex-wrap gap-3">
          {/* exact: see ViewingBanner. */}
          <Link to="/" activeOptions={{ exact: true }} className={secondaryButton}>
            Back to your page
          </Link>
          {!refused && (
            <button type="button" onClick={onRetry} className={secondaryButton}>
              Try again
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

/** While the page loads for someone the sidebar doesn't list: just the breathing light. */
function PageLoading() {
  return (
    <div role="status" className="flex items-center gap-3 py-10 text-ink-muted">
      <Breathing className="size-6" />
      Just a moment…
    </div>
  )
}

/** The breathing light over still placeholder cards, while their page loads. */
function CardsLoading() {
  return (
    <div>
      <p role="status" className="mb-4 flex items-center gap-3 text-ink-muted">
        <Breathing className="size-6" />
        Loading their page…
      </p>
      <div aria-hidden="true" className="grid grid-cols-1 items-start gap-5 sm:gap-6 md:grid-cols-2 lg:grid-cols-4">
        {['lg:col-span-2', 'lg:col-span-2'].map((span, i) => (
          <div key={i} className={`h-72 rounded-3xl border border-line bg-surface shadow-card ${span}`} />
        ))}
      </div>
    </div>
  )
}
