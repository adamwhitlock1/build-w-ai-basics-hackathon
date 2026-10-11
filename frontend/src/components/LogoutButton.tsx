import { useLogout } from '../api/hooks'

/** A quiet "Log out" text button. Logging out lands on the login screen. */
export function LogoutButton() {
  const logout = useLogout()

  return (
    <span className="inline-flex flex-col items-end gap-1">
      <button
        type="button"
        aria-disabled={logout.isPending || undefined}
        onClick={() => {
          if (!logout.isPending) logout.mutate()
        }}
        className="rounded-lg px-2 py-1 text-sm font-medium text-ink-soft underline-offset-4 transition-colors hover:text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-wait aria-disabled:no-underline"
      >
        {logout.isPending ? 'Logging out…' : 'Log out'}
      </button>
      {logout.isError && (
        <span role="alert" className="max-w-56 text-right text-xs text-danger">
          {logout.error.message} {logout.error.hint}
        </span>
      )}
    </span>
  )
}
