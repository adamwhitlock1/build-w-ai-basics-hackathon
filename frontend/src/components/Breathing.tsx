import { useLight } from '../motion/useLight'
import { LanternMark } from './LanternMark'

/**
 * The app's one waiting light: a small lantern whose light breathes, every
 * 2.8 seconds (brand/lantern-ux.md, "Thinking"). It's the only animation
 * that repeats. It's decoration: whatever it sits beside must say what's
 * happening in words, inside a role="status" for screen readers. Its ink is
 * the surrounding text color. A wait is "thinking" for the one-lit-thing
 * rule: while another wait holds the glow, this one dims.
 */
export function Breathing({ className = 'size-5' }: { className?: string }) {
  const lit = useLight('thinking', true)
  return <LanternMark breathing dim={!lit} className={`shrink-0 overflow-visible ${className}`} />
}
