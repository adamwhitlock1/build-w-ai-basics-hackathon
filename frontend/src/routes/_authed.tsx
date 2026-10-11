import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQueryOptions } from '../api/hooks'

/**
 * A pathless layout ("_" prefix: it adds no URL segment) that guards every
 * page inside src/routes/_authed. Logged out → /login; family not set up yet
 * → /create-family. This is only for a smooth experience: the server checks
 * the session on every private request itself.
 */
export const Route = createFileRoute('/_authed')({
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.query(meQueryOptions)
    if (!me) throw redirect({ to: '/login' })
    if (me.needsFamilySetup) throw redirect({ to: '/create-family' })
  },
  // No component: the matched child page renders in its place.
})
