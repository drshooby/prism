import z from "zod"
import { createTRPCRouter } from "../trpc"
import { githubProcedure } from "./github"
import { and, eq } from "drizzle-orm"
import { d } from "node_modules/drizzle-kit/index-BAUrj6Ib.mjs"
import { messages } from "@/server/db/schema"
import { env } from "@/env"
import { TRPCError } from "@trpc/server"

export const messagesRouter = createTRPCRouter({
  deleteMessage: githubProcedure
    .input(
      z.object({
        messageId: z.string(),
        conversationId: z.number()
      })
    )
    .mutation(async ({ ctx: { accessToken, db }, input }) => {
      const commitHash = await db.query.messages.findFirst({
        where: and(
          eq(messages.id, input.messageId),
          eq(messages.conversationId, input.conversationId)
        ),
        columns: {
          commitHash: true
        }
      })
      if (!commitHash) {
        throw new Error("Message not found")
      }

      await db
        .delete(messages)
        .where(
          and(
            eq(messages.commitHash, commitHash.commitHash),
            eq(messages.conversationId, input.conversationId)
          )
        )

      const res = await fetch(
        `${env.GO_SERVICE_URL}/conversations/${input.conversationId}/messages/${commitHash.commitHash}`,
        {
          method: "DELETE",
          headers: {
            Authorization: `Bearer ${accessToken}`
          }
        }
      )

      if (!res.ok) {
        throw new TRPCError({
          code: "INTERNAL_SERVER_ERROR",
          message: "Failed to delete message"
        })
      }

      return {
        success: true
      }
    })
})
