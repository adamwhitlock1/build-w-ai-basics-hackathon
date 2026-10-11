import type { WidgetItem } from '../../api/client'

/**
 * The main text to show for an item: its title, else its snippet, else
 * "label: value". The model may fill in fields the archetype doesn't expect,
 * so renderers use this rather than show a blank row.
 */
export function primaryText(item: WidgetItem): string {
  return item.title || item.snippet || [item.label, item.value].filter(Boolean).join(': ')
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
