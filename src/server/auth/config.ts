import { auth } from "."
import { db } from "@/server/db"
import {
  accounts,
  sessions,
  users,
  verificationTokens
} from "@/server/db/schema"
import {
  type DefaultSession,
  type NextAuthConfig,
  type Session
} from "next-auth"
import { env } from "@/env"
import GitHub from "next-auth/providers/github"
import { DrizzleAdapter } from "@auth/drizzle-adapter"
import { redirect, unauthorized } from "next/navigation"

/**
 * Module augmentation for `next-auth` types. Allows us to add custom properties to the `session`
 * object and keep type safety.
 *
 * @see https://next-auth.js.org/getting-started/typescript#module-augmentation
 */
declare module "next-auth" {
  interface Session extends DefaultSession {
    user: {
      id: string
      // ...other properties
      // role: UserRole;
    } & DefaultSession["user"]
  }

  // interface User {
  //   // ...other properties
  //   // role: UserRole;
  // }
}

/**
 * Options for NextAuth.js used to configure adapters, providers, callbacks, etc.
 *
 * @see https://next-auth.js.org/configuration/options
 */
export const authConfig = {
  providers: [
    GitHub({
      clientId: env.GITHUB_CLIENT_ID,
      clientSecret: env.GITHUB_CLIENT_SECRET,
      authorization: {
        params: {
          scope: "read:user user:email repo pull_requests read:org"
        }
      }
    })
    /**
     * ...add more providers here.
     *
     * Most other providers require a bit more work than the Discord provider. For example, the
     * GitHub provider requires you to add the `refresh_token_expires_in` field to the Account
     * model. Refer to the NextAuth.js docs for the provider you want to use. Example:
     *
     * @see https://next-auth.js.org/providers/github
     */
  ],
  adapter: DrizzleAdapter(db, {
    usersTable: users,
    accountsTable: accounts,
    sessionsTable: sessions,
    verificationTokensTable: verificationTokens
  }),
  callbacks: {
    session: ({ session, user }) => ({
      ...session,
      user: {
        ...session.user,
        id: user.id
      }
    })
  }
} satisfies NextAuthConfig

/**
 * Via a NextAuth `Session`, confirm a user is authorized.
 * That is, they have a valid `id` and `email`.
 * We can extend this function to expand what "authorized" means as needed.
 *
 * @param session a NextAuth `Session` object, likely returned from `signIn()`
 * @returns `true` if authorized, `false` otherwise
 */
export function isAuthorized(session: Session | null) {
  const userId = session?.user?.id
  const userEmail = session?.user?.email
  if (!userId || !userEmail) return false
  return true
}

/**
 * Attempts to authenticate the current NextAuth `Session`.
 * If not authenticated, redirect to the homepage to prompt for signin.
 * If not authorized, return Next.js' `unauthorized()`.
 * If successful, return the NextAuth `User` object.
 *
 * @returns a NextAuth `User` object if authenticated & authorized; redirect appropriately otherwise
 */
export async function protectRouteAndGetSessionUser() {
  const session = await auth()
  if (!session) redirect("/")
  const authorized = isAuthorized(session)
  if (!authorized) unauthorized()
  return session.user
}
