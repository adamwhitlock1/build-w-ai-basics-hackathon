import type { WidgetItem } from '../../api/client'

/** Props shared by every archetype renderer. `items` is never empty here. */
export interface ItemsRendererProps {
  items: WidgetItem[]
}
