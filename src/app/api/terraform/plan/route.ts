import { NextResponse } from "next/server";
import { z } from "zod";
import { createCaller } from "@/server/api/root";
import { createTRPCContext } from "@/server/api/trpc";

const Payload = z.object({
  repoId: z.number(),
  commitHash: z.string().min(1),
  name: z.string().optional(),
  plan: z.unknown(),
});

export async function POST(request: Request) {
  try {
    const json = (await request.json()) as unknown;
    const { repoId, commitHash, plan, name } = Payload.parse(json);

    const caller = createCaller(
      await createTRPCContext({ headers: new Headers(request.headers) }),
    );
    await caller.terraform.storePlan({ repoId, commitHash, plan, name });

    return NextResponse.json({ status: "ok" });
  } catch (err) {
    console.error("/api/terraform/plan error", err);
    if (err instanceof z.ZodError) {
      return NextResponse.json(
        { error: "Invalid payload", details: err.flatten() },
        { status: 400 },
      );
    }
    return NextResponse.json({ error: "Internal error" }, { status: 500 });
  }
}
