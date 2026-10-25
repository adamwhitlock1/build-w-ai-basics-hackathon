// The porch illustration (brand/brief.md, "Imagery and illustration"), in
// two slots: wide across the welcome pages, small in empty states. The
// drawing is src/brand/porch.svg, today a plain stand-in for a commissioned
// scene; brand/illustration.md is the format a replacement follows, so
// swapping the file is the whole job. Its colors are CSS variables set here
// from the theme, so it follows Daybreak and Porch light.
import porch from '../brand/porch.svg?raw'

const FIT = {
  // Fills the width and keeps the floor in view, cropping the ceiling.
  wide: 'xMidYMax slice',
  // The whole scene, centred.
  small: 'xMidYMid meet',
} as const

const markup = {
  wide: porch.replace('<svg ', `<svg preserveAspectRatio="${FIT.wide}" `),
  small: porch.replace('<svg ', `<svg preserveAspectRatio="${FIT.small}" `),
}

interface PorchSceneProps {
  slot: keyof typeof FIT
  className?: string
}

/** The porch scene: decoration only, so it's hidden from screen readers and never carries words. */
export function PorchScene({ slot, className = '' }: PorchSceneProps) {
  return (
    <span
      aria-hidden="true"
      className={`block [--pd:var(--color-wood-deep)] [--pg:var(--color-glass-core)] [--ph:var(--color-primary-soft)] [--pi:var(--color-ink)] [--pl:var(--color-accent)] [--pw:var(--color-wood)] [&>svg]:block [&>svg]:size-full ${className}`}
      // A static file from this repo, bundled at build time: no user input.
      dangerouslySetInnerHTML={{ __html: markup[slot] }}
    />
  )
}
