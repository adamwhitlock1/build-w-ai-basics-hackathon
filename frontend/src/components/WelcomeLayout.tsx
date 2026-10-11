import type { ReactNode } from 'react'
import { BrandMark } from './BrandMark'

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

/** The calm, centred frame around login, sign-up, create-family and other one-off screens. */
export function WelcomeLayout({ title, intro, children, footer, wide = false }: WelcomeLayoutProps) {
  return (
    <div className="min-h-dvh px-4 py-12 sm:px-6 sm:py-20">
      <main className={`mx-auto w-full ${wide ? 'max-w-xl' : 'max-w-md'}`}>
        <BrandMark />
        <h1 className="mt-4 text-4xl leading-tight text-ink sm:text-5xl">{title}</h1>
        {intro && <p className="mt-3 max-w-prose text-ink-soft">{intro}</p>}
        <div className="mt-8 rounded-3xl border border-line bg-surface p-6 shadow-card sm:p-8">{children}</div>
        {footer && <div className="mt-6 text-center text-sm text-ink-soft">{footer}</div>}
      </main>
    </div>
  )
}
