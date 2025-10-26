import { accounts } from "@/server/db/schema"
import { createTRPCRouter, protectedProcedure } from "../trpc"
import { auth } from "@/server/auth"
import { TRPCError } from "@trpc/server"
import { and, eq } from "drizzle-orm"
import { Octokit } from "octokit"
import { env } from "@/env"

export const githubProcedure = protectedProcedure.use(async ({ ctx, next }) => {
  const session = await auth()
  if (!session) {
    throw new TRPCError({ code: "UNAUTHORIZED" })
  }

  const account = await ctx.db.query.accounts.findFirst({
    where: and(
      eq(accounts.userId, session.user.id),
      eq(accounts.provider, "github")
    )
  })

  if (!account) {
    throw new TRPCError({ code: "UNAUTHORIZED" })
  }

  const nowSeconds = Math.floor(Date.now() / 1000)
  const isExpired =
    typeof account.expires_at === "number" && account.expires_at !== null
      ? nowSeconds >= account.expires_at - 60 // safety window
      : false

  let accessTokenToUse = account.access_token ?? undefined

  if (isExpired) {
    if (!account.refresh_token) {
      throw new TRPCError({
        code: "UNAUTHORIZED",
        message: "Missing refresh token"
      })
    }

    try {
      type GitHubRefreshResponse = {
        access_token?: string
        token_type?: string
        scope?: string
        expires_in?: number
        refresh_token?: string
        refresh_token_expires_in?: number
      }

      const tokenRes = await fetch(
        "https://github.com/login/oauth/access_token",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
            Accept: "application/json"
          },
          body: new URLSearchParams({
            client_id: env.GITHUB_CLIENT_ID,
            client_secret: env.GITHUB_CLIENT_SECRET,
            grant_type: "refresh_token",
            refresh_token: account.refresh_token
          })
        }
      )

      if (!tokenRes.ok) {
        throw new Error(`GitHub refresh HTTP ${tokenRes.status}`)
      }

      const data = (await tokenRes.json()) as GitHubRefreshResponse
      const {
        access_token: newAccessToken,
        token_type,
        scope,
        expires_in,
        refresh_token: newRefreshToken,
        refresh_token_expires_in
      } = data ?? {}

      if (!newAccessToken) {
        throw new Error("No access_token in refresh response")
      }

      const updated: {
        access_token?: string
        expires_at?: number
        refresh_token?: string
        refresh_token_expires_in?: number
        token_type?: string
        scope?: string
      } = {
        access_token: newAccessToken
      }

      if (typeof expires_in === "number") {
        updated.expires_at = nowSeconds + expires_in
      }
      if (typeof newRefreshToken === "string" && newRefreshToken.length > 0) {
        updated.refresh_token = newRefreshToken
      }
      if (typeof refresh_token_expires_in === "number") {
        updated.refresh_token_expires_in = refresh_token_expires_in
      }
      if (typeof token_type === "string") {
        updated.token_type = token_type
      }
      if (typeof scope === "string") {
        updated.scope = scope
      }

      await ctx.db
        .update(accounts)
        .set(updated)
        .where(
          and(
            eq(accounts.userId, session.user.id),
            eq(accounts.provider, "github")
          )
        )

      accessTokenToUse = newAccessToken
      account.access_token = newAccessToken
    } catch (err) {
      console.error("GitHub token refresh error:", err)
      throw new TRPCError({ code: "UNAUTHORIZED" })
    }
  }

  return next({
    ctx: {
      session: session,
      accessToken: accessTokenToUse
    }
  })
})

export const githubRouter = createTRPCRouter({
  getUserRepos: githubProcedure.query(async ({ ctx: { accessToken } }) => {
    const octokit = new Octokit({ auth: accessToken })

    const repos = await octokit.rest.repos.listForAuthenticatedUser({
      per_page: 100,
      sort: "updated"
    })

    return repos.data
  })
})
