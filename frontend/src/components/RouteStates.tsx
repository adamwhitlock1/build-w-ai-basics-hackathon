import { Link, useRouter, type ErrorComponentProps } from '@tanstack/react-router'
import { ApiError } from '../api/client'
import { FormError } from './FormError'
import { WelcomeLayout } from './WelcomeLayout'

const linkButton =
  'inline-flex h-11 items-center rounded-2xl border border-primary/30 bg-surface px-4 font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary'

/**
 * Shown when a page couldn't load — most often because the server is down.
 * "Try again" re-runs the page's guards and loaders.
 */
export function RouteError({ error }: ErrorComponentProps) {
  const router = useRouter()
  const message = error instanceof ApiError ? error.message : 'Something went wrong while loading this page.'
  const hint = error instanceof ApiError ? error.hint : 'Try again in a moment.'

  return (
    <WelcomeLayout title="That didn’t load">
      <FormError message={message} hint={hint} />
      <button type="button" onClick={() => void router.invalidate()} className={`mt-5 ${linkButton}`}>
        Try again
      </button>
    </WelcomeLayout>
  )
}

/** Shown while a page is still loading after a moment (the router waits about a second first). */
export function RoutePending() {
  return (
    <div role="status" className="flex min-h-dvh items-center justify-center gap-3 px-4 text-ink-muted">
      <span
        aria-hidden="true"
        className="size-5 shrink-0 rounded-full border-2 border-primary/20 border-t-primary motion-safe:animate-spin-calm"
      />
      Just a moment…
    </div>
  )
}

/** Shown for any address the app doesn't know. */
export function NotFound() {
  return (
    <WelcomeLayout title="Nothing here" intro="We couldn’t find that page.">
      <Link to="/" className={linkButton}>
        Go to your page
      </Link>
    </WelcomeLayout>
  )
}
