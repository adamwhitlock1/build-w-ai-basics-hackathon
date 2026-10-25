import { demoPaused } from './demoState'
import { usePublicConfig } from './config'

export function DemoNotice() {
  const { data } = usePublicConfig()
  if (!data?.demo) return null
  const q = data.quota
  const exhausted = demoPaused(q)
  return <aside aria-label="Judge demo" className="border-b border-line bg-primary-soft px-4 py-3 text-sm text-ink">
    <div className="mx-auto max-w-7xl">
      <strong>Rivera family · shared judge demo.</strong> This is fictional, resettable data. Other visitors can see your changes and chats. Please don’t enter personal information.
      <p className="mt-1">Initial cards, calendar events, conversations titled “Sample” and the sample briefing are prepared fictional examples. New AI responses are live. Judges: use the parent or kid credentials in the submission’s testing instructions.</p>
      {q && <p className="mt-1" role="status">{exhausted ? 'Demo allowance reached. Some live actions are paused; existing content is still available.' : `${q.modelsRemaining} AI calls and ${q.searchesRemaining} searches left today.`} Allowance resets {new Date(q.reset).toLocaleString(undefined, { timeZone: 'America/Denver', hour: 'numeric', minute: '2-digit', month: 'short', day: 'numeric' })} Denver time.</p>}
    </div>
  </aside>
}
