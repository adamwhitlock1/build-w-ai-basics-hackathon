import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './styles/index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    // Every widget request costs LLM + search credit, so nothing refetches
    // or retries behind the user's back. The backend already retries once.
    queries: { refetchOnWindowFocus: false, retry: false },
    mutations: { retry: false },
  },
})

const rootElement = document.getElementById('root')
if (!rootElement) {
  throw new Error('Porchlight: #root element is missing from index.html')
}

createRoot(rootElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
