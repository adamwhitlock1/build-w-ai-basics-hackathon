import { useState } from 'react'
import type { WidgetItem } from '../../api/client'
import { primaryText } from './itemText'
import { safeUrl } from './safeUrl'
import type { ItemsRendererProps } from './types'

/**
 * `cards`: a grid of small cards with an optional thumbnail, title and snippet.
 * Columns follow the widget's own width (container queries), not the
 * viewport's, so it adapts when a widget is resized on the page.
 */
export function CardsRenderer({ items }: ItemsRendererProps) {
  return (
    <ul className="grid grid-cols-1 gap-3 @md:grid-cols-2 @3xl:grid-cols-3">
      {items.map((item, i) => (
        <li key={i}>
          <SmallCard item={item} />
        </li>
      ))}
    </ul>
  )
}

function SmallCard({ item }: { item: WidgetItem }) {
  const href = safeUrl(item.url)
  const body = (
    <>
      <Thumbnail src={item.thumbnail} />
      <div className="p-3.5">
        <p className="line-clamp-2 font-medium leading-snug text-ink">{primaryText(item)}</p>
        {item.date && <p className="mt-1 text-xs text-ink-muted">{item.date}</p>}
        {item.title && item.snippet && (
          <p className="mt-1.5 line-clamp-3 text-sm leading-relaxed text-ink-soft">{item.snippet}</p>
        )}
      </div>
    </>
  )
  const frame = 'block h-full overflow-hidden rounded-2xl border border-line bg-bg/60'

  if (!href) {
    return <div className={frame}>{body}</div>
  }
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className={`${frame} transition-colors hover:border-primary/40 hover:bg-primary-soft/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary`}
    >
      {body}
    </a>
  )
}

/** A decorative thumbnail that quietly disappears if missing, unsafe or broken. */
function Thumbnail({ src }: { src: string }) {
  const [failed, setFailed] = useState(false)
  const href = safeUrl(src)
  if (!href || failed) return null
  return (
    <img
      src={href}
      alt=""
      loading="lazy"
      referrerPolicy="no-referrer"
      onError={() => setFailed(true)}
      className="aspect-[16/9] w-full bg-line object-cover"
    />
  )
}
