import type { WidgetItem } from '../../api/client'

/**
 * The main text to show for an item: its title, else its snippet, else
 * "label: value". The model may fill in fields the archetype doesn't expect,
 * so renderers use this rather than show a blank row.
 */
export function primaryText(item: WidgetItem): string {
  return item.title || item.snippet || [item.label, item.value].filter(Boolean).join(': ')
}
