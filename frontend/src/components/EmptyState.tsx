import type { ReactNode } from 'react'
import { PorchScene } from './PorchScene'

/**
 * An empty state: the porch scene's small slot beside a sentence saying
 * what's missing and what to do, on faint wood grain (a brand surface, with
 * no content behind it).
 */
export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="wood-grain relative flex items-center gap-4 overflow-hidden rounded-3xl border border-line p-4 sm:gap-5 sm:p-5">
      <PorchScene slot="small" className="h-14 w-28 shrink-0 overflow-hidden rounded-xl sm:h-16 sm:w-32" />
      <div className="min-w-0 text-ink-soft">{children}</div>
    </div>
  )
}
