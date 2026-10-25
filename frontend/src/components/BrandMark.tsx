// The lantern and kinlight wordmark, copied from brand/logo/themed/lockup.svg
// (re-copy it after brand/logo/build.py runs). Its colors are CSS variables:
// --mi the ink, --ml the light, --mg the glass core, set here from the
// theme's tokens so one drawing works in Daybreak and Porch light.
import lockup from '../brand/lockup.svg?raw'

interface BrandMarkProps {
  /** Shown quietly after the name, once the family has one. */
  familyName?: string
}

/** The Kinlight lantern and wordmark, optionally followed by the family's name. */
export function BrandMark({ familyName }: BrandMarkProps) {
  return (
    <p className="flex min-w-0 items-center gap-2 text-sm font-medium text-ink-muted">
      <span
        aria-hidden="true"
        className="flex h-8 shrink-0 [--mg:var(--color-glass-core)] [--mi:var(--color-ink)] [--ml:var(--color-accent)] [&>svg]:h-full [&>svg]:w-auto"
        // A static file from this repo, bundled at build time: no user input.
        dangerouslySetInnerHTML={{ __html: lockup }}
      />
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
