import type { ReactNode } from 'react'

interface FormErrorProps {
  /** What went wrong. */
  message: string
  /** What to do next. */
  hint?: string
  /** An optional next step, such as a link. */
  children?: ReactNode
}

/** An inline error that screen readers announce as soon as it appears. */
export function FormError({ message, hint, children }: FormErrorProps) {
  return (
    <div role="alert" className="rounded-2xl bg-danger-soft px-4 py-3 text-sm">
      <p className="font-medium text-danger">{message}</p>
      {hint && <p className="mt-0.5 text-ink-soft">{hint}</p>}
      {children}
    </div>
  )
}
