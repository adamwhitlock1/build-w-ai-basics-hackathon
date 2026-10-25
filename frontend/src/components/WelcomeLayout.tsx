import type { ReactNode } from 'react'
import { BrandMark } from './BrandMark'
import { PorchScene } from './PorchScene'

interface WelcomeLayoutProps {
  title: string
  intro?: ReactNode
  /** Goes on the card. */
  children: ReactNode
  /** Small print under the card, e.g. the login ⇄ sign-up switch. */
  footer?: ReactNode
  /** A roomier card, for longer forms. */
  wide?: boolean
}

/**
 * The calm, centred frame around login, sign-up, create-family and other
 * one-off screens: a brand surface, so its ground is porch boards with faint
 * grain, and the porch scene sits under the card. The card itself is plain.
 */
export function WelcomeLayout({ title, intro, children, footer, wide = false }: WelcomeLayoutProps) {
  return (
    <div className="porch-boards wood-grain relative min-h-dvh px-4 py-12 sm:px-6 sm:py-20">
      <main className={`mx-auto w-full ${wide ? 'max-w-xl' : 'max-w-md'}`}>
        <BrandMark />
        {/* break-words: a long name in the title (say, the family's on an
            invite) wraps instead of pushing the page sideways on a phone. */}
        <h1 className="mt-4 text-4xl leading-tight break-words text-ink sm:text-5xl">{title}</h1>
        {intro && <p className="mt-3 max-w-prose text-ink-soft">{intro}</p>}
        <div className="mt-8 rounded-3xl border border-line bg-surface p-6 shadow-card sm:p-8">{children}</div>
        {footer && <div className="mt-6 text-center text-sm text-ink-soft">{footer}</div>}
        <PorchScene slot="wide" className="mt-12 aspect-[2/1] w-full overflow-hidden rounded-3xl" />
      </main>
    </div>
  )
}
