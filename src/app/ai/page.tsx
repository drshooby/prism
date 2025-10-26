"use client"

import { useChat } from "@ai-sdk/react"
import { useRef, useState } from "react"
import { useSearchParams } from "next/navigation"
import { generateId } from "ai"

export default function Chat() {
  const [input, setInput] = useState("")
  const searchParams = useSearchParams()
  const conversationId = searchParams.get("conversationId") ?? undefined
  const stableGeneratedIdRef = useRef<string>(generateId())
  const { messages, sendMessage, status } = useChat({
    id:
      (conversationId ?? stableGeneratedIdRef.current) +
      "|" +
      `https://github.com/BenKamin03/terraform-test`
  })

  const makeDataUrl = (content: string) =>
    `data:text/plain;charset=utf-8,${encodeURIComponent(content)}`

  // Extract the latest data-plan-json chunk from assistant messages
  const planData = (() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i] as unknown as {
        role?: string
        parts?: Array<{ type?: string; data?: unknown }>
      }
      if (m?.role !== "assistant") continue
      const parts = m.parts ?? []
      for (let j = parts.length - 1; j >= 0; j--) {
        const p = parts[j]
        if (p?.type === "data-plan-json") return p.data
      }
    }
    return undefined
  })()

  const planJsonString = planData
    ? JSON.stringify(
        (planData as { plan?: unknown })?.plan ?? planData,
        null,
        2
      )
    : undefined

  return (
    <div className="w-full max-w-5xl mx-auto py-10 px-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <section className="border border-zinc-200 dark:border-zinc-800 rounded-lg overflow-hidden">
          <header className="px-4 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900/50">
            <h2 className="text-sm font-semibold text-zinc-800 dark:text-zinc-200">
              AI Process
            </h2>
          </header>
          <pre>{JSON.stringify(messages, null, 2)}</pre>
        </section>

        <section className="border border-zinc-200 dark:border-zinc-800 rounded-lg overflow-hidden">
          <header className="px-4 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900/50">
            <h2 className="text-sm font-semibold text-zinc-800 dark:text-zinc-200">
              Terraform Plan (data-plan-json)
            </h2>
          </header>
          {planJsonString ? (
            <>
              <pre>{planJsonString}</pre>
              <div className="px-4 py-3 border-t border-zinc-200 dark:border-zinc-800 bg-zinc-50/50 dark:bg-zinc-900/30">
                <a
                  href={makeDataUrl(planJsonString)}
                  download="terraform-plan.json"
                  className="inline-flex items-center rounded-md px-3 py-1.5 text-sm font-medium text-zinc-700 dark:text-zinc-200 bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-800 hover:bg-zinc-50 dark:hover:bg-zinc-900/70"
                >
                  Download JSON
                </a>
              </div>
            </>
          ) : (
            <div className="px-4 py-3 text-sm text-zinc-500 dark:text-zinc-400">
              No plan received yet.
            </div>
          )}
        </section>
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault()
          void sendMessage({ text: input })
          setInput("")
        }}
        className="mt-6"
      >
        <input
          className="w-full p-3 border border-zinc-300 dark:border-zinc-800 rounded-md bg-white dark:bg-zinc-900 shadow-sm"
          value={input}
          placeholder="Ask for Terraform changes..."
          onChange={(e) => setInput(e.currentTarget.value)}
          disabled={status !== "ready"}
        />
      </form>
    </div>
  )
}
