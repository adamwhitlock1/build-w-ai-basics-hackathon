// The header lockup: the lantern (LanternMark, which can move) beside the
// kinlight wordmark, copied from brand/logo/themed/wordmark.svg (re-copy it
// after brand/logo/build.py runs). The wordmark's colors are CSS variables:
// --mi the ink, --ml the light (the lit dot on the i), --mg the glass core,
// set here from the theme. The sizes follow brand/logo/lockup.svg: the
// lantern is about 96% of the lockup's height, the wordmark about 98%, with a
// gap of about 15% of the height between them.
import wordmark from '../brand/wordmark.svg?raw'
import { LanternMark } from './LanternMark'

interface BrandMarkProps {
  /** Shown quietly after the name, once the family has one. */
  familyName?: string
}

/** The Kinlight lantern and wordmark, optionally followed by the family's name. */
export function BrandMark({ familyName }: BrandMarkProps) {
  return (
    <p className="flex min-w-0 items-center gap-2 text-sm font-medium text-ink-muted">
      <span aria-hidden="true" className="flex h-8 shrink-0 items-center gap-[0.3rem] text-ink">
        <LanternMark header className="h-[96%] w-auto overflow-visible" />
        <span
          className="flex h-[98%] [--mg:var(--color-glass-core)] [--mi:var(--color-ink)] [--ml:var(--color-accent)] [&>svg]:h-full [&>svg]:w-auto"
          // A static file from this repo, bundled at build time: no user input.
          dangerouslySetInnerHTML={{ __html: wordmark }}
        />
      </span>
      <span className="sr-only">Kinlight</span>
      {familyName && (
        <>
          <span aria-hidden="true" className="shrink-0 text-line">
            /
          </span>
          <span className="truncate font-normal" title={familyName}>
            {familyName}
          </span>
        </>
      )}
    </p>
  )
}
