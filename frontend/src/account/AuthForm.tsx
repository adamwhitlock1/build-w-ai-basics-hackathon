import type { UseMutationResult } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import type { ApiError, Credentials, Me } from '../api/client'
import { TextField } from '../components/Field'
import { FormError } from '../components/FormError'
import { SubmitButton } from '../components/SubmitButton'
import { WelcomeLayout } from '../components/WelcomeLayout'
import { focusField, looksLikeEmail, MIN_PASSWORD_LENGTH } from './forms'

type Mode = 'login' | 'signup'

const COPY = {
  login: {
    title: 'Welcome back',
    intro: 'Log in to see your page.',
    submit: 'Log in',
    pending: 'Logging in…',
    passwordHint: undefined,
    passwordAutoComplete: 'current-password',
    switchPrompt: 'New to Porchlight?',
    switchLabel: 'Create an account',
    switchTo: '/signup',
  },
  signup: {
    title: 'Make yourself at home',
    intro: 'Create your account, then set up your family.',
    submit: 'Create account',
    pending: 'Creating your account…',
    passwordHint: `At least ${MIN_PASSWORD_LENGTH} characters.`,
    passwordAutoComplete: 'new-password',
    switchPrompt: undefined,
    switchLabel: 'I already have an account',
    switchTo: '/login',
  },
} as const

type FieldErrors = Partial<Record<keyof Credentials, string>>

interface AuthFormProps {
  mode: Mode
  /** useLogin() or useSignup(): both take credentials and resolve with Me. */
  auth: UseMutationResult<Me, ApiError, Credentials>
}

/** The email-and-password form behind both /login and /signup. */
export function AuthForm({ mode, auth }: AuthFormProps) {
  const copy = COPY[mode]
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (auth.isPending) return
    const form = e.currentTarget

    const found = validate(mode, email, password)
    setErrors(found)
    const firstInvalid = (['email', 'password'] as const).find((name) => found[name])
    if (firstInvalid) {
      auth.reset() // an earlier server error no longer applies
      focusField(form, firstInvalid)
      return
    }

    auth.mutate(
      { email: email.trim(), password },
      {
        onSuccess: (me) => void navigate({ to: me.needsFamilySetup ? '/create-family' : '/' }),
        onError: (err) => {
          // Wrong email or password: the form stays as typed, with the
          // password selected so it can simply be typed again.
          if (err.status === 401) focusField(form, 'password', { select: true })
        },
      },
    )
  }

  return (
    <WelcomeLayout
      title={copy.title}
      intro={copy.intro}
      footer={
        <p>
          {copy.switchPrompt && <>{copy.switchPrompt} </>}
          <Link
            to={copy.switchTo}
            className="font-medium text-primary underline-offset-4 hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          >
            {copy.switchLabel}
          </Link>
        </p>
      }
    >
      {/* noValidate: our own inline messages replace the browser's bubbles. */}
      <form noValidate onSubmit={handleSubmit} className="space-y-5">
        <TextField
          label="Email"
          name="email"
          type="email"
          autoComplete="email"
          inputMode="email"
          spellCheck={false}
          value={email}
          error={errors.email}
          onChange={(e) => {
            setEmail(e.target.value)
            setErrors((prev) => ({ ...prev, email: undefined }))
          }}
        />
        <TextField
          label="Password"
          name="password"
          type="password"
          autoComplete={copy.passwordAutoComplete}
          hint={copy.passwordHint}
          value={password}
          error={errors.password}
          onChange={(e) => {
            setPassword(e.target.value)
            setErrors((prev) => ({ ...prev, password: undefined }))
          }}
        />

        {/* Stays until the next attempt, so the form doesn't jump while retyping. */}
        {auth.isError && <FormError message={auth.error.message} hint={auth.error.hint} />}

        <SubmitButton pending={auth.isPending} pendingLabel={copy.pending}>
          {copy.submit}
        </SubmitButton>
      </form>
    </WelcomeLayout>
  )
}

/** Catches what we can before asking the server, with a message that says what to do. */
function validate(mode: Mode, email: string, password: string): FieldErrors {
  const errors: FieldErrors = {}
  const trimmed = email.trim()

  if (!trimmed) {
    errors.email = 'Enter your email address.'
  } else if (!looksLikeEmail(trimmed)) {
    errors.email = 'That doesn’t look like an email address. Check for a typo, like name@example.com.'
  }

  if (!password) {
    errors.password = mode === 'login' ? 'Enter your password.' : `Choose a password of at least ${MIN_PASSWORD_LENGTH} characters.`
  } else if (mode === 'signup' && password.length < MIN_PASSWORD_LENGTH) {
    errors.password = `Use at least ${MIN_PASSWORD_LENGTH} characters (${MIN_PASSWORD_LENGTH - password.length} more).`
  }

  return errors
}
