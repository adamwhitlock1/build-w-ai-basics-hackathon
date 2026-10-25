import type { QueryClient } from '@tanstack/react-query'
import { Outlet, createRootRouteWithContext } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useMe } from '../api/hooks'
import { applyTheme } from '../theme'

/** What every route's `beforeLoad` and `loader` receive as `context` (set up in main.tsx). */
export interface RouterContext {
  queryClient: QueryClient
}

/**
 * The root of every page. It adds no chrome of its own; each screen owns its
 * layout. Pending, error and not-found screens are the router's defaults
 * (see main.tsx). It keeps the page on the logged-in member's theme.
 */
export const Route = createRootRouteWithContext<RouterContext>()({
  component: Root,
})

function Root() {
  const theme = useMe().data?.user.theme
  useEffect(() => {
    if (theme) applyTheme(theme)
  }, [theme])
  return <Outlet />
}
