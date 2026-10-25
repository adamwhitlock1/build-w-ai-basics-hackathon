import { useId } from 'react'
import { useBirthYear, useSetBirthYear } from '../api/chat'

/** The ages a kid's birth year may give them (the server checks this too). */
const MIN_AGE = 5
const MAX_AGE = 17

/**
 * A parent setting a kid's birth year, which picks how Porchlight chat
 * talks to them: a younger voice up to 12, a teen one from 13. Kids can't
 * change it. It saves as soon as a year is picked.
 */
export function BirthYearField({ memberId, name }: { memberId: number; name: string }) {
  const id = useId()
  const birthYear = useBirthYear(memberId)
  const save = useSetBirthYear(memberId)
  const thisYear = new Date().getFullYear()
  const years = Array.from({ length: MAX_AGE - MIN_AGE + 1 }, (_, i) => thisYear - MIN_AGE - i)

  if (!birthYear.data) return null
  const value = save.isPending ? save.variables : birthYear.data.birthYear

  return (
    <div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-ink-soft">
      <label htmlFor={id}>{name}’s birth year</label>
      <select
        id={id}
        value={value ?? ''}
        disabled={save.isPending}
        onChange={(e) => save.mutate(e.target.value === '' ? null : Number(e.target.value))}
        className="h-9 rounded-xl border border-line bg-surface px-2 text-ink focus-visible:outline-2 focus-visible:outline-primary"
      >
        <option value="">Not set</option>
        {years.map((y) => (
          <option key={y} value={y}>
            {y} (about {thisYear - y})
          </option>
        ))}
      </select>
      <span className="text-xs text-ink-muted">Sets how Porchlight chat talks with them.</span>
      {save.isError && <span className="text-danger">{save.error.message}</span>}
      {save.isSuccess && !save.isPending && <span role="status">Saved.</span>}
    </div>
  )
}
