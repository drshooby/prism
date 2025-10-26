import { createTRPCRouter, protectedProcedure, publicProcedure } from "../trpc"
import { z } from "zod"
import { githubProcedure } from "./github"
import { Octokit } from "octokit"
import { TRPCError } from "@trpc/server"
import { importedRepositories } from "@/server/db/schema"
import { env } from "@/env"
import type { Plan } from "@/types/terraform"

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

      if (!project) {
        throw new TRPCError({
          code: "INTERNAL_SERVER_ERROR",
          message: "Failed to create project"
        })
      }

      const commitHash = await octokit.rest.repos.getCommit({
        owner: input.owner,
        repo: input.repoName,
        ref: "main"
      })

      if (!commitHash.data) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "Commit hash not found"
        })
      }

      try {
        const tfPlanResponse = await fetch(`${env.GO_SERVICE_URL}/plan`, {
          method: "POST",
          headers: {
            Authorization: `Bearer ${ctx.accessToken}`
          },
          body: JSON.stringify({
            repo: {
              owner: input.owner,
              name: input.repoName,
              repoId: repo.data.id,
              commitHash: commitHash.data.sha
            },
            accessToken: ctx.accessToken,
            user: {
              id: ctx.session.user.id,
              email: ctx.session.user.email,
              name: ctx.session.user.name
            }
          })
        })

        if (!tfPlanResponse.ok) {
          const code = tfPlanResponse.status
          throw new TRPCError({
            code: "INTERNAL_SERVER_ERROR",
            message: `Failed to get terraform plan: ${code}`
          })
        }

        const plan = (await tfPlanResponse.json()) as Plan

        return { projectId: project.id, plan }
      } catch (error) {
        throw new TRPCError({
          code: "INTERNAL_SERVER_ERROR",
          message: `Failed to get terraform plan (${env.GO_SERVICE_URL}/plan): ${error instanceof Error ? error.message : "Unknown error"}`
        })
      }
    })
})
