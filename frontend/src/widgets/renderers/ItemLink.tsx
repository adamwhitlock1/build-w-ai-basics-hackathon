import type { ReactNode } from 'react'
import { safeUrl } from './safeUrl'

interface ItemLinkProps {
  url: string
  className?: string
  children: ReactNode
}

/** Renders children as an external link when `url` is safe, or as plain text otherwise. */
export function ItemLink({ url, className = '', children }: ItemLinkProps) {
  const href = safeUrl(url)
  if (!href) {
    return <span className={className}>{children}</span>
  }
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className={`underline-offset-4 decoration-primary/40 hover:text-primary hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${className}`}
    >
      {children}
    </a>
  )
}
