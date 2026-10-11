import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthForm } from '../account/AuthForm'
import { meQueryOptions, useLogin } from '../api/hooks'

export const Route = createFileRoute('/login')({
  beforeLoad: async ({ context }) => {
    // Already logged in: your page is where you want to be (and "/" sends
    // you on to the create-family intro if that isn't done yet).
    if (await context.queryClient.query(meQueryOptions)) throw redirect({ to: '/' })
  },
  component: LoginPage,
})

function LoginPage() {
  const login = useLogin()
  return <AuthForm mode="login" auth={login} />
}
