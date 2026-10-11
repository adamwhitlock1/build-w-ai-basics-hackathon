import type { WidgetItem } from '../../api/client'

/**
 * The main text to show for an item: its title, else its snippet, else
 * "label: value". The model may fill in fields the archetype doesn't expect,
 * so renderers use this rather than show a blank row.
 */
export function primaryText(item: WidgetItem): string {
  return item.title || item.snippet || [item.label, item.value].filter(Boolean).join(': ')
}

/** Letters and digits only, lowercased, so punctuation, quote marks and spacing don't matter. */
function normalize(text: string): string {
  return text.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, '')
}

/**
 * The item's snippet, unless it just repeats the title (the model sometimes
 * puts a quote in both). Empty when there's no title, since the snippet is
 * then the main text.
 */
export function detailText(item: WidgetItem): string {
  if (!item.title || !item.snippet) return ''
  return normalize(item.snippet) === normalize(item.title) ? '' : item.snippet
}

/**
 * A hero's big text and the line under it. When the title and snippet are
 * the same words, or one is a cut-short copy of the other (a quote the model
 * trimmed), the fuller one is shown once, without wrapping quote marks.
 */
export function heroText(item: WidgetItem): { display: string; detail: string } {
  const display = primaryText(item)
  if (!item.title || !item.snippet) return { display, detail: '' }
  const title = normalize(item.title)
  const snippet = normalize(item.snippet)
  // Containment only counts for real sentences, so a short title like
  // "Artemis II" doesn't swallow a snippet that merely mentions it.
  const overlaps =
    title === snippet || (Math.min(title.length, snippet.length) >= 20 && (title.includes(snippet) || snippet.includes(title)))
  if (!overlaps) return { display, detail: item.snippet }
  const fuller = snippet.length > title.length ? item.snippet : item.title
  return { display: fuller.trim().replace(/^["'“”‘’]+|["'“”‘’]+$/g, ''), detail: '' }
}

/**
 * The small line under an item's main text: its date and label, e.g.
 * "Sat, Oct 12 · Red Rocks" for an event or "nytimes.com · 2 hours ago" for
 * an article (`order` says which comes first). The label is left out when
 * primaryText already shows it.
 */
export function metaLine(item: WidgetItem, order: 'date-first' | 'label-first' = 'date-first'): string {
  const label = item.title || item.snippet ? item.label : ''
  const parts = order === 'date-first' ? [item.date, label] : [label, item.date]
  return parts.filter(Boolean).join(' · ')
}
