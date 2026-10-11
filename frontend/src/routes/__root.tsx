import type { QueryClient } from '@tanstack/react-query'
import { Outlet, createRootRouteWithContext } from '@tanstack/react-router'

/** What every route's `beforeLoad` and `loader` receive as `context` (set up in main.tsx). */
export interface RouterContext {
  queryClient: QueryClient
}

/**
 * The root of every page. It adds no chrome of its own; each screen owns its
 * layout. Pending, error and not-found screens are the router's defaults
 * (see main.tsx).
 */
export const Route = createRootRouteWithContext<RouterContext>()({
  component: Outlet,
})
