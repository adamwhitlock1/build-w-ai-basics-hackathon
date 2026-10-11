import { useEffect, useId, useRef, useState } from 'react'
import type { PendingInvite } from '../api/client'
import { useRevokeInvite } from '../api/hooks'
import { COPY_BY_HAND, CopyLinkButton, LinkField } from './CopyLink'
import { RoleTag } from './RoleTag'
import { formatShortDate, inviteLink } from './people'

interface PendingInvitesProps {
  familyId: number
  invites: PendingInvite[]
  /** Hides the Copy and Revoke buttons (while the page is being arranged). */
  readOnly: boolean
  /** Hands a sentence to the sidebar's live region, for screen readers. */
  onAnnounce: (message: string) => void
  /** Called after an invite is revoked and its row has gone, so focus can land somewhere sensible. */
  onRevoked: () => void
}

/**
 * Invites nobody has accepted yet (the co-parents from the create-family
 * intro among them), each with its link to copy again and a way to revoke it.
 */
export function PendingInvites({ familyId, invites, readOnly, onAnnounce, onRevoked }: PendingInvitesProps) {
  const headingId = useId()
  if (invites.length === 0) return null

  return (
    <section aria-labelledby={headingId} className="mt-7">
      <h3 id={headingId} className="px-2 font-sans text-xs font-semibold tracking-wide text-ink-muted uppercase">
        Pending invites
      </h3>
      <ul className="mt-2 space-y-2">
        {invites.map((invite) => (
          <PendingInviteRow
            key={invite.id}
            familyId={familyId}
            invite={invite}
            readOnly={readOnly}
            onAnnounce={onAnnounce}
            onRevoked={onRevoked}
          />
        ))}
      </ul>
    </section>
  )
}

interface PendingInviteRowProps extends Omit<PendingInvitesProps, 'invites'> {
  invite: PendingInvite
}

/** One pending invite: who it's for, as what, until when, and its buttons. */
function PendingInviteRow({ familyId, invite, readOnly, onAnnounce, onRevoked }: PendingInviteRowProps) {
  const revoke = useRevokeInvite(familyId)
  const [confirming, setConfirming] = useState(false)
  // How many times copying the link failed; each one selects it again (below).
  const [copyFailures, setCopyFailures] = useState(0)
  const keepRef = useRef<HTMLButtonElement>(null)
  const revokeRef = useRef<HTMLButtonElement>(null)
  // Set by keep(), so focus goes back to "Revoke" once the question closes
  // (but not when the row first appears).
  const refocusRevoke = useRef(false)
  const until = formatShortDate(invite.expiresAt)

  // When the question appears, start on the safe answer; when it's
  // dismissed, go back to the button that asked it.
  useEffect(() => {
    if (confirming) {
      keepRef.current?.focus()
    } else if (refocusRevoke.current) {
      refocusRevoke.current = false
      revokeRef.current?.focus()
    }
  }, [confirming])

  function confirmRevoke() {
    if (revoke.isPending) return
    revoke.mutate(invite.id, {
      onSuccess: () => {
        onAnnounce(`Revoked the invite for ${invite.email}. That link no longer works.`)
        onRevoked()
      },
      onError: (error) => {
        // 404: it was revoked elsewhere, or just accepted. The list is being
        // refreshed, and this row (with its error) is about to go, so say it
        // here and move focus somewhere that stays.
        if (error.status === 404) {
          onAnnounce(`The invite for ${invite.email} was already revoked or used.`)
          onRevoked()
        }
      },
    })
  }

  function keep() {
    // While revoking, the request is on its way anyway; don't pretend to stop it.
    if (revoke.isPending) return
    refocusRevoke.current = true
    setConfirming(false)
    revoke.reset() // forget a failed attempt
  }

  return (
    <li className="rounded-2xl border border-line bg-surface px-3 py-2.5">
      <div className="flex items-start justify-between gap-2">
        <p className="min-w-0 truncate text-sm font-medium text-ink" title={invite.email}>
          {invite.email}
        </p>
        <RoleTag isAdmin={invite.isAdmin} />
      </div>
      {until && <p className="mt-0.5 text-xs text-ink-muted">Link works until {until}</p>}

      {/* `hidden` keeps the row's state (an open question, say) while the page is arranged. */}
      <div hidden={readOnly} className="mt-1.5">
        {confirming ? (
          <div
            className="rounded-xl bg-danger-soft px-3 py-2"
            // Escape answers "Keep", as it closes the invite form.
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.preventDefault()
                keep()
              }
            }}
          >
            <p className="text-sm text-ink">
              {revoke.isError ? (
                <span role="alert">
                  <span className="font-medium text-danger">Couldn’t revoke it.</span>{' '}
                  {[revoke.error.message, revoke.error.hint].filter(Boolean).join(' ')}
                </span>
              ) : (
                'Revoke this invite? Its link will stop working.'
              )}
            </p>
            <div className="mt-1.5 flex flex-wrap items-center gap-1">
              <button
                type="button"
                onClick={confirmRevoke}
                aria-disabled={revoke.isPending || undefined}
                className="rounded-lg px-2 py-1 text-sm font-semibold text-danger underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-danger aria-disabled:cursor-wait aria-disabled:no-underline"
              >
                {revoke.isPending ? 'Revoking…' : 'Revoke'}
              </button>
              <button
                ref={keepRef}
                type="button"
                onClick={keep}
                aria-disabled={revoke.isPending || undefined}
                className="rounded-lg px-2 py-1 text-sm font-medium text-ink-soft underline-offset-4 hover:text-ink hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-wait aria-disabled:no-underline"
              >
                Keep
              </button>
            </div>
          </div>
        ) : (
          <div className="-ml-2 flex flex-wrap items-center gap-1">
            <CopyLinkButton
              link={inviteLink(invite.url)}
              label={`Copy link for ${invite.email}`}
              onCopied={() => onAnnounce(`Copied the invite link for ${invite.email}.`)}
              onFailed={() => setCopyFailures((n) => n + 1)}
            />
            <button
              ref={revokeRef}
              type="button"
              aria-label={`Revoke the invite for ${invite.email}`}
              onClick={() => setConfirming(true)}
              className="rounded-lg px-2 py-1 text-sm font-medium text-ink-muted underline-offset-4 hover:text-danger hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
              Revoke
            </button>
          </div>
        )}
        {/* The browser wouldn't copy: here's the link, selected, to copy by hand. */}
        {copyFailures > 0 && !confirming && (
          <div className="mt-2 space-y-1.5">
            <p role="alert" className="text-xs leading-relaxed text-ink-muted">
              {COPY_BY_HAND}
            </p>
            <LinkField link={inviteLink(invite.url)} selectRequest={copyFailures} />
          </div>
        )}
      </div>
    </li>
  )
}
