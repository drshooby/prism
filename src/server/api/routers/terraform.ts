import { TRPCError } from "@trpc/server";
import { createTRPCRouter, publicProcedure } from "../trpc";
import { z } from "zod";
import { importedRepositories, terraformPlans } from "@/server/db/schema";
import { eq } from "drizzle-orm";

export const terraformRouter = createTRPCRouter({
  storePlan: publicProcedure
    .input(
      z.object({
        repoId: z.number(),
        commitHash: z.string().min(1),
        name: z.string().optional(),
        plan: z.unknown(),
      }),
    )
    .mutation(async ({ ctx, input }) => {
      const { repoId, commitHash, plan, name } = input;

      const existingRepo = await ctx.db.query.importedRepositories.findFirst({
        where: eq(importedRepositories.repoId, repoId),
      });
      if (!existingRepo) {
        throw new TRPCError({
          code: "BAD_REQUEST",
          message: "Repository not imported",
        });
      }

      await ctx.db
        .insert(terraformPlans)
        .values({ repositoryId: existingRepo.id, commitHash, plan, name })
        .onConflictDoUpdate({
          target: [terraformPlans.repositoryId, terraformPlans.commitHash],
          set: { plan, name },
        });

      return { ok: true } as const;
    }),
});
