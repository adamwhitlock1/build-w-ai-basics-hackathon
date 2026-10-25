import type { ReactNode } from 'react'
import { Breathing } from './Breathing'

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
      {pending && <Breathing className="size-5" />}
      {pending ? pendingLabel : children}
    </button>
  )
}
