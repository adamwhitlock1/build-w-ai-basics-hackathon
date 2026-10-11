import { ItemLink } from './ItemLink'
import { metaLine, primaryText } from './itemText'
import type { ItemsRendererProps } from './types'

/**
 * `list`: a vertical list; each row is a title (linked if it has a url), a
 * date-and-label line (an event's "Sat, Oct 12 · Red Rocks") and a snippet.
 */
export function ListRenderer({ items }: ItemsRendererProps) {
  return (
    <ul className="divide-y divide-line">
      {/* Index keys are fine: the list never reorders, and urls can repeat. */}
      {items.map((item, i) => {
        const meta = metaLine(item)
        return (
          <li key={i} className="py-3.5 first:pt-0 last:pb-0">
            <ItemLink url={item.url} className="font-medium leading-snug text-ink">
              {primaryText(item)}
            </ItemLink>
            {meta && <p className="mt-1 text-xs text-ink-muted">{meta}</p>}
            {item.title && item.snippet && (
              <p className="mt-1.5 line-clamp-3 text-sm leading-relaxed text-ink-soft">{item.snippet}</p>
            )}
          </li>
        )
      })}
    </ul>
  )
}
