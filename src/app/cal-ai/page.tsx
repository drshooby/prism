"use client"

import { useState } from "react"
import { api } from "@/trpc/react"

export default function Chat() {
  const [prompt, setPrompt] = useState("")
  const [response, setResponse] = useState<string>("")

  const chatMutation = api.chat.send.useMutation({
    onSuccess: (data) => {
      setResponse(JSON.stringify(data, null, 2))
    },
    onError: (error) => {
      setResponse(`Error: ${error.message}`)
    },
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    chatMutation.mutate({
      message: prompt,
      conversationId: "test-conversation",
      repoUrl: "https://github.com/your-org/your-repo",
      projectId: "your-project-id",
    })
  }

  return (
    <div style={{ padding: "20px" }}>
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="Enter your prompt..."
          style={{ width: "100%", padding: "10px", marginBottom: "10px" }}
        />
        <button type="submit" disabled={chatMutation.isPending}>
          {chatMutation.isPending ? "Loading..." : "Submit"}
        </button>
      </form>
      {response && (
        <div style={{ marginTop: "20px", padding: "10px", border: "1px solid #ccc" }}>
          <pre>{response}</pre>
        </div>
      )}
    </div>
  )
}
