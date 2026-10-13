import { ArrangeFrame } from './ArrangeFrame'
import { ArticlesBlock } from './blocks/Articles'
import { CustomBlock } from './blocks/Custom'
import { LocalEventsBlock } from './blocks/LocalEvents'
import { TodayBlock } from './blocks/Today'
import { DEFAULT_SPAN } from './layout'
import type { PorchlightConfig } from './types'

/**
 * Puck's map of the page for the Arrange editor: which kinds of card exist
 * and how each one renders while arranging. (The normal page doesn't use
 * Puck; see LayoutView.tsx.)
 *
 * The page itself (Puck's "root") holds one slot field, `blocks`: the list
 * of cards, laid out as a four-column grid. Puck lets you drag the cards
 * around inside it; ArrangeFrame adds resizing on top.
 */
export const config: PorchlightConfig = {
  root: {
    fields: { blocks: { type: 'slot' } },
    // `blocks` arrives as a component that renders the slot's cards. Its
    // className styles the grid element itself, so the cards are its items.
    render: ({ blocks: Blocks }) => <Blocks className="grid grid-cols-4 items-start gap-6" minEmptyHeight={200} />,
  },
  components: {
    // Every card is `inline` (Puck adds no wrapper, so ArrangeFrame is the
    // grid item) and stores only its width, plus a widget id for Custom.
    // `defaultProps` fill in anything a stored card is missing. Puck's own
    // delete (its Backspace and Delete keys) is off for every card: the page
    // always has its one Today, one Articles and one Local Events card, and a widget
    // you described leaves only through "Remove" in the action bar, which
    // deletes the widget itself (see BlockActionBar). Nothing can be
    // duplicated (see ArrangeCanvas).
    Today: {
      label: 'Today',
      inline: true,
      defaultProps: { span: 4 },
      permissions: { delete: false },
      render: ({ id, span, puck }) => (
        <ArrangeFrame id={id} span={span} dragRef={puck.dragRef} editing={puck.isEditing}>
          <TodayBlock />
        </ArrangeFrame>
      ),
    },
    Articles: {
      label: 'Articles',
      inline: true,
      defaultProps: { span: DEFAULT_SPAN },
      permissions: { delete: false },
      render: ({ id, span, puck }) => (
        <ArrangeFrame id={id} span={span} dragRef={puck.dragRef} editing={puck.isEditing}>
          <ArticlesBlock />
        </ArrangeFrame>
      ),
    },
    LocalEvents: {
      label: 'Local events',
      inline: true,
      defaultProps: { span: DEFAULT_SPAN },
      permissions: { delete: false },
      render: ({ id, span, puck }) => (
        <ArrangeFrame id={id} span={span} dragRef={puck.dragRef} editing={puck.isEditing}>
          <LocalEventsBlock />
        </ArrangeFrame>
      ),
    },
    Custom: {
      label: 'Your widget',
      inline: true,
      // No real widget has id 0, so a card missing its id says "gone".
      defaultProps: { span: DEFAULT_SPAN, widgetId: 0 },
      permissions: { delete: false },
      render: ({ id, span, widgetId, puck }) => (
        <ArrangeFrame id={id} span={span} dragRef={puck.dragRef} editing={puck.isEditing}>
          <CustomBlock widgetId={widgetId} />
        </ArrangeFrame>
      ),
    },
  },
}
