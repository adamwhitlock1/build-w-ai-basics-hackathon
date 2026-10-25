import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useMe } from '../api/hooks'
import { CrisisAlert } from './CrisisAlert'

/** The frame of the parents' review pages: a way back, the crisis alert, and the page. */
export function ReviewsLayout({ back, children }: { back: { to: string; label: string }; children: ReactNode }) {
  const { data: me } = useMe()
  return (
    <main className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6 sm:py-12">
      <Link to={back.to} className="text-sm font-medium text-primary underline-offset-4 hover:underline">
        ← {back.label}
      </Link>
      <div className="mt-4">
        <CrisisAlert />
      </div>
      {me && !me.user.isAdmin ? (
        <p className="rounded-3xl bg-surface p-6 text-ink-soft shadow-card">Only parents can review kids’ chats.</p>
      ) : (
        children
      )}
    </main>
  )
}
