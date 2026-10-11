import { createContext } from 'react'

/**
 * Whose page some data belongs to: your own, or (for a parent) another
 * family member's. The server has the same idea. /api/me/... and
 * /api/widgets/... answer for you; the parents-only /api/members/{id}/...
 * run the very same code for member {id} instead.
 */
export type PageOwner =
  /** Your own page. `userId` is undefined only until /api/me has loaded. */
  | { isYou: true; userId: number | undefined }
  /** A family member's page, which a parent is looking at. */
  | { isYou: false; userId: number }

/**
 * Which family member's page the cards inside are on: null (the default) for
 * your own, or the member's user id while a parent views their page. The
 * page sets it once, around the cards, and the hooks the cards call
 * (useArticles, usePageHomepage, ...) read it through usePageOwner(). That
 * way the cards are exactly the same wherever they're shown, like the
 * interests editor's context (account/interestsEditorContext.ts).
 */
export const ViewedMemberContext = createContext<number | null>(null)
