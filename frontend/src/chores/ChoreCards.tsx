import { Link } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode, type RefObject } from 'react'
import { useChores, useChoreSummary, useSaveChore, type ChoreWrite } from '../api/chores'
import { nameFor } from '../family/people'
import { isStaleDayError, splitByDone, type Checklist } from './model'

const button =
  'rounded-xl border border-primary/30 px-3 py-2 text-sm text-primary hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-60'
const input =
  'min-w-0 flex-1 rounded-xl border border-line bg-bg px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-primary'
const card = 'min-w-0 rounded-3xl border border-line bg-surface p-5 text-ink shadow-card sm:p-6'

/**
 * The chore cards, in a row across the page below the morning briefing:
 * side by side from tablet width up, each as wide as a half-page card, so
 * they don't take the whole page. A kid's one checklist sits in the left half.
 */
export function ChoreRow({ children }: { children: ReactNode }) {
  return <div className="grid grid-cols-1 items-start gap-5 sm:gap-6 md:grid-cols-2">{children}</div>
}

export function ChoreChecklist({
  viewer,
  subject,
  parent,
}: {
  viewer: number
  subject: number
  parent: boolean
}) {
  const query = useChores(viewer, subject)
  const save = useSaveChore(viewer, subject)
  const [editing, setEditing] = useState<number | 'new' | null>(null)
  const [title, setTitle] = useState('')
  const [removing, setRemoving] = useState<number | null>(null)
  const [notice, setNotice] = useState('')
  const heading = useRef<HTMLHeadingElement>(null)
  const doneSummary = useRef<HTMLElement>(null)
  const returnFocus = useRef<HTMLButtonElement | null>(null)
  const formId = useId()
  const data = query.data
  const stale = query.expired || (save.error && isStaleDayError(save.error))
  const checkboxId = (id: number) => `${formId}-chore-${id}`
  // Checking a chore moves it between the open list and the done list, which
  // replaces its checkbox, so focus follows it once the list shows it in its
  // new place. If it landed in the closed "Done" section, focus goes to that
  // section's toggle instead. (The notice set after every save re-renders,
  // so this always gets to run.)
  const focusAfter = useRef<{ id: number; completed: boolean } | null>(null)
  useEffect(() => {
    const want = focusAfter.current
    if (!want || !data) return
    const chore = data.items.find((c) => c.id === want.id)
    if (chore && chore.completed !== want.completed) return
    focusAfter.current = null
    const box = chore ? document.getElementById(checkboxId(chore.id)) : null
    box?.focus()
    if (box && document.activeElement === box) return
    ;(doneSummary.current ?? heading.current)?.focus()
  })
  async function write(change: ChoreWrite) {
    const focusTarget = returnFocus.current
    setNotice('Saving chore…')
    try {
      await save.mutateAsync(change)
      setNotice('Chore saved.')
      setEditing(null)
      setRemoving(null)
      if (change.operation === 'completion' && change.id !== undefined) {
        focusAfter.current = { id: change.id, completed: Boolean(change.completed) }
        return
      }
      requestAnimationFrame(() => {
        if (change.operation === 'remove') heading.current?.focus()
        else if (focusTarget?.isConnected) focusTarget.focus()
      })
    } catch {
      setNotice('Chore was not confirmed. Reload to check its saved state, then try again.')
      // Not saved: the chore stays (or goes back) where it was.
      if (change.operation === 'completion' && change.id !== undefined) {
        focusAfter.current = { id: change.id, completed: !change.completed }
        return
      }
      requestAnimationFrame(() => {
        if (focusTarget?.isConnected) focusTarget.focus()
      })
    }
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    void write({
      operation: editing === 'new' ? 'create' : 'rename',
      id: typeof editing === 'number' ? editing : undefined,
      title,
    })
  }
  const error = save.error ?? query.error
  return (
    <section className={card} aria-labelledby={`${formId}-heading`}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 ref={heading} tabIndex={-1} id={`${formId}-heading`} className="text-2xl outline-none">
          Today's chores
        </h2>
        {data && (
          <span className="text-sm text-ink-soft">
            {data.completedCount} of {data.totalCount} complete
          </span>
        )}
      </div>
      <p className="mt-1 text-sm text-ink-soft">Daily reset at midnight · America/Denver</p>
      <p role="status" className="mt-2 text-sm text-ink-soft">
        {notice}
      </p>
      {query.isPending && <p role="status">Loading chores…</p>}
      {stale && <p role="alert">A new chore day has started. Reload today before checking a chore.</p>}
      {error && (
        <p role="alert" className="mt-2">
          {error.message}
        </p>
      )}
      {(error || stale) && (
        <button
          className={`${button} mt-2`}
          onClick={async () => {
            await query.refetch()
            save.reset()
          }}
        >
          Reload chores
        </button>
      )}
      {data && (
        <>
          {data.items.length === 0 && <p className="my-4 text-ink-soft">No chores assigned yet</p>}
          <ChoreLists data={data} doneSummary={doneSummary}>
            {(chore) => (
              <li
                key={chore.id}
                className="flex flex-col items-stretch gap-2 py-3 sm:flex-row sm:flex-wrap sm:items-center"
              >
                <label className="flex min-w-0 flex-1 items-center gap-3 break-words">
                  <input
                    id={checkboxId(chore.id)}
                    type="checkbox"
                    className="size-5 shrink-0 accent-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                    checked={chore.completed}
                    disabled={save.isPending || Boolean(stale)}
                    onChange={(e) =>
                      void write({
                        operation: 'completion',
                        id: chore.id,
                        date: data.date,
                        completed: e.target.checked,
                      })
                    }
                  />
                  <span className={chore.completed ? 'text-ink-soft line-through' : ''}>{chore.title}</span>
                </label>
                {parent && (
                  <div className="flex gap-2">
                    <button
                      className={button}
                      disabled={save.isPending}
                      aria-label={`Rename ${chore.title}`}
                      onClick={(e) => {
                        returnFocus.current = e.currentTarget
                        setTitle(chore.title)
                        setEditing(chore.id)
                        setRemoving(null)
                      }}
                    >
                      Rename
                    </button>
                    <button
                      className={button}
                      disabled={save.isPending}
                      aria-label={`Remove ${chore.title}`}
                      onClick={(e) => {
                        returnFocus.current = e.currentTarget
                        setRemoving(chore.id)
                        setEditing(null)
                      }}
                    >
                      Remove
                    </button>
                  </div>
                )}
                {removing === chore.id && (
                  <div
                    className="w-full rounded-xl border border-line p-3"
                    role="group"
                    aria-label="Confirm removal"
                  >
                    <p className="mb-2 break-words">Remove “{chore.title}” from every day's checklist?</p>
                    <div className="flex gap-2">
                      <button
                        autoFocus
                        className={button}
                        disabled={save.isPending}
                        onClick={() => {
                          setRemoving(null)
                          returnFocus.current?.focus()
                        }}
                      >
                        Cancel
                      </button>
                      <button
                        className={button}
                        disabled={save.isPending}
                        onClick={() => void write({ operation: 'remove', id: chore.id })}
                      >
                        Confirm removal
                      </button>
                    </div>
                  </div>
                )}
              </li>
            )}
          </ChoreLists>
          {parent && (
            <>
              <button
                className={button}
                disabled={save.isPending || data.totalCount >= 30}
                onClick={(e) => {
                  returnFocus.current = e.currentTarget
                  setEditing('new')
                  setTitle('')
                  setRemoving(null)
                }}
              >
                Add chore
              </button>
              {data.totalCount >= 30 && (
                <p className="mt-2 text-sm text-ink-soft">
                  30 chores assigned. Remove one before adding another.
                </p>
              )}
              {editing !== null && (
                <form onSubmit={submit} className="mt-4">
                  <label className="block text-sm" htmlFor={formId}>
                    {editing === 'new' ? 'New chore' : 'Chore title'}
                  </label>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <input
                      key={editing}
                      autoFocus
                      id={formId}
                      className={input}
                      value={title}
                      required
                      disabled={save.isPending}
                      onChange={(e) => setTitle(e.target.value)}
                    />
                    <button
                      className={button}
                      disabled={save.isPending || !title.trim() || Array.from(title.trim()).length > 120}
                    >
                      Save chore
                    </button>
                    <button
                      type="button"
                      className={button}
                      disabled={save.isPending}
                      onClick={() => {
                        setEditing(null)
                        returnFocus.current?.focus()
                      }}
                    >
                      Cancel
                    </button>
                  </div>
                  <p className="mt-1 text-sm text-ink-soft">Up to 120 characters.</p>
                </form>
              )}
            </>
          )}
        </>
      )}
    </section>
  )
}

type Chore = Checklist['items'][number]

/**
 * The chores still to do, then the finished ones in a "Done" section that
 * starts closed, so a long day's list stays short as it gets done.
 */
function ChoreLists({
  data,
  doneSummary,
  children: item,
}: {
  data: Checklist
  doneSummary: RefObject<HTMLElement>
  children: (chore: Chore) => ReactNode
}) {
  const { open, done } = splitByDone(data.items)
  return (
    <>
      {open.length > 0 && <ul className="my-3 divide-y divide-line">{open.map(item)}</ul>}
      {open.length === 0 && done.length > 0 && <p className="my-4 text-ink-soft">All done for today.</p>}
      {done.length > 0 && (
        <details className="my-3">
          <summary
            ref={doneSummary}
            className="cursor-pointer rounded-lg py-1 text-sm font-medium text-ink-soft hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          >
            Done ({done.length})
          </summary>
          <ul className="mt-1 divide-y divide-line">{done.map(item)}</ul>
        </details>
      )}
    </>
  )
}

export function FamilyChoreProgress({ viewer }: { viewer: number }) {
  const query = useChoreSummary(viewer)
  return (
    <section className={card} aria-labelledby="family-chores-heading">
      <h2 id="family-chores-heading" className="text-2xl">
        Family chores
      </h2>
      <p className="mt-1 text-sm text-ink-soft">Today's progress · America/Denver</p>
      {query.isPending && <p role="status">Loading family progress…</p>}
      {(query.error || query.expired) && (
        <div role="alert">
          <p>{query.error?.message ?? 'Refreshing for the new chore day.'}</p>
          <button className={button} onClick={() => void query.refetch()}>
            Reload progress
          </button>
        </div>
      )}
      {!query.expired && query.data && (
        <ul className="mt-3 divide-y divide-line">
          {query.data.members.map((member) => (
            <li key={member.userId}>
              <Link
                to="/"
                search={{ as: member.userId === viewer ? undefined : member.userId }}
                className="flex flex-wrap justify-between gap-2 rounded-lg py-3 text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-primary"
              >
                <span>{nameFor(member.email)}</span>
                <span className="text-ink-soft">
                  {member.totalCount
                    ? `${member.completedCount} of ${member.totalCount} complete`
                    : 'No chores assigned'}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
