import { useLogout } from '../api/hooks'

interface LogoutButtonProps {
  /** Which side the button and any error line up with: the end (default) or the start. */
  align?: 'start' | 'end'
}

/** A quiet "Log out" text button. Logging out lands on the login screen. */
export function LogoutButton({ align = 'end' }: LogoutButtonProps) {
  const logout = useLogout()

  return (
    <span className={`inline-flex flex-col gap-1 ${align === 'start' ? 'items-start' : 'items-end'}`}>
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
        <span role="alert" className={`max-w-56 text-xs text-danger ${align === 'start' ? 'text-left' : 'text-right'}`}>
          {logout.error.message} {logout.error.hint}
        </span>
      )}
    </span>
  )
}
