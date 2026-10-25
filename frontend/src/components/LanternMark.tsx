// The Kinlight lantern, drawn from brand/logo/themed/lantern.svg on its
// 64-unit grid, with each lit part its own element so the lantern's moments
// (brand/lantern-ux.md) can animate them with opacity and transform alone:
// the glass is an unlit pane under a lit one, the flame is its own shape, and
// the glow is two rings of light. The ink is currentColor, so the lantern
// takes the text color wherever it sits; the light is the theme's amber.

interface LanternMarkProps {
  className?: string
  /** Kindle: the glass warms with one small flicker, the flame rises and the glow spreads. */
  kindling?: boolean
  /** Thinking: the light brightens and settles, on a loop. */
  breathing?: boolean
  /** Swing on the ring and settle; change `swingKey` to swing again. */
  swingKey?: number
  /** Marks the header lantern, where the dusk spreads from. */
  header?: boolean
}

const GLASS = 'M23.5 24.5 L40.5 24.5 L38 45 L26 45 Z'
const CAP = 'M32 10.5 L18 21 L46 21 Z'
// Each lit part scales or moves around its own centre, in the drawing's units.
const AROUND_GLOW = { transformBox: 'view-box', transformOrigin: '32px 34.5px' } as const
const AROUND_RING = { transformBox: 'view-box', transformOrigin: '32px 6.5px' } as const

export function LanternMark({ className, kindling, breathing, swingKey, header }: LanternMarkProps) {
  const glow = kindling ? 'animate-kindle-glow' : breathing ? 'animate-breathe-glow' : ''
  const glass = kindling ? 'animate-kindle-glass' : breathing ? 'animate-breathe' : ''
  const flame = kindling ? 'animate-kindle-flame' : breathing ? 'animate-breathe' : ''
  return (
    <svg
      viewBox="0 0 64 64"
      aria-hidden="true"
      focusable="false"
      className={className}
      data-header-lantern={header ? '' : undefined}
    >
      {/* Swinging restarts whenever swingKey changes, since the group remounts. */}
      <g key={swingKey} className={swingKey ? 'animate-swing' : undefined} style={AROUND_RING}>
        <g className={glow} style={AROUND_GLOW}>
          <circle cx="32" cy="34.5" r="22" fill="var(--color-accent)" opacity="0.12" />
          <circle cx="32" cy="34.5" r="14.5" fill="var(--color-accent)" opacity="0.24" />
        </g>
        <circle cx="32" cy="6.5" r="2.6" fill="none" stroke="currentColor" strokeWidth="2" />
        <rect x="31" y="8.8" width="2" height="2.6" fill="currentColor" />
        <path d={CAP} fill="currentColor" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
        <rect x="19.5" y="20.5" width="25" height="4" rx="1.6" fill="currentColor" />
        {/* The pane before it's lit, then the light in it. */}
        <path d={GLASS} fill="currentColor" opacity="0.18" />
        <path d={GLASS} fill="var(--color-accent)" className={glass} />
        <ellipse cx="32" cy="37" rx="3.4" ry="5.4" fill="var(--color-glass-core)" className={flame} />
        <path d={`${GLASS} M32 24.5 V45`} fill="none" stroke="currentColor" strokeWidth="3" strokeLinejoin="round" />
        <rect x="24.5" y="44.5" width="15" height="4" rx="1.6" fill="currentColor" />
        <rect x="28.5" y="48.5" width="7" height="3" rx="1.2" fill="currentColor" />
      </g>
    </svg>
  )
}
