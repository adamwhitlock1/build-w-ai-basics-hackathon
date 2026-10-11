import { roleName } from './people'

/** A small "Parent" or "Kid" label. */
export function RoleTag({ isAdmin }: { isAdmin: boolean }) {
  return (
    <span
      className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
        isAdmin ? 'bg-primary-soft text-primary-strong' : 'bg-bg text-ink-soft ring-1 ring-line'
      }`}
    >
      {roleName(isAdmin)}
    </span>
  )
}
