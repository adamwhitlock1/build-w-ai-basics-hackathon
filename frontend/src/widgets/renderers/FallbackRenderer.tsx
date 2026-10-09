import { ItemLink } from './ItemLink'
import type { ItemsRendererProps } from './types'

/**
 * Generic card list for an archetype this build doesn't know yet.
 * Shows whatever fields each item has, so content is never lost.
 */
export function FallbackRenderer({ items }: ItemsRendererProps) {
  return (
    <ul className="space-y-3">
      {items.map((item, i) => {
        const meta = [item.date, item.label, item.value].filter(Boolean).join(' · ')
        return (
          <li key={i} className="rounded-2xl border border-line bg-bg/60 p-3.5">
            {item.title && (
              <ItemLink url={item.url} className="font-medium leading-snug text-ink">
                {item.title}
              </ItemLink>
            )}
            {meta && <p className="mt-1 text-xs text-ink-muted">{meta}</p>}
            {item.snippet && <p className="mt-1.5 line-clamp-3 text-sm leading-relaxed text-ink-soft">{item.snippet}</p>}
          </li>
        )
      })}
    </ul>
  )
}
