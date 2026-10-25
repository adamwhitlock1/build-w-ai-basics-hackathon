import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { focusField, MIN_PASSWORD_LENGTH } from '../account/forms'
import type { InviteDetails, Me } from '../api/client'
import { inviteQueryOptions, meQueryOptions, useAcceptInvite, useInvite, useLogout, useMe } from '../api/hooks'
import { TextField } from '../components/Field'
import { FormError } from '../components/FormError'
import { SubmitButton } from '../components/SubmitButton'
import { WelcomeLayout } from '../components/WelcomeLayout'
import { familyPhrase } from '../family/people'
import { Breathing } from '../components/Breathing'

/**
 * /invite/<token>: where an invite link lands. It's public (outside the
 * _authed guard), so it works for someone who has never logged in. They set
 * a password and go straight to their own page.
 */
export const Route = createFileRoute('/invite/$token')({
  loader: ({ context, params }) =>
    // Who's logged in (if anyone) and what the link is for, side by side.
    // prefetchQuery never fails: a link that doesn't work is explained by
    // the page itself, not the router's error screen. `staleTime: 0` asks
    // the server afresh on every visit (it may have been used or revoked
    // since), while the page itself then sticks with that answer.
    Promise.all([
      context.queryClient.query(meQueryOptions),
      context.queryClient.prefetchQuery({ ...inviteQueryOptions(params.token), staleTime: 0 }),
    ]),
  component: InvitePage,
})

const linkClass =
  'font-medium text-primary underline-offset-4 hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary'

const linkButtonClass =
  'inline-flex h-11 items-center rounded-2xl border border-primary/30 bg-surface px-4 font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary'

function InvitePage() {
  const { token } = Route.useParams()
  const invite = useInvite(token)
  const { data: me } = useMe()
  const accept = useAcceptInvite(token)
  // After "Log out and continue", the password field takes focus, since
  // the button that had it is gone. (Noted as it's tapped: once logged out,
  // this page shows the form and that button is no longer around to say so.)
  const [cameFromLogout, setCameFromLogout] = useState(false)

  // Accepted: the new member is logged in and on their way to their page.
  // This comes first, since "logged in" is now true too.
  if (accept.isSuccess) {
    return (
      <WelcomeLayout title="Welcome home">
        <p role="status" className="flex items-center gap-3 text-ink-soft">
          <Breathing className="size-6" />
          Opening your page…
        </p>
      </WelcomeLayout>
    )
  }

  if (invite.isError) {
    return (
      <BrokenLink
        status={invite.error.status}
        message={invite.error.message}
        hint={invite.error.hint}
        loggedIn={!!me}
        onRetry={() => void invite.refetch()}
      />
    )
  }

  if (!invite.data) {
    return (
      <WelcomeLayout title="Your invite">
        <p role="status" className="flex items-center gap-3 text-ink-soft">
          <Breathing className="size-6" />
          Checking your invite…
        </p>
      </WelcomeLayout>
    )
  }

  const details = invite.data
  const title = `Join ${familyPhrase(details.familyName)} on Kinlight`

  // Someone is logged in on this device already (a parent testing the link,
  // say). Accepting would switch accounts, so make that a clear choice.
  if (me && accept.isIdle) {
    return (
      <WelcomeLayout title={title}>
        <LoggedInAlready me={me} details={details} onLogOut={() => setCameFromLogout(true)} />
      </WelcomeLayout>
    )
  }

  return (
    <WelcomeLayout
      title={title}
      intro={
        details.isAdmin
          ? 'You’re invited as a parent, so you can invite and look after the family too. Choose a password and your own page will be ready.'
          : 'Choose a password and your own page will be ready, with articles, events and widgets picked just for you.'
      }
      footer={
        <p>
          Already set your password?{' '}
          <Link to="/login" className={linkClass}>
            Log in
          </Link>
        </p>
      }
    >
      <SetPasswordForm details={details} accept={accept} focusPassword={cameFromLogout} />
    </WelcomeLayout>
  )
}

interface SetPasswordFormProps {
  details: InviteDetails
  accept: ReturnType<typeof useAcceptInvite>
  /** Put the cursor in the password field as the form appears. */
  focusPassword: boolean
}

/** The invited email (fixed) and a new password. */
function SetPasswordForm({ details, accept, focusPassword }: SetPasswordFormProps) {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [passwordError, setPasswordError] = useState<string>()
  const formRef = useRef<HTMLFormElement>(null)

  useEffect(() => {
    if (focusPassword && formRef.current) focusField(formRef.current, 'password')
  }, [focusPassword])

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (accept.isPending) return
    const form = e.currentTarget

    // The server's rule, checked here first so the message says how many more.
    const problem = !password
      ? `Choose a password of at least ${MIN_PASSWORD_LENGTH} characters.`
      : password.length < MIN_PASSWORD_LENGTH
        ? `Use at least ${MIN_PASSWORD_LENGTH} characters (${MIN_PASSWORD_LENGTH - password.length} more).`
        : undefined
    setPasswordError(problem)
    if (problem) {
      accept.reset() // an earlier server error no longer applies
      focusField(form, 'password')
      return
    }

    accept.mutate(
      { password },
      {
        // The hook has already logged them in. replace: going "back" shouldn't
        // land on a link that's now used up.
        onSuccess: (me) => void navigate({ to: me.needsFamilySetup ? '/create-family' : '/', replace: true }),
      },
    )
  }

  // These mean the link can't be used any more (it was used or revoked in
  // the meantime), or the email has an account now: logging in is the way on.
  const offerLogin = accept.isError && [404, 409, 410].includes(accept.error.status)

  return (
    // noValidate: our own inline messages replace the browser's bubbles.
    <form ref={formRef} noValidate onSubmit={handleSubmit} className="space-y-5">
      <div>
        <p className="font-medium text-ink">Your email</p>
        <p className="mt-1 text-sm leading-relaxed text-ink-muted">You’ll log in with this.</p>
        <p className="mt-2 rounded-2xl border border-line bg-bg px-4 py-3 break-words text-ink">{details.email}</p>
        {/* Lets password managers save the new password under this email. */}
        <input type="email" name="username" autoComplete="username" value={details.email} readOnly hidden />
      </div>

      <TextField
        label="Choose a password"
        name="password"
        type="password"
        autoComplete="new-password"
        hint={`At least ${MIN_PASSWORD_LENGTH} characters.`}
        value={password}
        error={passwordError}
        onChange={(e) => {
          setPassword(e.target.value)
          setPasswordError(undefined)
        }}
      />

      {/* Stays until the next attempt, so the form doesn't jump while retyping. */}
      {accept.isError && (
        <FormError message={accept.error.message} hint={accept.error.hint}>
          {offerLogin && (
            <Link to="/login" className={`mt-2 inline-block ${linkClass}`}>
              Go to log in
            </Link>
          )}
        </FormError>
      )}

      <SubmitButton pending={accept.isPending} pendingLabel="Setting up your page…">
        Set password and open my page
      </SubmitButton>
    </form>
  )
}

interface LoggedInAlreadyProps {
  me: Me
  details: InviteDetails
  /** Called as "Log out and continue" is tapped. Once logged out, the page shows the password form. */
  onLogOut: () => void
}

/** Shown when someone is already logged in on this device: log out first, or go back. */
function LoggedInAlready({ me, details, onLogOut }: LoggedInAlreadyProps) {
  // Logs out but stays here, so the invite can be accepted next.
  const logout = useLogout({ stayHere: true })
  const sameEmail = me.user.email === details.email

  return (
    <div className="space-y-4">
      <p className="text-ink">
        You’re logged in as <span className="font-medium break-words">{me.user.email}</span>.
      </p>
      {sameEmail ? (
        <p className="text-ink-soft">This invite is for you, but you already have an account, so there’s nothing more to do.</p>
      ) : (
        <>
          <p className="text-ink-soft">
            This invite is for <span className="font-medium break-words text-ink">{details.email}</span>. To set up
            their account on this device, log out first.
          </p>
          {logout.isError && <FormError message={logout.error.message} hint={logout.error.hint} />}
          <button
            type="button"
            aria-disabled={logout.isPending || undefined}
            onClick={() => {
              if (logout.isPending) return
              onLogOut()
              logout.mutate()
            }}
            className="flex h-12 w-full items-center justify-center rounded-2xl bg-primary px-5 font-medium text-on-primary transition-colors hover:bg-primary-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-wait aria-disabled:bg-primary-strong"
          >
            {logout.isPending ? 'Logging out…' : 'Log out and continue'}
          </button>
        </>
      )}
      <p className="text-center text-sm text-ink-soft">
        <Link to="/" className={linkClass}>
          {sameEmail ? 'Go to your page' : 'Or go back to your page'}
        </Link>
      </p>
    </div>
  )
}

interface BrokenLinkProps {
  status: number
  message: string
  hint: string
  loggedIn: boolean
  onRetry: () => void
}

/**
 * The link didn't work. For an unknown, revoked, expired or used link, the
 * server's own words say why and what to do; anything else (the server is
 * down, say) gets "Try again".
 */
function BrokenLink({ status, message, hint, loggedIn, onRetry }: BrokenLinkProps) {
  const linkIsDead = status === 404 || status === 410
  return (
    <WelcomeLayout title={linkIsDead ? 'This invite can’t be used' : 'That didn’t load'}>
      <FormError message={message} hint={hint} />
      <div className="mt-5">
        {!linkIsDead ? (
          <button type="button" onClick={onRetry} className={linkButtonClass}>
            Try again
          </button>
        ) : loggedIn ? (
          <Link to="/" className={linkButtonClass}>
            Go to your page
          </Link>
        ) : (
          <Link to="/login" className={linkButtonClass}>
            Go to log in
          </Link>
        )}
      </div>
    </WelcomeLayout>
  )
}
