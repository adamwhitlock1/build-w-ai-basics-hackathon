import { LocalEventsWidget } from '../../widgets/LocalEventsWidget'

/**
 * The Local Events card, as placed on the page and in the Arrange editor. Like
 * the Articles card, it shares one query with every other place it's shown,
 * so arranging the page costs no new search.
 */
export function LocalEventsBlock() {
  return <LocalEventsWidget />
}
