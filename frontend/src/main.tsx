import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { setUnauthorizedHandler } from './api/client'
import { endSession } from './api/hooks'
import { NotFound, RouteError, RoutePending } from './components/RouteStates'
// Generated from the files in src/routes by the TanStack Router Vite plugin.
import { routeTree } from './routeTree.gen'
// The typefaces ship with the app, so no font request leaves its server.
// Fraunces' full set has every axis, including the soft one headings use.
import '@fontsource-variable/fraunces/full.css'
import '@fontsource-variable/figtree'
import './styles/index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    // Every widget request costs LLM + search credit, so nothing refetches
    // or retries behind the user's back. The backend already retries once.
    queries: { refetchOnWindowFocus: false, retry: false },
    mutations: { retry: false },
  },
})

const router = createRouter({
  routeTree,
  // Handed to every route's beforeLoad and loader (see routes/__root.tsx).
  context: { queryClient },
  // Start loading a page when its link is hovered or focused.
  defaultPreload: 'intent',
  // TanStack Query owns the caching, so the router always asks it rather
  // than keeping its own copy of preloaded data.
  defaultPreloadStaleTime: 0,
  defaultPendingComponent: RoutePending,
  defaultErrorComponent: RouteError,
  defaultNotFoundComponent: NotFound,
  scrollRestoration: true,
})

// Registers this router's type, so <Link to>, navigate() and redirect() are
// checked against the real routes at compile time.
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// A 401 from any private endpoint means the session expired: forget it here
// too and show the login screen (spec > Important Failure Modes).
setUnauthorizedHandler(() => void endSession(queryClient, router))

const rootElement = document.getElementById('root')
if (!rootElement) {
  throw new Error('Kinlight: #root element is missing from index.html')
}

createRoot(rootElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
