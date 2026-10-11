import { useId, useRef, useState } from 'react'
import type { FamilyMember } from '../api/client'
import { useFamily, useMe } from '../api/hooks'
import { BrandMark } from '../components/BrandMark'
import { LogoutButton } from '../components/LogoutButton'
import { PlusIcon } from '../widgets/icons'
import { InviteForm } from './InviteForm'
import { PendingInvites } from './PendingInvites'
import { RoleTag } from './RoleTag'
import { avatarTint, initialsFor } from './people'

interface FamilySidebarProps {
  /**
   * True while the page is being arranged. The sidebar stays, but its buttons
   * step aside until "Done", so nothing here (logging out, say) can close
   * the editor before its last change is saved.
   */
  arranging: boolean
}

/**
 * The family sidebar beside your page: the family's name, its members, and
 * for parents the pending invites and "Invite member". "Log out" sits at the
 * bottom.
 *
 * On wide screens (1024px and up) it's a column down the left. Narrower, it
 * becomes a strip above the page with the family's name and a "Family"
 * button that opens the rest; "Log out" moves up into the strip there, so
 * it's in reach without opening "Family".
 */
export function FamilySidebar({ arranging }: FamilySidebarProps) {
  const { data: me } = useMe()
  const family = useFamily()
  // Narrow screens only: whether the "Family" section is open.
  const [open, setOpen] = useState(false)
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
  const members = family.data?.members ?? []
  const pendingInvites = family.data?.pendingInvites ?? []

  function closeInvite() {
    setInviting(false)
    inviteButtonRef.current?.focus()
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
            <MemberList members={members} />
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
        </section>

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

        {/* On narrow screens "Log out" is up in the strip, so this part only
            shows there while arranging, to say why the buttons are gone. */}
        <div className={`mt-8 lg:mt-auto lg:pt-10 ${arranging ? '' : 'max-lg:hidden'}`}>
          <div className="border-t border-line pt-3">
            {arranging ? (
              <p className="px-2 py-1 text-xs leading-relaxed text-ink-muted">
                Tap “Done” to save your page, then you can {isAdmin ? 'invite someone or ' : ''}log out.
              </p>
            ) : (
              <LogoutButton align="start" />
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

/** Everyone in the family (or, for a kid, just themselves), parents first. */
function MemberList({ members }: { members: FamilyMember[] }) {
  return (
    <ul className="mt-2 space-y-0.5">
      {members.map((member) => (
        // Plain rows for now; switching to someone's page comes later.
        <li key={member.id} className="flex items-center gap-3 rounded-2xl px-2 py-2">
          <Avatar email={member.email} />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium text-ink" title={member.email}>
              {member.email}
            </span>
            <span className="mt-0.5 flex items-center gap-1.5">
              <RoleTag isAdmin={member.isAdmin} />
              {member.isYou && (
                <span className="rounded-full bg-ink-soft px-2 py-0.5 text-xs font-medium text-white">you</span>
              )}
            </span>
          </span>
        </li>
      ))}
    </ul>
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
