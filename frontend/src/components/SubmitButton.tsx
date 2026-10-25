import type { ReactNode } from 'react'

interface SubmitButtonProps {
  /** True while the form's request is in flight. */
  pending: boolean
  /** Replaces the label while pending, e.g. "Logging in…". */
  pendingLabel: string
  children: ReactNode
}

/**
 * A form's main button. While pending it shows a calm spinner and the form
 * ignores further submits. It uses aria-disabled rather than disabled, so
 * keyboard focus stays on it instead of falling back to the page.
 */
export function SubmitButton({ pending, pendingLabel, children }: SubmitButtonProps) {
  return (
    <button
      type="submit"
      aria-disabled={pending || undefined}
      className="flex h-12 w-full items-center justify-center gap-2.5 rounded-2xl bg-primary px-5 font-medium text-on-primary transition-colors hover:bg-primary-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-wait aria-disabled:bg-primary-strong"
    >
      {pending && (
        <span
          aria-hidden="true"
          className="size-4 shrink-0 rounded-full border-2 border-on-primary/30 border-t-on-primary motion-safe:animate-spin-calm"
        />
      )}
      {pending ? pendingLabel : children}
    </button>
  )
}
