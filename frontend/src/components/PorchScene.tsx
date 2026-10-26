// The wide artwork follows data-theme in CSS, including saved choices and
// Automatic mode. Empty states keep the simpler, theme-colored SVG.
import porch from '../brand/porch.svg?raw'

const smallPorch = porch.replace('<svg ', '<svg preserveAspectRatio="xMidYMid meet" ')

interface PorchSceneProps {
  slot: 'wide' | 'small'
  className?: string
}

/** The porch scene: decoration only, so it's hidden from screen readers and never carries words. */
export function PorchScene({ slot, className = '' }: PorchSceneProps) {
  if (slot === 'wide') {
    return (
      <span
        aria-hidden="true"
        className={`porch-scene block aspect-[2/1] bg-contain bg-center bg-no-repeat ${className}`}
      />
    )
  }

  return (
    <span
      aria-hidden="true"
      className={`block [--pd:var(--color-wood-deep)] [--pg:var(--color-glass-core)] [--ph:var(--color-primary-soft)] [--pi:var(--color-ink)] [--pl:var(--color-accent)] [--pw:var(--color-wood)] [&>svg]:block [&>svg]:size-full ${className}`}
      // A static file from this repo, bundled at build time: no user input.
      dangerouslySetInnerHTML={{ __html: smallPorch }}
    />
  )
}
