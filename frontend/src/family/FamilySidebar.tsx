import { Link } from '@tanstack/react-router'
import { useId, useRef, useState } from 'react'
import type { FamilyMember } from '../api/client'
import { useFamily, useMe } from '../api/hooks'
import { BrandMark } from '../components/BrandMark'
import { useAlerts } from '../api/chat'
import { LogoutButton } from '../components/LogoutButton'
import { ThemeSwitch } from '../components/ThemeSwitch'
import { PlusIcon } from '../widgets/icons'
import { InviteForm } from './InviteForm'
import { PendingInvites } from './PendingInvites'
import { RoleTag } from './RoleTag'
import { avatarTint, initialsFor } from './people'

interface FamilySidebarProps {
  /**
   * True while the page is being arranged. The sidebar stays, but its buttons
   * step aside until "Done", so nothing here (logging out, or switching to
   * someone else's page, say) can close the editor before its last change is
   * saved.
   */
  arranging: boolean
  /**
   * True while a widget you described is being built. Switching pages would
   * take its prompt away, and with it the error and "Try again" if the build
   * fails, so a parent's member rows stay plain until it's done.
   */
  building: boolean
  /** The family member whose page is showing, or null for your own. */
  shownMemberId: number | null
}

/**
 * The family sidebar beside the page: the family's name, its members, and
 * for parents the pending invites and "Invite member". "Log out" sits at the
 * bottom.
 *
 * For parents, each member is a link to their page (/?as=<their id>), and
 * your own row leads back to yours; the page showing is marked with
 * aria-current. Kids see only themselves, as plain text.
 *
 * On wide screens (1024px and up) it's a column down the left. Narrower, it
 * becomes a strip above the page with the family's name and a "Family"
 * button that opens the rest; "Log out" moves up into the strip there, so
 * it's in reach without opening "Family".
 */
export function FamilySidebar({ arranging, building, shownMemberId }: FamilySidebarProps) {
  const { data: me } = useMe()
  const family = useFamily()
  // Narrow screens only: whether the "Family" section is open.
  const [open, setOpen] = useState(false)
  const toggleRef = useRef<HTMLButtonElement>(null)
  const [inviting, setInviting] = useState(false)
  const inviteButtonRef = useRef<HTMLButtonElement>(null)
  const nameId = useId()
  const panelId = useId()
  const inviteFormId = useId()
  const membersHeadingId = useId()

  // What the live region says. Each message gets a new key, so even the same
  // words twice in a row ("Link copied.") are announced again.
  const [announcement, setAnnouncement] = useState({ key: 0, text: '' })
  const announce = (text: string) => setAnnouncement((prev) => ({ key: prev.key + 1, text }))

  // The name and id are in Me from the start, so they show before the
  // member list arrives.
  const familyName = family.data?.family.name ?? me?.family.name ?? ''
  const familyId = family.data?.family.id ?? me?.family.id
  const isAdmin = me?.user.isAdmin ?? false
  // How many flags from the kids' chats are waiting for a parent.
  const unreviewed = useAlerts(isAdmin).data?.unreviewed ?? 0
  // Parents can switch pages, except while arranging or building a widget
  // (see `arranging` and `building`).
  const canSwitch = isAdmin && !arranging && !building
  const members = family.data?.members ?? []
  const pendingInvites = family.data?.pendingInvites ?? []

  function closeInvite() {
    setInviting(false)
    inviteButtonRef.current?.focus()
  }

  /** A parent picked someone's page (maybe their own) in the member list. */
  function handlePick(member: FamilyMember) {
    // On narrow screens the list is inside the "Family" panel, which closes
    // to show the page. Its link is hidden with it, so focus goes to the
    // "Family" button, ready to open it again.
    setOpen(false)
    if (toggleRef.current?.getClientRects().length) toggleRef.current.focus()
    if (!isShown(member, shownMemberId)) {
      announce(member.isYou ? 'Showing your page.' : `Showing ${member.email}’s page.`)
    }
  }

  function handleRevoked() {
    // Its row (and the button that had focus) is gone; land on a button
    // that stays put.
    inviteButtonRef.current?.focus()
  }

  return (
    <aside
      aria-labelledby={nameId}
      className="border-b border-line bg-surface px-4 py-4 sm:px-6 lg:sticky lg:top-0 lg:flex lg:h-dvh lg:w-64 lg:shrink-0 lg:flex-col lg:overflow-y-auto lg:border-r lg:border-b-0 lg:px-5 lg:py-10 xl:w-72"
    >
      <div className="flex items-center justify-between gap-3">
        <BrandMark />
        {/* Narrow screens only; wide ones have it at the bottom. Like the
            one there, it steps aside while arranging. */}
        {!arranging && (
          <div className="lg:hidden">
            <LogoutButton />
          </div>
        )}
      </div>
      <div className="mt-1 flex items-center justify-between gap-3 lg:mt-5">
        <h2
          id={nameId}
          title={familyName}
          className="min-w-0 text-2xl leading-tight text-ink max-lg:truncate lg:text-3xl lg:break-words"
        >
          {familyName || 'Your family'}
        </h2>
        {/* Only on narrow screens; on wide ones everything below is always shown. */}
        <button
          ref={toggleRef}
          type="button"
          aria-expanded={open}
          aria-controls={panelId}
          onClick={() => setOpen((wasOpen) => !wasOpen)}
          className="flex h-11 shrink-0 items-center gap-2 rounded-2xl border border-line bg-bg pr-3 pl-2 text-sm font-medium text-ink-soft transition-colors hover:border-primary/40 hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary lg:hidden"
        >
          {/* A peek at who's in the family. */}
          <span aria-hidden="true" className="flex -space-x-1">
            {members.slice(0, 3).map((member) => (
              <Avatar key={member.id} email={member.email} size="small" />
            ))}
          </span>
          Family
          <ChevronIcon className={`size-4 transition-transform ${open ? 'rotate-180' : ''}`} />
        </button>
      </div>

      {/* Closed on narrow screens until "Family" is tapped. */}
      <div id={panelId} className={`mt-5 lg:mt-8 lg:flex lg:flex-1 lg:flex-col ${open ? '' : 'max-lg:hidden'}`}>
        <section aria-labelledby={membersHeadingId}>
          <h3 id={membersHeadingId} className="px-2 font-sans text-xs font-semibold tracking-wide text-ink-muted uppercase">
            Members
          </h3>
          {family.data ? (
            <MemberList
              members={members}
              switcher={canSwitch ? { shownMemberId, onPick: handlePick } : undefined}
            />
          ) : family.isError ? (
            <div role="alert" className="mt-2 rounded-2xl bg-danger-soft px-3 py-2.5 text-sm">
              <p className="font-medium text-danger">{family.error.message}</p>
              {family.error.hint && <p className="mt-0.5 text-ink-soft">{family.error.hint}</p>}
              <button
                type="button"
                onClick={() => void family.refetch()}
                className="mt-1.5 -ml-2 rounded-lg px-2 py-1 font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
              >
                Try again
              </button>
            </div>
          ) : (
            <MembersLoading />
          )}
          {!isAdmin && family.data && (
            <p className="mt-3 px-2 text-xs leading-relaxed text-ink-muted">Parents can invite family members.</p>
          )}
          {isAdmin && !arranging && members.length > 1 && (
            <p className="mt-3 px-2 text-xs leading-relaxed text-ink-muted">
              {building
                ? 'Once your new widget is ready, you can pick someone to see their page.'
                : 'Pick someone to see their page and set their interests.'}
            </p>
          )}
        </section>

        {isAdmin && !arranging && (
          <Link
            to="/assistant"
            className="mt-6 flex h-11 items-center rounded-2xl px-3 text-sm font-medium text-ink transition-colors hover:bg-primary-soft hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          >
            Assistant
          </Link>
        )}

        {isAdmin && !arranging && (
          <Link
            to="/reviews"
            className="flex h-11 items-center justify-between rounded-2xl px-3 text-sm font-medium text-ink transition-colors hover:bg-primary-soft hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          >
            Reviews
            {unreviewed > 0 && (
              <span className="rounded-full bg-danger px-2 py-0.5 text-xs text-on-danger">
                {unreviewed}
                <span className="sr-only"> to review</span>
              </span>
            )}
          </Link>
        )}

        {isAdmin && familyId !== undefined && (
          <>
            <PendingInvites
              familyId={familyId}
              invites={pendingInvites}
              readOnly={arranging}
              onAnnounce={announce}
              onRevoked={handleRevoked}
            />

            {/* `hidden` rather than unmounting, so a half-typed invite is
                still there after arranging. */}
            <div hidden={arranging} className="mt-6">
              <button
                ref={inviteButtonRef}
                type="button"
                aria-expanded={inviting}
                aria-controls={inviteFormId}
                onClick={() => (inviting ? closeInvite() : setInviting(true))}
                className="flex h-11 w-full items-center justify-center gap-2 rounded-2xl border border-primary/30 bg-surface px-4 text-sm font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-expanded:bg-primary-soft"
              >
                <PlusIcon className="size-4" />
                Invite member
              </button>
              {inviting && (
                <div className="mt-3">
                  <InviteForm
                    id={inviteFormId}
                    familyId={familyId}
                    pendingInvites={family.data?.pendingInvites}
                    onClose={closeInvite}
                    onAnnounce={announce}
                  />
                </div>
              )}
            </div>
          </>
        )}

        {/* The theme switch, and on wide screens "Log out" (narrow ones
            have it up in the strip). While arranging, a note says why the
            buttons are gone. */}
        <div className="mt-8 lg:mt-auto lg:pt-10">
          <div className="border-t border-line pt-3">
            {arranging ? (
              <p className="px-2 py-1 text-xs leading-relaxed text-ink-muted">
                Tap “Done” to save your page, then you can{' '}
                {isAdmin ? 'switch pages, invite someone or log out.' : 'log out.'}
              </p>
            ) : (
              <>
                <ThemeSwitch />
                <div className="mt-3 max-lg:hidden">
                  <LogoutButton align="start" />
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Always mounted, so changes to its text are announced reliably. */}
      <p role="status" className="sr-only">
        <span key={announcement.key}>{announcement.text}</span>
      </p>
    </aside>
  )
}

interface MemberListProps {
  members: FamilyMember[]
  /**
   * Makes each member a link to their page, for parents: whose page is
   * showing (null for your own), and what to do when someone is picked.
   * Without it, the rows are plain.
   */
  switcher?: { shownMemberId: number | null; onPick: (member: FamilyMember) => void }
}

/** Everyone in the family (or, for a kid, just themselves), parents first. */
function MemberList({ members, switcher }: MemberListProps) {
  const row = 'flex items-center gap-3 rounded-2xl px-2 py-2'
  return (
    <ul className="mt-2 space-y-0.5">
      {members.map((member) => {
        if (!switcher) {
          return (
            <li key={member.id} className={row}>
              <MemberRow member={member} />
            </li>
          )
        }
        const shown = isShown(member, switcher.shownMemberId)
        return (
          <li key={member.id}>
            <Link
              to="/"
              // Your own row clears ?as= (back to your page); anyone else's sets it.
              search={member.isYou ? {} : { as: member.id }}
              // The router marks a link whose URL matches exactly; this marks
              // the row whose page is actually showing, which is the same
              // except for oddities such as your own id in ?as=.
              activeOptions={{ exact: true }}
              aria-current={shown ? 'page' : undefined}
              onClick={(e) => {
                // Cmd- or Ctrl-click (and the like) opens it in a new tab or
                // window instead, so nothing changes here.
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
                switcher.onPick(member)
              }}
              className={`${row} transition-colors hover:bg-bg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-[current=page]:bg-primary-soft`}
            >
              <MemberRow member={member} />
            </Link>
          </li>
        )
      })}
    </ul>
  )
}

/** Whether it's this member's page that's showing (`shownMemberId` null: your own). */
function isShown(member: FamilyMember, shownMemberId: number | null) {
  return member.isYou ? shownMemberId === null : shownMemberId === member.id
}

/** One member's avatar, email and tags. */
function MemberRow({ member }: { member: FamilyMember }) {
  return (
    <>
      <Avatar email={member.email} />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium text-ink" title={member.email}>
          {member.email}
        </span>
        <span className="mt-0.5 flex items-center gap-1.5">
          <RoleTag isAdmin={member.isAdmin} />
          {member.isYou && (
            <span className="rounded-full bg-ink-soft px-2 py-0.5 text-xs font-medium text-bg">you</span>
          )}
        </span>
      </span>
    </>
  )
}

/** Soft placeholder rows while the member list loads. */
function MembersLoading() {
  return (
    <div role="status" className="mt-2">
      <span className="sr-only">Loading your family…</span>
      <ul aria-hidden="true" className="space-y-0.5">
        {['w-32', 'w-24'].map((width) => (
          <li key={width} className="flex items-center gap-3 px-2 py-2">
            <span className="size-9 shrink-0 rounded-full bg-line motion-safe:animate-pulse" />
            <span className={`h-3.5 rounded-full bg-line motion-safe:animate-pulse ${width}`} />
          </li>
        ))}
      </ul>
    </div>
  )
}

/** A round avatar with someone's initials. Decorative: their email is always shown beside it. */
function Avatar({ email, size = 'normal' }: { email: string; size?: 'normal' | 'small' }) {
  const sizes = size === 'small' ? 'size-7 text-[0.625rem] ring-2 ring-bg' : 'size-9 text-xs'
  return (
    <span
      aria-hidden="true"
      className={`flex shrink-0 items-center justify-center rounded-full font-semibold tracking-wide text-ink ${sizes} ${avatarTint(email)}`}
    >
      {initialsFor(email)}
    </span>
  )
}

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 20 20" fill="none" className={className}>
      <path d="M5 8l5 5 5-5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
