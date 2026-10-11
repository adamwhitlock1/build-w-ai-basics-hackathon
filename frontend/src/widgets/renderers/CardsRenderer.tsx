import { useState } from 'react'
import type { WidgetItem } from '../../api/client'
import { metaLine, primaryText } from './itemText'
import { safeUrl } from './safeUrl'
import type { ItemsRendererProps } from './types'

/**
 * `cards`: a grid of small cards, each a title, a source-and-date line and
 * a snippet, with an optional thumbnail beside them. The thumbnail sits
 * beside the text rather than above it, so five articles with pictures
 * still fit on about one screen.
 * Columns follow the widget's own width (container queries), not the
 * viewport's, so it adapts when a widget is resized on the page. A second
 * column comes only once each card has room for a readable line of text
 * next to its thumbnail.
 */
export function CardsRenderer({ items }: ItemsRendererProps) {
  return (
    <ul className="grid grid-cols-1 gap-3 @2xl:grid-cols-2 @5xl:grid-cols-3">
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
  const meta = metaLine(item, 'label-first')
  const body = (
    <div className="flex gap-3 p-3.5">
      {/* min-w-0 lets long words wrap instead of pushing the thumbnail out. */}
      <div className="min-w-0 flex-1">
        <p className="line-clamp-2 font-medium leading-snug text-ink">{primaryText(item)}</p>
        {meta && <p className="mt-1 text-xs text-ink-muted">{meta}</p>}
        {item.title && item.snippet && (
          <p className="mt-1.5 line-clamp-2 text-sm leading-relaxed text-ink-soft">{item.snippet}</p>
        )}
      </div>
      <Thumbnail src={item.thumbnail} />
    </div>
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

/**
 * A small square thumbnail, cropped to fit. It's decorative, so it quietly
 * disappears if missing, unsafe or broken.
 */
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
      className="size-16 shrink-0 rounded-xl bg-line object-cover @sm:size-20"
    />
  )
}
