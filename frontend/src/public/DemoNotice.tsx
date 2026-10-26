import { useLayoutEffect, useRef } from 'react'
import { demoPaused } from './demoState'
import { usePublicConfig } from './config'

export function DemoNotice() {
  const { data } = usePublicConfig()
  const noticeRef = useRef<HTMLElement>(null)

  useLayoutEffect(() => {
    const notice = noticeRef.current
    if (!notice) return
    const root = document.documentElement
    const measure = () => root.style.setProperty('--demo-notice-height', `${notice.getBoundingClientRect().height}px`)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(notice)
    return () => {
      observer.disconnect()
      root.style.removeProperty('--demo-notice-height')
    }
  }, [data?.demo])

  if (!data?.demo) return null
  const q = data.quota
  const exhausted = demoPaused(q)
  return <aside ref={noticeRef} aria-label="Judge demo" className="lg:sticky lg:top-0 lg:z-30 border-b border-line bg-primary-soft px-4 py-3 text-sm text-ink">
    <div className="mx-auto max-w-7xl">
      <strong>Rivera family · shared judge demo.</strong> This is fictional, resettable data. Other visitors can see your changes and chats. Please don’t enter personal information.
      <p className="mt-1">Initial cards, calendar events, conversations titled “Sample” and the sample briefing are prepared fictional examples. New AI responses are live. Judges: find the parent and kid logins in the <strong>“Tell us more about the value your project provides for you?”</strong> section of the hackathon submission.</p>
      {q && <p className="mt-1" role="status">{exhausted ? 'Demo allowance reached. Some live actions are paused; existing content is still available.' : `${q.modelsRemaining} AI calls and ${q.searchesRemaining} searches left today.`} Allowance resets {new Date(q.reset).toLocaleString(undefined, { timeZone: 'America/Denver', hour: 'numeric', minute: '2-digit', month: 'short', day: 'numeric' })} Denver time.</p>}
    </div>
  </aside>
}
