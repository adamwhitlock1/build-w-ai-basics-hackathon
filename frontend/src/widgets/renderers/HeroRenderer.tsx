import { primaryText } from './itemText'
import { safeUrl } from './safeUrl'
import type { ItemsRendererProps } from './types'

/**
 * `hero`: one featured item. The title is the big display text, the snippet
 * sits below, and the label is the attribution (e.g. a quote and its author).
 */
export function HeroRenderer({ items }: ItemsRendererProps) {
  const [item] = items
  const display = primaryText(item)
  const snippet = item.title ? item.snippet : ''
  // When there's no title or snippet, the label is already part of display.
  const attribution = item.title || item.snippet ? item.label : ''
  const href = safeUrl(item.url)

  return (
    <figure>
      <p className="font-display text-2xl leading-snug text-ink @md:text-3xl">{display}</p>
      {snippet && <p className="mt-4 leading-relaxed text-ink-soft">{snippet}</p>}
      {(attribution || item.date || href) && (
        <figcaption className="mt-5 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-sm text-ink-muted">
          {attribution && <span className="font-medium text-ink-soft">— {attribution}</span>}
          {item.date && <span>{item.date}</span>}
          {href && (
            <a
              href={href}
              target="_blank"
              rel="noreferrer"
              className="text-primary underline-offset-4 hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
              Source
            </a>
          )}
        </figcaption>
      )}
    </figure>
  )
}
