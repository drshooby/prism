"use client"

import { useState } from "react"

export default function Chat() {
  const [prompt, setPrompt] = useState("")
  const [response, setResponse] = useState<string>("")
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setResponse("")

    try {
      const res = await fetch("/api/cal-ai", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          message: prompt,
          conversationId: "test-branch",
          repoUrl: "git@github.com:ccrawford4/oneflow-infra.git",
          projectId: "8b780994-3ac2-4694-a867-54da692fe6f9",
        }),
      })

      if (!res.ok) {
        const error = await res.json()
        throw new Error(error.error || `Error ${res.status}`)
      }

      const data = await res.json()
      setResponse(JSON.stringify(data, null, 2))
    } catch (error) {
      console.error("Error:", error)
      setResponse(`Error: ${error instanceof Error ? error.message : String(error)}`)
    } finally {
      setLoading(false)
    }
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
        <button type="submit" disabled={loading}>
          {loading ? "Loading..." : "Submit"}
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
