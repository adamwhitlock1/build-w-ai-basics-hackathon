import { queryOptions } from '@tanstack/react-query'
import { Link, createFileRoute, redirect } from '@tanstack/react-router'
import { AuthForm } from '../account/AuthForm'
import { apiFetch } from '../api/client'
import { meQueryOptions, useSignup } from '../api/hooks'
import { WelcomeLayout } from '../components/WelcomeLayout'

/**
 * Whether the sign-up form is open. In production it closes once a family
 * exists: everyone else joins by invite.
 */
const signupStateQueryOptions = queryOptions({
  queryKey: ['signup'],
  queryFn: ({ signal }) => apiFetch<{ open: boolean }>('/api/signup', { signal }),
  staleTime: 0,
})

export const Route = createFileRoute('/signup')({
  beforeLoad: async ({ context }) => {
    // Already logged in: no second account needed.
    if (await context.queryClient.query(meQueryOptions)) throw redirect({ to: '/' })
  },
  loader: ({ context }) => context.queryClient.fetchQuery(signupStateQueryOptions),
  component: SignupPage,
})

function SignupPage() {
  const { open } = Route.useLoaderData()
  const signup = useSignup()
  if (!open) return <SignupClosed />
  return <AuthForm mode="signup" auth={signup} />
}

/** Sign-up is closed: say how to join instead of showing a form. */
function SignupClosed() {
  return (
    <WelcomeLayout
      title="Joining Kinlight"
      footer={
        <Link
          to="/login"
          className="font-medium text-primary underline-offset-4 hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          I already have an account
        </Link>
      }
    >
      <p className="text-ink">Sign-up is closed. Ask a parent in your family for an invite.</p>
      <p className="mt-2 text-sm text-ink-soft">An invite is a link that opens a page where you choose your password.</p>
    </WelcomeLayout>
  )
}
