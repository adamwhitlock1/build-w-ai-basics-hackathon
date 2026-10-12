import { TodayWidget } from '../../widgets/TodayWidget'

/**
 * The Today card, as placed on the page and in the Arrange editor. Like the
 * other cards, it reads its events through the same query wherever it's
 * shown, so switching to Arrange doesn't fetch the calendar again.
 */
export function TodayBlock() {
  return <TodayWidget />
}
