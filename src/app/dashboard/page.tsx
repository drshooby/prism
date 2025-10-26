// Components
import { SignInOutWrapper } from "@/components/SignInOutWrapper"

// Functions
import { api } from "@/trpc/server"
import { protectRouteAndGetSessionUser } from "@/server/auth/config"

export default async function Dashboard() {
  const user = await protectRouteAndGetSessionUser()
  const availableRepos = await api.github.getUserRepos()

  return (
    <main>
      <p>{<span>Logged in as {user.name}</span>}</p>
      <SignInOutWrapper />
      <ul>
        {availableRepos?.map((repo) => (
          <li key={repo.id}>{repo.name}</li>
        ))}
      </ul>
    </main>
  )
}
