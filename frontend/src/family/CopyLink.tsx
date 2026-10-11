import { forwardRef, useEffect, useRef, useState } from 'react'

/** How long "Copied" shows before the button reads "Copy link" again. */
const COPIED_MS = 2000

/** What to say when the browser won't let us copy, so the link can be copied by hand. */
export const COPY_BY_HAND = 'Your browser didn’t let us copy it, so the link is selected for you: press Ctrl+C (⌘C on a Mac).'

interface CopyLinkButtonProps {
  /** The full link to copy. */
  link: string
  /**
   * What screen readers hear, e.g. "Copy link for sam@example.com". It should
   * start with the visible words, "Copy link", so voice control finds it.
   */
  label?: string
  /** Called once it's copied, e.g. to announce it. */
  onCopied?: () => void
  /**
   * Called when the browser won't copy (the clipboard is missing on pages
   * that aren't served securely, and it can be refused), so the caller can
   * show the link to copy by hand.
   */
  onFailed: () => void
  /** Looks: a quiet text button (in lists) or an outlined one (beside a link). */
  variant?: 'quiet' | 'outlined'
}

const BUTTON_STYLES = {
  quiet:
    'rounded-lg px-2 py-1 text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary',
  outlined:
    'h-11 shrink-0 rounded-2xl border border-primary/30 bg-surface px-4 text-sm font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary',
}

/** A "Copy link" button that reads "Copied" for a moment after it works. */
export const CopyLinkButton = forwardRef<HTMLButtonElement, CopyLinkButtonProps>(function CopyLinkButton(
  { link, label, onCopied, onFailed, variant = 'quiet' },
  ref,
) {
  const [copied, setCopied] = useState(false)

  // "Copied" goes back to "Copy link" after a moment.
  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), COPIED_MS)
    return () => window.clearTimeout(timer)
  }, [copied])

  async function copy() {
    try {
      await navigator.clipboard.writeText(link)
    } catch {
      onFailed()
      return
    }
    setCopied(true)
    onCopied?.()
  }

  return (
    <button
      ref={ref}
      type="button"
      aria-label={label}
      onClick={() => void copy()}
      className={`inline-flex items-center gap-1.5 ${BUTTON_STYLES[variant]}`}
    >
      {copied && <CheckIcon />}
      {copied ? 'Copied' : 'Copy link'}
    </button>
  )
})

interface LinkBoxProps {
  link: string
  /** What screen readers hear for the copy button. */
  label?: string
  onCopied?: () => void
}

/**
 * A link in a read-only box with a "Copy link" button beside it. If the
 * browser won't copy, the link in the box is selected for copying by hand.
 */
export const LinkBox = forwardRef<HTMLButtonElement, LinkBoxProps>(function LinkBox({ link, label, onCopied }, ref) {
  // How many times copying failed; each one selects the link again.
  const [failures, setFailures] = useState(0)

  return (
    <div className="flex flex-wrap items-start gap-2">
      <div className="min-w-0 flex-1 basis-40">
        <LinkField link={link} selectRequest={failures} />
      </div>
      <CopyLinkButton
        ref={ref}
        link={link}
        label={label}
        onCopied={onCopied}
        onFailed={() => setFailures((n) => n + 1)}
        variant="outlined"
      />
      {failures > 0 && (
        <p role="alert" className="w-full text-xs leading-relaxed text-ink-muted">
          {COPY_BY_HAND}
        </p>
      )}
    </div>
  )
})

interface LinkFieldProps {
  link: string
  /**
   * Each new number puts focus in the field with the link selected, ready to
   * copy by hand; 0 (the default) doesn't.
   */
  selectRequest?: number
}

/**
 * The link as read-only text in an input, so it's easy to select (focusing
 * it selects all of it) and a long link scrolls instead of breaking the
 * layout.
 */
export function LinkField({ link, selectRequest = 0 }: LinkFieldProps) {
  const ref = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (selectRequest > 0) ref.current?.focus() // onFocus below selects it
  }, [selectRequest])

  return (
    <input
      ref={ref}
      type="text"
      readOnly
      value={link}
      aria-label="Invite link"
      onFocus={(e) => e.currentTarget.select()}
      className="h-11 w-full rounded-2xl border border-line bg-bg px-3 font-mono text-xs text-ink-soft focus:border-primary/50 focus:bg-surface focus:outline-none focus:ring-4 focus:ring-primary/10"
    />
  )
}

function CheckIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 20 20" fill="none" className="size-4">
      <path d="M5 10.5l3 3 7-7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
