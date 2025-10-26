import { openrouter } from "@openrouter/ai-sdk-provider"
import {
  streamText,
  convertToModelMessages,
  createUIMessageStreamResponse,
  generateText
} from "ai"
import type { UIMessage, UIMessageChunk } from "ai"
import { getLocalTerraformFileTools } from "@/server/tools/terraform-file-tools"
import { env } from "@/env"
import { z } from "zod"
import { auth } from "@/server/auth"
import { db } from "@/server/db"
import { accounts } from "@/server/db/schema"
import { and, eq } from "drizzle-orm"
import type { Session } from "next-auth"
import type { Plan } from "@/types/terraform"
import * as os from "node:os"
import * as path from "node:path"

export const runtime = "nodejs"
export const dynamic = "force-dynamic"
export const fetchCache = "force-no-store"
export const maxDuration = 60

const tfFileSchema = z.object({
  files: z.array(
    z.object({
      path: z.string(),
      content: z.string()
    })
  ),
  count: z.number()
})

export async function POST(req: Request) {
  const url = new URL(req.url)
  const queryConversationId =
    url.searchParams.get("conversationId") ??
    url.searchParams.get("id") ??
    undefined
  const { messages } = (await req.json()) as {
    messages: UIMessage[]
    conversationId?: string
  }
  const conversationId = queryConversationId?.split("|")[0]
  const repoUrl = queryConversationId?.split("|")[1]

  const session = await auth()
  const githubToken = await ensureValidAccessToken(session)

  if (!githubToken) {
    return new Response("Unauthorized", { status: 401 })
  }

  const appliedDiffs: { path: string; patch: string }[] = []
  let initialFiles: { path: string; content: string }[] = []

  const safeConversationId = (conversationId ?? "").replace(
    /[^a-zA-Z0-9._-]/g,
    "_"
  )
  const baseDir = safeConversationId
    ? path.join(os.tmpdir(), "prism-tf", safeConversationId)
    : undefined

  const tools = getLocalTerraformFileTools({
    onDiffApplied: (entries) => {
      for (const e of entries) appliedDiffs.push(e)
    },
    baseDir
  })

  try {
    if (conversationId && typeof conversationId === "string") {
      const res = await fetch(
        `${env.GO_SERVICE_URL}/conversations/${encodeURIComponent(conversationId)}?repo_url=${encodeURIComponent(repoUrl ?? "")}`,
        { method: "GET", headers: { Authorization: `Bearer ${githubToken}` } }
      )
      if (res.ok) {
        const payload = tfFileSchema.parse(await res.json())
        if (payload.count > 0) {
          initialFiles = payload.files

          const resetTool = (
            tools as unknown as {
              files_reset_store?: {
                execute: (i: Record<string, never>) => Promise<unknown>
              }
            }
          ).files_reset_store
          const setFilesTool = (
            tools as unknown as {
              files_set_files?: {
                execute: (i: {
                  files: { path: string; content: string }[]
                }) => Promise<unknown>
              }
            }
          ).files_set_files

          if (resetTool && setFilesTool) {
            await resetTool.execute({} as Record<string, never>)
            await setFilesTool.execute({ files: initialFiles })
          }
        }
      }
    }
  } catch {
    // ignore errors
  }

  const BASE_SYSTEM_PROMPT =
    "You are a Terraform engineer. Always prefer using the available Terraform MCP tools for planning, formatting, validating, and inspecting state over guessing. Before finalizing answers, ensure code compiles and passes lint via these tools.\n\nUse an in-memory file store via tools: `files_reset_store`, `files_load_stub`, `files_set_files`, `files_get_files`, `files_apply_diff`, and `terraform_lint_only`. Initialize the store with the user's provided files (or call `files_load_stub` to start) before editing.\n\nImportant: Always apply changes by calling tools. Do not just print code. Produce unified git-style diffs (---/+++ and @@) and immediately call `files_apply_diff` with those diffs. Keep iterating until lint passes (fmt=0, validate=0, tflint=0).\n\nScope: `.tf`, `.tf.json`, `.tfvars`, `.tftmpl`. Keep modules/provider versions pinned and use stable resources. Make sure to check the contents of the terraform at the start of the conversation in order to understand the context."

  function extractTextTranscript(uiMessages: UIMessage[]): string {
    const lines: string[] = []
    for (const m of uiMessages) {
      const role = m.role ?? "assistant"
      const textParts = (m.parts ?? [])
        .map((p: unknown) => {
          const typed = p as { type?: string; text?: string }
          return typed?.type === "text" && typeof typed.text === "string"
            ? typed.text
            : ""
        })
        .filter(Boolean)
      if (textParts.length > 0) {
        lines.push(`${role.toUpperCase()}: ${textParts.join("\n")}`)
      }
    }
    return lines.join("\n\n")
  }

  const RECENT_TURNS_TO_KEEP = 6
  const earlyHistory = messages.slice(
    0,
    Math.max(0, messages.length - RECENT_TURNS_TO_KEEP)
  )
  const earlyTranscript = extractTextTranscript(earlyHistory)

  let systemPrompt = BASE_SYSTEM_PROMPT
  let messagesForModel: UIMessage[] = messages

  if (earlyTranscript.length > 4000) {
    try {
      const { text: summary } = await generateText({
        model: openrouter("deepseek/deepseek-v3.2-exp"),
        system:
          "Summarize the prior Terraform-focused conversation for the assistant. Capture:\n- current Terraform files and state if mentioned\n- key decisions, constraints, providers/versions\n- pending TODOs or next steps requested by the user.\nKeep it concise (<= 250 words). Do not include diffs or large code blocks.",
        prompt: earlyTranscript,
        maxOutputTokens: 350
      })
      systemPrompt = `${BASE_SYSTEM_PROMPT}\n\nConversation summary (for context):\n${summary}`
      messagesForModel = messages.slice(-RECENT_TURNS_TO_KEEP)
    } catch {
      messagesForModel = messages.slice(-RECENT_TURNS_TO_KEEP)
      systemPrompt = BASE_SYSTEM_PROMPT
    }
  }

  if (initialFiles.length > 0) {
    const MAX_TOTAL_CHARS = 3500
    const MAX_PER_FILE = 500
    let used = 0
    const parts: string[] = []
    for (const f of initialFiles) {
      if (used >= MAX_TOTAL_CHARS) break
      const snippet = f.content.slice(0, MAX_PER_FILE)
      const header = `FILE: ${f.path}`
      const block = `${header}\n${snippet}`
      parts.push(block)
      used += block.length
    }
    const filesContext = `\n\nInitial Terraform files (truncated for context):\n${parts.join("\n\n")}`
    systemPrompt = `${systemPrompt}${filesContext}`
  }

  const result = streamText({
    model: openrouter("deepseek/deepseek-v3.2-exp"),
    toolChoice: "required",
    system: systemPrompt,
    messages: convertToModelMessages(messagesForModel),
    tools,
    stopWhen: () => false
  })

  const baseStream = result.toUIMessageStream()
  const finalStream = baseStream.pipeThrough(
    new TransformStream<UIMessageChunk, UIMessageChunk>({
      transform(chunk, controller) {
        controller.enqueue(chunk)
      },
      async flush(controller) {
        console.log("flushing")
        try {
          type FilesGetFilesTool = {
            execute: (input: Record<string, never>) => Promise<{
              status: string
              files: { path: string; content: string }[]
            }>
          }
          const filesTool = (
            tools as unknown as {
              files_get_files?: FilesGetFilesTool
            }
          ).files_get_files
          const filesResult = filesTool
            ? await filesTool.execute({} as Record<string, never>)
            : {
                status: "ok",
                files: [] as { path: string; content: string }[]
              }

          if (conversationId) {
            const changedPaths = new Set<string>(
              appliedDiffs.map((d) => d.path)
            )
            const changedFiles = filesResult.files.filter((f) =>
              changedPaths.has(f.path)
            )
            if (changedFiles.length > 0) {
              const form = new FormData()
              if (githubToken) {
                form.append("github_token", githubToken)
              }
              if (repoUrl) form.append("repo_url", repoUrl)
              for (const f of changedFiles) {
                form.append(
                  "files",
                  JSON.stringify({ path: f.path, content: f.content })
                )
              }
              try {
                const planRes = await fetch(
                  `${env.GO_SERVICE_URL}/conversation/${encodeURIComponent(conversationId)}`,
                  { method: "POST", body: form }
                )

                const plan = (await planRes.json()) as Plan

                console.log("plan", JSON.stringify(plan, null, 2))

                controller.enqueue({
                  type: "data-plan-json",
                  id: "final",
                  data: {
                    plan: plan
                  }
                } as unknown as UIMessageChunk)
              } catch (error) {
                // ignore errors
                console.error(
                  "error getting plan",
                  JSON.stringify(error, null, 2)
                )
              }
            }
          }
        } catch {
          // ignore errors
        }
      }
    })
  )

  return createUIMessageStreamResponse({ stream: finalStream })
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
