import { ItemLink } from './ItemLink'
import type { ItemsRendererProps } from './types'

/** `key_value`: rows of label → value, for stats, scores and prices. */
export function KeyValueRenderer({ items }: ItemsRendererProps) {
  return (
    <dl className="divide-y divide-line">
      {items.map((item, i) => {
        const name = item.label || item.title
        if (item.value) {
          return (
            <div key={i} className="flex items-baseline justify-between gap-4 py-3 first:pt-0 last:pb-0">
              <dt className="min-w-0 break-words text-ink-soft">
                <ItemLink url={item.url}>{name || item.snippet}</ItemLink>
              </dt>
              <dd className="max-w-[60%] shrink-0 break-words text-right font-medium tabular-nums text-ink">
                {item.value}
              </dd>
            </div>
          )
        }
        // No value (e.g. raw search results when shaping failed): show the
        // name with its snippet underneath, so long text never lands in the
        // value column.
        return (
          <div key={i} className="py-3 first:pt-0 last:pb-0">
            <dt className="break-words font-medium leading-snug text-ink">
              <ItemLink url={item.url}>{name || item.snippet}</ItemLink>
            </dt>
            {name && item.snippet && (
              <dd className="mt-1 line-clamp-3 text-sm leading-relaxed text-ink-soft">{item.snippet}</dd>
            )}
          </div>
        )
      })}
    </dl>
  )
}
