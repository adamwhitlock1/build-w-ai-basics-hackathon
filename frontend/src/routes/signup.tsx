import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthForm } from '../account/AuthForm'
import { meQueryOptions, useSignup } from '../api/hooks'

export const Route = createFileRoute('/signup')({
  beforeLoad: async ({ context }) => {
    // Already logged in: no second account needed.
    if (await context.queryClient.query(meQueryOptions)) throw redirect({ to: '/' })
  },
  component: SignupPage,
})

function SignupPage() {
  const signup = useSignup()
  return <AuthForm mode="signup" auth={signup} />
}
