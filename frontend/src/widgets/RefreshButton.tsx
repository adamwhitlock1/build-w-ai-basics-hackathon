import { forwardRef } from 'react'
import { Breathing } from '../components/Breathing'
import { RefreshIcon } from './icons'

interface RefreshButtonProps {
  /** Names the button for screen readers and the hover tooltip, e.g. "Refresh articles". */
  label: string
  /** True while loading: the breathing light replaces the icon and clicks are ignored. */
  refreshing: boolean
  onRefresh: () => void
}

/**
 * A quiet round refresh button for a widget's header. While refreshing it
 * uses aria-disabled rather than disabled, so keyboard focus stays on it.
 * forwardRef lets the widget move focus here (React 18 doesn't pass `ref`
 * through as an ordinary prop).
 */
export const RefreshButton = forwardRef<HTMLButtonElement, RefreshButtonProps>(function RefreshButton(
  { label, refreshing, onRefresh },
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      aria-label={label}
      title={label}
      aria-disabled={refreshing || undefined}
      onClick={() => {
        if (!refreshing) onRefresh()
      }}
      className="rounded-full p-2 text-ink-muted transition-colors hover:bg-bg hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-disabled:cursor-wait aria-disabled:text-primary aria-disabled:hover:bg-transparent"
    >
      {refreshing ? <Breathing className="size-5" /> : <RefreshIcon className="size-5" />}
    </button>
  )
})
