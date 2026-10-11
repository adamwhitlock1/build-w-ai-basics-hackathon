interface BrandMarkProps {
  /** Shown quietly after the name, once the family has one. */
  familyName?: string
}

/** "Porchlight" with its small amber light-on dot, optionally followed by the family's name. */
export function BrandMark({ familyName }: BrandMarkProps) {
  return (
    <p className="flex min-w-0 items-center gap-2 text-sm font-medium text-ink-muted">
      <span
        aria-hidden="true"
        className="size-2.5 shrink-0 rounded-full bg-accent shadow-[0_0_0_4px] shadow-accent-soft"
      />
      <span className="shrink-0">Porchlight</span>
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
