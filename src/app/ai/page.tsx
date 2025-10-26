"use client";

import { useChat } from "@ai-sdk/react";
import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { generateId } from "ai";

export default function Chat() {
  const [input, setInput] = useState("");
  const searchParams = useSearchParams();
  const conversationId = searchParams.get("conversationId") ?? undefined;
  const { messages, sendMessage, status, error } = useChat({
    id: conversationId ?? generateId(),
  });

  const makeDataUrl = (content: string) =>
    `data:text/plain;charset=utf-8,${encodeURIComponent(content)}`;

  return (
    <div className="w-full max-w-5xl mx-auto py-10 px-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <section className="border border-zinc-200 dark:border-zinc-800 rounded-lg overflow-hidden">
          <header className="px-4 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900/50">
            <h2 className="text-sm font-semibold text-zinc-800 dark:text-zinc-200">
              AI Process
            </h2>
          </header>
          <div className="p-4 h-[60vh] overflow-auto space-y-3">
            {messages.map((message) => (
              <div key={message.id} className="whitespace-pre-wrap text-sm">
                <div className="mb-1 font-medium text-zinc-600 dark:text-zinc-400">
                  {message.role === "user" ? "User" : "AI"}
                </div>
                {message.parts.map((part, i) => {
                  if ((part as { type?: string }).type === "text") {
                    return (
                      <div
                        key={`${message.id}-${i}`}
                        className="text-zinc-900 dark:text-zinc-100"
                      >
                        {(part as { text?: string }).text}
                      </div>
                    );
                  }
                  if (part.type === "reasoning") {
                    return (
                      <div key={i} className="text-zinc-500 dark:text-zinc-400">
                        {part.text}
                      </div>
                    );
                  }
                  const typeString = String((part as { type: string }).type);
                  if (typeString === "data-terraform-files") {
                    const data = (
                      part as unknown as {
                        data?: {
                          files?: { path: string; content: string }[];
                          appliedDiffs?: { path: string; patch: string }[];
                        };
                      }
                    ).data;
                    const files = data?.files ?? [];
                    return (
                      <div key={`${message.id}-${i}`} className="text-xs">
                        <div className="font-medium text-emerald-700 dark:text-emerald-300">
                          Terraform files ready ({files.length})
                        </div>
                        <div className="mt-2 space-y-2">
                          {files.map((f) => (
                            <div
                              key={`${message.id}-${i}-${f.path}`}
                              className="flex items-center justify-between gap-3 rounded border border-zinc-200 dark:border-zinc-800 p-2"
                            >
                              <div className="truncate text-zinc-800 dark:text-zinc-200">
                                {f.path}
                              </div>
                              <div className="shrink-0 flex items-center gap-2">
                                <a
                                  href={makeDataUrl(f.content)}
                                  download={f.path}
                                  className="text-xs px-2 py-1 rounded bg-emerald-600 text-white hover:bg-emerald-700"
                                >
                                  Download
                                </a>
                                <button
                                  type="button"
                                  onClick={() => {
                                    void navigator.clipboard.writeText(
                                      f.content,
                                    );
                                  }}
                                  className="text-xs px-2 py-1 rounded border border-zinc-200 dark:border-zinc-700"
                                >
                                  Copy
                                </button>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    );
                  }
                  const isStaticTool = typeString.startsWith("tool-");
                  const isDynamicTool = typeString === "dynamic-tool";
                  if (isStaticTool || isDynamicTool) {
                    const label = isDynamicTool
                      ? `Tool: ${(part as { toolName?: string }).toolName ?? "dynamic"}`
                      : `Tool: ${typeString.slice(5)}`;
                    const state = (part as { state?: string }).state ?? "";
                    const input = (part as { input?: unknown }).input;
                    const output = (part as { output?: unknown }).output;
                    const errorText = (part as { errorText?: string })
                      .errorText;
                    return (
                      <div key={`${message.id}-${i}`} className="text-xs">
                        <div className="text-amber-700 dark:text-amber-300">
                          {label} ({state})
                        </div>
                        {input !== undefined && (
                          <pre className="mt-1 rounded bg-zinc-100 dark:bg-zinc-900 p-2 overflow-auto text-[10px]">
                            {typeof input === "string"
                              ? input
                              : JSON.stringify(input, null, 2)}
                          </pre>
                        )}
                        {output !== undefined && (
                          <pre className="mt-1 rounded bg-zinc-100 dark:bg-zinc-900 p-2 overflow-auto text-[10px]">
                            {typeof output === "string"
                              ? output
                              : JSON.stringify(output, null, 2)}
                          </pre>
                        )}
                        {errorText && (
                          <div className="text-red-600 dark:text-red-400 mt-1">
                            {errorText}
                          </div>
                        )}
                      </div>
                    );
                  }
                  return null;
                })}
              </div>
            ))}
            {error && (
              <div className="text-xs text-red-600 dark:text-red-400">
                {String(error.message ?? error)}
              </div>
            )}
          </div>
        </section>
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void sendMessage({ text: input });
          setInput("");
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
  );
}
