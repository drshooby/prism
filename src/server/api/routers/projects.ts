import { createTRPCRouter, protectedProcedure, publicProcedure } from "../trpc"
import { z } from "zod"
import { githubProcedure } from "./github"
import { Octokit } from "octokit"
import { TRPCError } from "@trpc/server"
import { importedRepositories } from "@/server/db/schema"

export const projectsRouter = createTRPCRouter({
  createProject: githubProcedure
    .input(
      z.object({
        repoName: z.string(),
        owner: z.string()
      })
    )
    .mutation(async ({ ctx, input }) => {
      const octokit = new Octokit({ auth: ctx.accessToken })

      const repo = await octokit.rest.repos.get({
        owner: input.owner,
        repo: input.repoName
      })

      if (!repo.data) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "Repository not found"
        })
      }

      const [project] = await ctx.db
        .insert(importedRepositories)
        .values({
          owner: input.owner,
          name: input.repoName,
          repoId: repo.data.id,
          userId: ctx.session.user.id
        })
        .returning({ id: importedRepositories.id })

      /**
       *  TODO - get tf plan json
       *
       * fetch http://echo-app:1323/terraform/plan
       *
       * payload: {
       *  repo: {
       *    owner: input.owner,
       *    name: input.repoName,
       *    repoId: repo.data.id,
       *    commitHash: repo.data.commit.sha,
       *  },
       *  accessToken: ctx.accessToken,
       *  user: {
       *    id: ctx.session.user.id,
       *    email: ctx.session.user.email,
       *    name: ctx.session.user.name,
       *  },
       * }
       * */

      return project
    })
})
