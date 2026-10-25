import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { focusField, looksLikeEmail } from '../account/forms'
import type { InviteRole, PendingInvite } from '../api/client'
import { useCreateInvite } from '../api/hooks'
import { TextField } from '../components/Field'
import { FormError } from '../components/FormError'
import { SubmitButton } from '../components/SubmitButton'
import { LinkBox } from './CopyLink'
import { formatShortDate, inviteLink, roleName } from './people'

interface InviteFormProps {
  /** DOM id, so the "Invite member" button can point at it with aria-controls. */
  id: string
  familyId: number
  /**
   * The invites pending now (undefined while they load): to tell a new invite
   * from one that email already had, and to notice when the one just made
   * stops being pending.
   */
  pendingInvites: PendingInvite[] | undefined
  /** Called on Cancel, Escape or Done. */
  onClose: () => void
  /** Hands a sentence to the sidebar's live region, for screen readers. */
  onAnnounce: (message: string) => void
}

/** The roles to choose from, in order, with what each one means. */
const ROLES: { value: InviteRole; label: string; detail?: string }[] = [
  { value: 'parent', label: 'Parent', detail: '(can invite and manage)' },
  { value: 'kid', label: 'Kid' },
]

/** An invite that's ready to share, and how to describe it. */
interface Created {
  invite: PendingInvite
  message: string
}

/**
 * The sidebar's inline "Invite member" form: an email and a role. Kinlight
 * doesn't send email, so on success it shows the invite link, ready to copy
 * and send however you like. Errors from the server ("Already a member.")
 * show inline.
 */
export function InviteForm({ id, familyId, pendingInvites, onClose, onAnnounce }: InviteFormProps) {
  const createInvite = useCreateInvite(familyId)
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<InviteRole>('kid')
  const [emailError, setEmailError] = useState<string>()
  const [created, setCreated] = useState<Created | null>(null)
  const formRef = useRef<HTMLFormElement>(null)
  const copyRef = useRef<HTMLButtonElement>(null)
  const anotherRef = useRef<HTMLButtonElement>(null)
  const headingId = useId()
  // Radio buttons are grouped by a shared name; useId keeps it unique on the page.
  const radioGroup = useId()

  // The invite just made leaves the pending list once it's revoked (from the
  // list below, or by another parent) or accepted. Its link no longer works
  // then, so it mustn't still be offered for copying. (The hook adds a new
  // invite to the list as it's made, so it's never missing at the start.)
  const createdGone =
    created !== null &&
    pendingInvites !== undefined &&
    !pendingInvites.some((invite) => invite.id === created.invite.id)

  // The cursor starts in the email field each time the form shows, and the
  // link's "Copy" button takes focus once an invite is ready.
  useEffect(() => {
    if (created) copyRef.current?.focus()
    else if (formRef.current) focusField(formRef.current, 'email')
  }, [created])

  // If focus was on that "Copy" button when it went away, it fell to the
  // page itself; catch it on the next thing to do.
  useEffect(() => {
    if (createdGone && document.activeElement === document.body) anotherRef.current?.focus()
  }, [createdGone])

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (createInvite.isPending) return

    // Tidied as the server tidies it, so the "had an invite" check matches.
    const address = email.trim().toLowerCase()
    const problem = !address
      ? 'Enter their email address.'
      : !looksLikeEmail(address)
        ? 'That doesn’t look like an email address. Check for a typo, like name@example.com.'
        : undefined
    setEmailError(problem)
    if (problem) {
      createInvite.reset() // an earlier server error no longer applies
      focusField(e.currentTarget, 'email')
      return
    }

    // Asking again for an email that already has an invite gives back the
    // same invite (and link), so say so rather than "new invite".
    const before = pendingInvites?.find((invite) => invite.email === address)
    createInvite.mutate(
      { email: address, role },
      {
        onSuccess: (invite) => {
          const message = describe(invite, before)
          setCreated({ invite, message })
          onAnnounce(message)
        },
      },
    )
  }

  /** Back to an empty form, for the next person. */
  function inviteAnother() {
    setEmail('')
    setRole('kid')
    setEmailError(undefined)
    createInvite.reset()
    setCreated(null)
  }

  function handleKeyDown(e: KeyboardEvent<HTMLElement>) {
    if (e.key === 'Escape' && !createInvite.isPending) onClose()
  }

  return (
    <section
      id={id}
      aria-labelledby={headingId}
      onKeyDown={handleKeyDown}
      className="rounded-3xl border border-line bg-surface p-4 shadow-card"
    >
      <h3 id={headingId} className="text-lg text-ink">
        {createdGone ? 'Link no longer works' : created ? 'Invite link ready' : 'Invite someone'}
      </h3>

      {created ? (
        <div className="mt-3 space-y-4">
          {createdGone ? (
            <p className="text-sm leading-relaxed text-ink-soft">
              The invite for {created.invite.email} was revoked or has been used, so its link no longer works.
            </p>
          ) : (
            <>
              <p className="text-sm leading-relaxed text-ink-soft">{created.message}</p>
              <LinkBox
                ref={copyRef}
                link={inviteLink(created.invite.url)}
                label={`Copy link for ${created.invite.email}`}
                onCopied={() => onAnnounce('Link copied.')}
              />
            </>
          )}
          <div className="flex flex-wrap items-center justify-between gap-2">
            <button
              ref={anotherRef}
              type="button"
              onClick={inviteAnother}
              className="rounded-lg px-1 py-1 text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
              Invite someone else
            </button>
            <button
              type="button"
              onClick={onClose}
              className="h-10 rounded-2xl bg-ink px-4 text-sm font-medium text-bg transition-colors hover:bg-ink-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
              Done
            </button>
          </div>
        </div>
      ) : (
        // noValidate: our own inline messages replace the browser's bubbles.
        <form ref={formRef} noValidate onSubmit={handleSubmit} className="mt-3 space-y-4">
          <TextField
            label="Email"
            name="email"
            type="email"
            inputMode="email"
            autoComplete="off"
            spellCheck={false}
            placeholder="name@example.com"
            value={email}
            error={emailError}
            onChange={(e) => {
              setEmail(e.target.value)
              setEmailError(undefined)
              // The server's answer was about the last email; drop it once this one changes.
              if (createInvite.isError) createInvite.reset()
            }}
          />

          <fieldset>
            <legend className="font-medium text-ink">They’ll join as</legend>
            <div className="mt-2 space-y-2">
              {ROLES.map((option) => (
                <label
                  key={option.value}
                  className="flex cursor-pointer items-start gap-3 rounded-2xl border border-line bg-bg px-3 py-2.5 text-sm transition-colors hover:border-primary/40 has-checked:border-primary/50 has-checked:bg-primary-soft has-focus-visible:ring-4 has-focus-visible:ring-primary/15"
                >
                  <input
                    type="radio"
                    name={radioGroup}
                    value={option.value}
                    checked={role === option.value}
                    onChange={() => setRole(option.value)}
                    className="mt-0.5 size-4 shrink-0 accent-primary focus:outline-none"
                  />
                  <span>
                    <span className="font-medium text-ink">{option.label}</span>
                    {option.detail && <span className="text-ink-soft"> {option.detail}</span>}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>

          {createInvite.isError && <FormError message={createInvite.error.message} hint={createInvite.error.hint} />}

          <SubmitButton pending={createInvite.isPending} pendingLabel="Making the link…">
            Create invite link
          </SubmitButton>
          <button
            type="button"
            // While saving, the invite is made anyway, so don't pretend to cancel.
            onClick={() => !createInvite.isPending && onClose()}
            aria-disabled={createInvite.isPending || undefined}
            className="h-10 w-full rounded-2xl text-sm font-medium text-ink-soft transition-colors hover:bg-bg hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-not-allowed aria-disabled:opacity-50 aria-disabled:hover:bg-transparent"
          >
            Cancel
          </button>
        </form>
      )}
    </section>
  )
}

/**
 * What to say about a ready invite: a new one, or the one that email
 * already had (perhaps now for a different role). Kinlight sends no
 * email, so it says to pass the link on.
 */
function describe(invite: PendingInvite, before: PendingInvite | undefined): string {
  const until = formatShortDate(invite.expiresAt)
  const works = until ? ` It works until ${until}.` : ''
  if (!before) {
    return `Send ${invite.email} this link to set a password and open their own page.${works}`
  }
  if (before.isAdmin !== invite.isAdmin) {
    return `${invite.email} already had an invite. It’s now for a ${roleName(invite.isAdmin).toLowerCase()}; the link is the same.${works}`
  }
  return `${invite.email} already has an invite. Here’s the same link again.${works}`
}
