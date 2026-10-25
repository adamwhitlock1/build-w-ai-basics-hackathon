import { createUsePuck } from '@puckeditor/core'
import type { KinlightConfig } from './types'

/**
 * Reads the Arrange editor's state, typed for our cards. Pass a selector, e.g.
 * `useKinlightPuck((s) => s.selectedItem)`, and the component re-renders
 * only when that part changes.
 *
 * Like every Puck hook it throws outside <Puck>, so only editor-only pieces
 * (the action bar, the resize handle) may call it.
 */
export const useKinlightPuck = createUsePuck<KinlightConfig>()
