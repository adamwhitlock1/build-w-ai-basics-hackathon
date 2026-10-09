import type { Archetype, WidgetItem } from '../../api/client'
import { CardsRenderer } from './CardsRenderer'
import { FallbackRenderer } from './FallbackRenderer'
import { HeroRenderer } from './HeroRenderer'
import { KeyValueRenderer } from './KeyValueRenderer'
import { ListRenderer } from './ListRenderer'
import { TableRenderer } from './TableRenderer'

interface WidgetItemsProps {
  archetype: Archetype
  items: WidgetItem[]
}

/** Lays out a widget's items according to its archetype. */
export function WidgetItems({ archetype, items }: WidgetItemsProps) {
  switch (archetype) {
    case 'list':
      return <ListRenderer items={items} />
    case 'cards':
      return <CardsRenderer items={items} />
    case 'hero':
      return <HeroRenderer items={items} />
    case 'key_value':
      return <KeyValueRenderer items={items} />
    case 'table':
      return <TableRenderer items={items} />
    default:
      // The type only allows known archetypes, but this runs on live server
      // data: anything unexpected gets a generic card list, not a blank card.
      return <FallbackRenderer items={items} />
  }
}
