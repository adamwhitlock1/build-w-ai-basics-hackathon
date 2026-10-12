import { useEvents, usePageOwner } from '../api/hooks'
import { CuratedWidgetCard } from './CuratedWidgetCard'

/**
 * The Local Events widget: upcoming events near whoever's page it's on,
 * picked from their location and interests, with a refresh button. It loads
 * its own data, so it can sit anywhere on a page.
 */
export function LocalEventsWidget() {
  const events = useEvents()
  // On a family member's page, a parent reads it, so it speaks about them.
  const { isYou } = usePageOwner()
  return (
    <CuratedWidgetCard
      result={events}
      title={isYou ? 'Happening near you' : 'Happening near them'}
      noun="events"
      loadingMessage={isYou ? 'Looking for events near you…' : 'Looking for events near them…'}
      missing={{
        no_location: isYou
          ? { message: 'Add your city to see events near you.', action: 'Add your city', field: 'location' }
          : { message: 'Add their city to see events near them.', action: 'Add their city', field: 'location' },
      }}
    />
  )
}
