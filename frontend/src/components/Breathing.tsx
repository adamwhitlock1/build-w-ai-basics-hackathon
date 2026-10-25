import { LanternMark } from './LanternMark'

/**
 * The app's one waiting light: a small lantern whose light breathes, every
 * 2.8 seconds (brand/lantern-ux.md, "Thinking"). It's the only animation
 * that repeats. It's decoration: whatever it sits beside must say what's
 * happening in words, inside a role="status" for screen readers. Its ink is
 * the surrounding text color.
 */
export function Breathing({ className = 'size-5' }: { className?: string }) {
  return <LanternMark breathing className={`shrink-0 overflow-visible ${className}`} />
}
