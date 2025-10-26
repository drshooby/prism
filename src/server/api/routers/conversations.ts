import { createTRPCRouter, protectedProcedure } from "../trpc"
import { githubProcedure } from "./github"
import { eq } from "drizzle-orm"
import { conversations, importedRepositories } from "@/server/db/schema"
import { z } from "zod"
import { env } from "@/env"
import { TRPCError } from "@trpc/server"

export const conversationsRouter = createTRPCRouter({
  getConversations: protectedProcedure.query(
    async ({ ctx: { db, session } }) => {
      const conversationsData = await db.query.conversations.findMany({
        where: eq(conversations.userId, session.user.id)
      })
      return conversationsData
    }
  ),

  createConversation: githubProcedure
    .input(
      z.object({
        projectId: z.number()
      })
    )
    .mutation(async ({ ctx: { db, session }, input }) => {
      const conversation = await db.insert(conversations).values({
        userId: session.user.id,
        projectId: input.projectId
      })
      return conversation
    }),

  createPR: githubProcedure
    .input(
      z.object({
        conversationId: z.number(),
        mergeBranch: z.string(),
        prTitle: z.string(),
        prBody: z.string()
      })
    )
    .mutation(async ({ ctx: { db, accessToken }, input }) => {
      const [conversation] = await db
        .select()
        .from(conversations)
        .where(eq(conversations.id, input.conversationId))
        .innerJoin(
          importedRepositories,
          eq(conversations.projectId, importedRepositories.id)
        )
        .limit(1)

      const prUrlResponse = await fetch(
        `${env.GO_SERVICE_URL}/conversations/${input.conversationId}/pr`,
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${accessToken}`
          },
          body: JSON.stringify({
            repo_url: `https://github.com/${conversation?.imported_repository?.owner}/${conversation?.imported_repository?.name}.git`,
            github_token: accessToken,
            base_branch: input.mergeBranch,
            pr_title: input.prTitle,
            pr_body: input.prBody
          })
        }
      )

      if (!prUrlResponse.ok) {
        throw new TRPCError({
          code: "INTERNAL_SERVER_ERROR",
          message: `Failed to create PR: ${prUrlResponse.status}`
        })
      }

      const prUrl = (await prUrlResponse.json()) as {
        pr_number: number
        pr_url: string
        branch: string
        base: string
      }

      return {
        prNumber: prUrl.pr_number,
        prUrl: prUrl.pr_url,
        branch: prUrl.branch,
        base: prUrl.base
      }
    })
})
