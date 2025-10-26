// Components
import { SignInOutWrapper } from "@/components/SignInOutWrapper"

// Functions
import { auth } from "@/server/auth"
import { redirect } from "next/navigation"
import { isAuthorized } from "@/server/auth/config"

export default async function Home() {
  const session = await auth()
  const authorized = isAuthorized(session)
  if (authorized) redirect("/dashboard")

  return <SignInOutWrapper />
}
