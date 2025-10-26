import { z } from "zod"
import { createTRPCRouter, protectedProcedure } from "@/server/api/trpc"
import { accounts } from "@/server/db/schema"
import { eq } from "drizzle-orm"

export const chatRouter = createTRPCRouter({
	send: protectedProcedure
		.input(
			z.object({
				message: z.string(),
				conversationId: z.string(),
				repoUrl: z.string(),
				projectId: z.string(),
			})
		)
		.mutation(async ({ input, ctx }) => {
			const GO_SERVICE_URL = process.env.GO_SERVICE_URL || "http://echo-app:1323"

			// Get GitHub token from user's account
			const userAccount = await ctx.db.query.accounts.findFirst({
				where: eq(accounts.userId, ctx.session.user.id),
			})

			if (!userAccount?.access_token) {
				throw new Error("GitHub account not connected")
			}

			const response = await fetch(`${GO_SERVICE_URL}/chat`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
				body: JSON.stringify({
					message: input.message,
					conversation_id: input.conversationId,
					repo_url: input.repoUrl,
					github_token: userAccount.access_token,
					user_id: ctx.session.user.id,
					project_id: input.projectId,
				}),
			})

			if (!response.ok) {
				const error = await response.json()
				throw new Error(error.error || "Failed to process chat")
			}

			return await response.json()
		}),
})
