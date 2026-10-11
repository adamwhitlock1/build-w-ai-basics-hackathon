import { createContext, useContext } from 'react'

/** A field of the interests editor that can take focus when it opens. */
export type ProfileField = 'interests' | 'location'

/** Opens the interests editor with the cursor in `field` (interests by default). */
export type OpenInterestsEditor = (field?: ProfileField) => void

/**
 * Lets a widget open the page's interests editor (from an "Add your city"
 * button, say) without the page handing it a callback. Widgets stay free of
 * page wiring that way, so they can become drag-and-drop blocks as they are.
 * Null where no page provides an editor.
 */
export const InterestsEditorContext = createContext<OpenInterestsEditor | null>(null)

/** The page's "open the interests editor" function, or null when there's no editor here. */
export function useOpenInterestsEditor(): OpenInterestsEditor | null {
  return useContext(InterestsEditorContext)
}
