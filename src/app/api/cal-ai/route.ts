import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/server/auth"
import { db } from "@/server/db"
import { accounts } from "@/server/db/schema"
import { and, eq } from "drizzle-orm"
import { env } from "@/env"
import type { Session } from "next-auth"

export async function POST(req: NextRequest) {
	try {
		const session = await auth()
		const githubToken = await ensureValidAccessToken(session)

		if (!githubToken) {
			return NextResponse.json(
				{ error: "Unauthorized" },
				{ status: 401 }
			)
		}

		const body = await req.json()
		const { message, conversationId, repoUrl, projectId } = body

		//	const GO_SERVICE_URL = env.GO_SERVICE_URL || "http://echo-app:1323"
		const GO_SERVICE_URL = "http://echo-app:1323"
		//	const GO_SERVICE_URL = "http://0.0.0.0:1323"

		console.log("[API /cal-ai] Forwarding chat message to Go service:", {
			message,
			conversationId,
			repoUrl,
			projectId,
			userId: session?.user.id,
			endpoint: `${GO_SERVICE_URL}/chat`,
		})

		const response = await fetch(`${GO_SERVICE_URL}/chat`, {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify({
				message,
				conversation_id: conversationId,
				repo_url: repoUrl,
				github_token: githubToken,
				user_id: session?.user.id,
				project_id: projectId,
			}),
		})

		console.log("[API /cal-ai] Received response from Go service:", { status: response.status })

		if (!response.ok) {
			let errorMessage = "Failed to process chat"
			try {
				const error = await response.json()
				errorMessage = error.error || errorMessage
			} catch {
				const text = await response.text()
				errorMessage = `Go service error (${response.status}): ${text}`
			}
			return NextResponse.json(
				{ error: errorMessage },
				{ status: response.status }
			)
		}

		const data = await response.json()
		return NextResponse.json(data)
	} catch (error) {
		console.error("[API /cal-ai] Error:", error)
		return NextResponse.json(
			{ error: error instanceof Error ? error.message : "Internal server error" },
			{ status: 500 }
		)
	}
}

const ensureValidAccessToken = async (session: Session | null) => {
	if (!session) {
		throw new Error("Unauthorized")
	}

	const account = await db.query.accounts.findFirst({
		where: and(
			eq(accounts.userId, session.user.id),
			eq(accounts.provider, "github")
		)
	})

	if (!account) {
		throw new Error("Unauthorized")
	}

	const nowSeconds = Math.floor(Date.now() / 1000)
	const isExpired =
		typeof account.expires_at === "number" && account.expires_at !== null
			? nowSeconds >= account.expires_at - 60 // safety window
			: false

	let accessTokenToUse = account.access_token ?? undefined

	if (isExpired) {
		if (!account.refresh_token) {
			throw new Error("Missing refresh token")
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

			await db
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
			throw new Error("GitHub token refresh error")
		}
	}

	return accessTokenToUse
}
