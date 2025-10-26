"use client"

import { Button } from "@/components/Button"
import { signIn } from "next-auth/react"

async function handleLoginWithGitHub() {
  return await signIn("github", { callbackUrl: "/dashboard" })
}

export function SignIn() {
  return <Button onClick={handleLoginWithGitHub}>Sign in with GitHub</Button>
}
