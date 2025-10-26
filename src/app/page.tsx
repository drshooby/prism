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
      <div>
        <div>
          {data?.map((repo) => (
            <div key={repo.id}>{repo.name}</div>
          ))}
        </div>

        <h1>
          Create <span>T3</span> App
        </h1>
        <div>
          <Link
            href="https://create.t3.gg/en/usage/first-steps"
            target="_blank"
          >
            <h3>HELLOOOOO</h3>
            <p>
              Just the basics - Everything you need to know to set up your
              database and authentication.
            </p>
          </Link>
          <Link href="https://create.t3.gg/en/introduction" target="_blank">
            <h3>Documentation →</h3>
            <p>
              Learn more about Create T3 App, the libraries it uses, and how to
              deploy it.
            </p>
          </Link>
        </div>
        <div>
          <p>{hello ? hello.greeting : "Loading tRPC query..."}</p>

          <div>
            <p>{session && <span>Logged in as {session.user?.name}</span>}</p>
            <Link href={session ? "/api/auth/signout" : "/api/auth/signin"}>
              {session ? "Sign out" : "Sign in"}
            </Link>
          </div>
        </div>
      </div>
    </main>
  )
}
