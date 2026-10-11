import { Link, createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { focusField, looksLikeEmail, splitEmails } from '../account/forms'
import { meQueryOptions, useCreateFamily, useMe } from '../api/hooks'
import { TextAreaField, TextField } from '../components/Field'
import { FormError } from '../components/FormError'
import { LogoutButton } from '../components/LogoutButton'
import { SubmitButton } from '../components/SubmitButton'
import { WelcomeLayout } from '../components/WelcomeLayout'

export const Route = createFileRoute('/create-family')({
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.query(meQueryOptions)
    if (!me) throw redirect({ to: '/login' })
    if (!me.needsFamilySetup) throw redirect({ to: '/' })
  },
  component: CreateFamilyPage,
})

/** Server limits, enforced here with maxLength so nobody types past them. */
const NAME_MAX = 80
const PROFILE_MAX = 500

interface FieldErrors {
  name?: string
  coAdminEmails?: string
}

/** The create-family intro, shown right after signing up. */
function CreateFamilyPage() {
  const navigate = useNavigate()
  const { data: me } = useMe()
  const createFamily = useCreateFamily()
  const [name, setName] = useState('')
  const [coAdmins, setCoAdmins] = useState('')
  const [interests, setInterests] = useState('')
  const [location, setLocation] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (createFamily.isPending) return
    const form = e.currentTarget

    const coAdminEmails = splitEmails(coAdmins)
    const found = validate(name, coAdminEmails)
    setErrors(found)
    const firstInvalid = (['name', 'coAdminEmails'] as const).find((field) => found[field])
    if (firstInvalid) {
      createFamily.reset() // an earlier server error no longer applies
      focusField(form, firstInvalid)
      return
    }

    createFamily.mutate(
      { name: name.trim(), coAdminEmails, interests: interests.trim(), location: location.trim() },
      // replace: going "back" shouldn't land on a form that's already done.
      { onSuccess: () => void navigate({ to: '/', replace: true }) },
    )
  }

  return (
    <WelcomeLayout
      wide
      title="Set up your family"
      intro="A few details to get your page started. Everyone you invite later gets a page of their own."
      footer={
        <p className="flex flex-wrap items-center justify-center gap-x-1">
          {me && <span>Signed in as {me.user.email}.</span>}
          <LogoutButton />
        </p>
      }
    >
      <form noValidate onSubmit={handleSubmit} className="space-y-6">
        <TextField
          label="Family name"
          name="name"
          placeholder="e.g. The Riveras"
          autoComplete="off"
          maxLength={NAME_MAX}
          value={name}
          error={errors.name}
          onChange={(e) => {
            setName(e.target.value)
            setErrors((prev) => ({ ...prev, name: undefined }))
          }}
        />
        <TextField
          label="Co-admin emails"
          optional
          hint="Another parent or grown-up who helps run things. Separate several with commas; we’ll keep an invite ready for each."
          name="coAdminEmails"
          type="text"
          inputMode="email"
          autoComplete="off"
          spellCheck={false}
          placeholder="partner@example.com"
          value={coAdmins}
          error={errors.coAdminEmails}
          onChange={(e) => {
            setCoAdmins(e.target.value)
            setErrors((prev) => ({ ...prev, coAdminEmails: undefined }))
          }}
        />
        <TextAreaField
          label="What would you like to see on your page?"
          hint="Your interests, in your own words. We’ll use them to pick articles and events for you."
          name="interests"
          placeholder="e.g. space launches, Premier League soccer, easy weeknight dinners"
          maxLength={PROFILE_MAX}
          value={interests}
          onChange={(e) => setInterests(e.target.value)}
        />
        <TextField
          label="Where are you?"
          hint="A city is fine. It’s for finding events near you."
          name="location"
          placeholder="e.g. Denver, Colorado"
          autoComplete="address-level2"
          maxLength={PROFILE_MAX}
          value={location}
          onChange={(e) => setLocation(e.target.value)}
        />

        {createFamily.isError && (
          <FormError message={createFamily.error.message} hint={createFamily.error.hint}>
            {createFamily.error.status === 409 && (
              <Link
                to="/"
                className="mt-2 inline-block font-medium text-primary underline-offset-4 hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
              >
                Go to your page
              </Link>
            )}
          </FormError>
        )}

        <SubmitButton pending={createFamily.isPending} pendingLabel="Creating your family…">
          Create family
        </SubmitButton>
      </form>
    </WelcomeLayout>
  )
}

/** Catches what we can before asking the server, with a message that says what to do. */
function validate(name: string, coAdminEmails: string[]): FieldErrors {
  const errors: FieldErrors = {}
  if (!name.trim()) {
    errors.name = 'Give your family a name. It’s what everyone in it will see.'
  }
  const invalid = coAdminEmails.filter((email) => !looksLikeEmail(email))
  if (invalid.length > 0) {
    const list = invalid.map((email) => `“${email}”`).join(', ')
    errors.coAdminEmails = `${list} ${invalid.length === 1 ? 'doesn’t' : 'don’t'} look like an email address. Check for a typo, or leave this blank.`
  }
  return errors
}
