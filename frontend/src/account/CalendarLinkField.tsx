import { useState, type FormEvent } from 'react'
import type { CalendarSummary } from '../api/client'
import { useSetCalendar } from '../api/hooks'
import { FormError } from '../components/FormError'
import { TextField } from '../components/Field'
import { SubmitButton } from '../components/SubmitButton'

interface CalendarLinkFieldProps {
  /** What's connected now, from the page's profile. Never the link itself. */
  calendar: CalendarSummary
  /** Your own calendar, or the family member's a parent is looking at. */
  whose: 'yours' | 'theirs'
}

const quietButton =
  'rounded-xl px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-wait aria-disabled:opacity-60'

/**
 * Connects the calendar behind the Today card. It saves on its own (the
 * server opens the link first, to check it's a calendar), so it's a form of
 * its own beside the interests form rather than part of it. Once connected
 * it only says which service the calendar is from: the link is a secret,
 * and the server never sends it back.
 */
export function CalendarLinkField({ calendar, whose }: CalendarLinkFieldProps) {
  const save = useSetCalendar()
  const [replacing, setReplacing] = useState(false)
  const [url, setUrl] = useState('')
  const editing = !calendar.connected || replacing
  const yours = whose === 'yours'

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (save.isPending || !url.trim()) return
    save.mutate(url.trim(), {
      onSuccess: () => {
        setUrl('')
        setReplacing(false)
      },
    })
  }

  function remove() {
    if (save.isPending) return
    save.mutate(null)
  }

  return (
    <section
      data-calendar-field
      aria-label={yours ? 'Your calendar' : 'Their calendar'}
      className="border-t border-line pt-5"
    >
      {editing ? (
        <form noValidate onSubmit={handleSubmit} className="space-y-3">
          <TextField
            label={yours ? 'Your calendar' : 'Their calendar'}
            hint={
              <>
                Paste a private calendar link to fill the Today card. In Google Calendar, open the calendar’s
                settings and copy “Secret address in iCal format”. In iCloud, share the calendar publicly and copy
                its link. The link stays private on our server. Changes can take a few hours to reach Google’s link.
              </>
            }
            name="calendar"
            type="url"
            inputMode="url"
            autoComplete="off"
            spellCheck={false}
            placeholder="https://calendar.google.com/…/basic.ics"
            value={url}
            onChange={(e) => {
              setUrl(e.target.value)
              if (save.isError) save.reset()
            }}
          />
          {save.isError && <FormError message={save.error.message} hint={save.error.hint} />}
          <div className="flex flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-end">
            {replacing && (
              <button
                type="button"
                onClick={() => {
                  setReplacing(false)
                  setUrl('')
                  save.reset()
                }}
                className={`${quietButton} text-ink-soft hover:bg-bg hover:text-ink`}
              >
                Keep the current one
              </button>
            )}
            <div className="sm:w-44">
              <SubmitButton pending={save.isPending} pendingLabel="Checking…">
                Connect
              </SubmitButton>
            </div>
          </div>
        </form>
      ) : (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-sm font-medium text-ink">{yours ? 'Your calendar' : 'Their calendar'}</p>
            <p className="mt-0.5 text-sm text-ink-soft">
              <span aria-hidden="true" className="mr-1.5 inline-block size-2 rounded-full bg-primary align-middle" />
              {calendar.provider || 'Calendar'} connected
            </p>
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setReplacing(true)}
              className={`${quietButton} border border-primary/30 bg-surface text-primary hover:bg-primary-soft`}
            >
              Replace link
            </button>
            <button
              type="button"
              onClick={remove}
              aria-disabled={save.isPending || undefined}
              className={`${quietButton} text-ink-soft hover:bg-danger-soft hover:text-danger`}
            >
              {save.isPending ? 'Removing…' : 'Remove'}
            </button>
          </div>
        </div>
      )}
      {!editing && save.isError && (
        <div className="mt-3">
          <FormError message={save.error.message} hint={save.error.hint} />
        </div>
      )}
    </section>
  )
}
