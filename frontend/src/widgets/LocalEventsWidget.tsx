import { useEvents } from '../api/hooks'
import { CuratedWidgetCard } from './CuratedWidgetCard'

/**
 * The Local Events widget: upcoming events near you, picked from your
 * location and interests, with a refresh button. It loads its own data, so it
 * can sit anywhere on a page.
 */
export function LocalEventsWidget() {
  const events = useEvents()
  return (
    <CuratedWidgetCard
      result={events}
      title="Local events"
      noun="events"
      loadingMessage="Looking for events near you…"
      missing={{
        no_location: {
          message: 'Add your city to see events near you.',
          action: 'Add your city',
          field: 'location',
        },
      }}
    />
  )
}
