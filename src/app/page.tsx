// Components
import Link from "next/link"

// Functions
import { auth } from "@/server/auth"
import { api } from "@/trpc/server"

// Styles
import styles from "./Home.module.css"

export default async function Home() {
  const hello = await api.post.hello({ text: "from tRPC" })
  const data = await api.github.getUserRepos()
  const session = await auth()

  if (session?.user) {
    void api.post.getLatest.prefetch()
  }

  return (
    <main>
      <p>{hello ? hello.greeting : "Loading tRPC query..."}</p>
      <p>{session && <span>Logged in as {session.user?.name}</span>}</p>
      <Link href={session ? "/api/auth/signout" : "/api/auth/signin"}>
        {session ? "Sign out" : "Sign in"}
      </Link>
      <ul>
        {data?.map((repo) => (
          <li key={repo.id}>{repo.name}</li>
        ))}
      </ul>
    </main>
  )
}
