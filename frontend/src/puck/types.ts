import type { Config, Data, Slot } from '@puckeditor/core'
import type { Span } from '../api/client'

/**
 * The props each kind of card stores in the layout, besides the `id` Puck
 * gives every card. These match LayoutBlock in api/client.ts.
 */
export interface BlockPropsMap {
  Articles: { span: Span }
  LocalEvents: { span: Span }
  Custom: { span: Span; widgetId: number }
}

/** The page itself (Puck's "root") has one prop: the slot holding the cards. */
export interface RootProps {
  blocks: Slot
}

/** Our Puck configuration's type: which cards exist and what they store. */
export type PorchlightConfig = Config<{ components: BlockPropsMap; root: RootProps }>

/** A layout as Puck sees it while arranging. */
export type PorchlightData = Data<BlockPropsMap, RootProps>
