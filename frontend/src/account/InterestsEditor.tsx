import type { UseMutationResult } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import type { ApiError, Profile } from '../api/client'
import { FormError } from '../components/FormError'
import { SubmitButton } from '../components/SubmitButton'
import { focusField } from './forms'
import type { ProfileField } from './interestsEditorContext'
import { ProfileFields } from './ProfileFields'

export interface InterestsEditorProps {
  /** DOM id, so the button that opens the editor can point at it with aria-controls. */
  id?: string
  /** The panel's heading. */
  heading?: string
  /**
   * The saved interests and location the form starts from. If a fresher copy
   * is passed before anything's been typed (the page asks for one as the
   * editor opens: see useFreshPageProfile), the form shows that instead.
   */
  initial: Profile
  /** Whose interests these are, for the fields' wording (see ProfileFields). Yours by default. */
  whose?: 'yours' | 'theirs'
  /**
   * Saves the form: useUpdateProfile() for your own interests, or
   * useUpdateMemberProfile() for a parent setting a family member's. Its
   * pending and error states show here.
   */
  save: UseMutationResult<unknown, ApiError, Profile>
  /**
   * Which field gets the cursor. It's read when the editor opens, and again
   * whenever a new object is passed, so asking twice still moves focus.
   */
  focusRequest?: { field: ProfileField }
  /** Called once the save succeeds, or straight away if nothing changed (`changed` false). */
  onSaved: (changed: boolean) => void
  /** Called on Cancel or Escape. */
  onCancel: () => void
}

/**
 * An inline panel for editing interests and location. It sits in the page
 * rather than in a modal, so the rest of the page stays usable while it's open.
 */
export function InterestsEditor({
  id,
  heading = 'Your interests',
  initial,
  whose = 'yours',
  save,
  focusRequest,
  onSaved,
  onCancel,
}: InterestsEditorProps) {
  const [profile, setProfile] = useState<Profile>(initial)
  // The `initial` the form was last filled from. When a new one arrives and
  // the form still shows the old one untouched, show the new one. (This is
  // React's pattern for adjusting state when a prop changes: set it during
  // render, and React renders again straight away with the new values.)
  const [filledFrom, setFilledFrom] = useState(initial)
  if (initial !== filledFrom) {
    setFilledFrom(initial)
    if (profile === filledFrom) setProfile(initial)
  }
  const formRef = useRef<HTMLFormElement>(null)
  const headingId = useId()

  // Put the cursor in the requested field when the editor opens, and again
  // whenever a new request arrives while it's already open.
  useEffect(() => {
    if (formRef.current) focusField(formRef.current, focusRequest?.field ?? 'interests')
  }, [focusRequest])

  function handleChange(next: Profile) {
    setProfile(next)
    // The error was about the last attempt; once the form changes, clear it.
    if (save.isError) save.reset()
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (save.isPending) return
    const next = { interests: profile.interests.trim(), location: profile.location.trim() }
    // Saving the same values would only reload the widgets, which spends
    // search credit for nothing, so just close.
    if (next.interests === initial.interests.trim() && next.location === initial.location.trim()) {
      onSaved(false)
      return
    }
    save.mutate(next, { onSuccess: () => onSaved(true) })
  }

  function cancel() {
    // While saving, the save goes ahead anyway, so don't pretend to cancel it.
    if (save.isPending) return
    save.reset()
    onCancel()
  }

  function handleKeyDown(e: KeyboardEvent<HTMLElement>) {
    if (e.key === 'Escape') cancel()
  }

  return (
    <section
      id={id}
      aria-labelledby={headingId}
      onKeyDown={handleKeyDown}
      className="rounded-3xl border border-line bg-surface p-5 shadow-card sm:p-6"
    >
      <h2 id={headingId} className="text-xl text-ink">
        {heading}
      </h2>

      <form ref={formRef} noValidate onSubmit={handleSubmit} className="mt-5 space-y-5">
        <ProfileFields value={profile} onChange={handleChange} whose={whose} />

        {save.isError && <FormError message={save.error.message} hint={save.error.hint} />}

        <div className="flex flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-end">
          <button
            type="button"
            onClick={cancel}
            aria-disabled={save.isPending || undefined}
            className="h-12 rounded-2xl px-5 font-medium text-ink-soft transition-colors hover:bg-bg hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-not-allowed aria-disabled:opacity-50 aria-disabled:hover:bg-transparent"
          >
            Cancel
          </button>
          <div className="sm:w-44">
            <SubmitButton pending={save.isPending} pendingLabel="Saving…">
              Save
            </SubmitButton>
          </div>
        </div>
      </form>
    </section>
  )
}
