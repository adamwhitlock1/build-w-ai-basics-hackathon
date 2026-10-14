import { BriefingWidget } from '../../widgets/BriefingWidget'

/**
 * The morning briefing card, as placed on the page and in the Arrange
 * editor. It reads the same query wherever it's shown, so switching to
 * Arrange doesn't ask for it again.
 */
export function BriefingBlock() {
  return <BriefingWidget />
}
