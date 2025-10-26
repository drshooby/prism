import type { ToolSet } from "ai"
import { z } from "zod"
import * as fs from "node:fs/promises"
import * as path from "node:path"
import * as os from "node:os"
import { spawn } from "node:child_process"

type ExecResult = {
  code: number | null
  stdout: string
  stderr: string
}

async function execCmd(
  cmd: string,
  args: string[],
  cwd: string
): Promise<ExecResult> {
  return await new Promise((resolve) => {
    const child = spawn(cmd, args, { cwd, stdio: ["ignore", "pipe", "pipe"] })
    let stdout = ""
    let stderr = ""
    child.stdout.setEncoding("utf8")
    child.stderr.setEncoding("utf8")
    child.stdout.on("data", (d) => {
      stdout += String(d)
    })
    child.stderr.on("data", (d) => {
      stderr += String(d)
    })
    child.on("close", (code) => resolve({ code, stdout, stderr }))
    child.on("error", (err) =>
      resolve({ code: -1, stdout, stderr: String(err) })
    )
  })
}

function applyUnifiedDiff(original: string, patch: string): string {
  const origLines = original.split("\n")
  const out: string[] = []
  let oIdx = 0
  const lines = patch.split("\n")

  const normalizeForCompare = (s: string): string =>
    s.replace(/\r/g, "").replace(/[ \t]+$/, "") // drop CR and trailing whitespace
  const looseEquals = (a: string | undefined, b: string): boolean => {
    if (a === undefined) return false
    return normalizeForCompare(a) === normalizeForCompare(b)
  }
  const findForward = (
    target: string,
    fromIdx: number,
    windowSize = 200
  ): number => {
    const normTarget = normalizeForCompare(target)
    const end = Math.min(origLines.length, fromIdx + windowSize)
    for (let j = fromIdx; j < end; j++) {
      if (normalizeForCompare(origLines[j]!) === normTarget) return j
    }
    return -1
  }

  let i = 0
  while (i < lines.length) {
    const line = lines[i]!
    if (line.startsWith("@@")) {
      i++
      while (
        i < lines.length &&
        !lines[i]?.startsWith("@@") &&
        !lines[i]?.startsWith("--- ") &&
        !lines[i]?.startsWith("diff ") &&
        !lines[i]?.startsWith("+++ ")
      ) {
        const hl = lines[i]!
        if (hl.startsWith(" ")) {
          const content = hl.slice(1)
          const current = oIdx < origLines.length ? origLines[oIdx] : undefined
          if (looseEquals(current, content)) {
            out.push(current ?? "")
            oIdx++
          } else {
            const found = findForward(content, oIdx)
            if (found !== -1) {
              while (oIdx < found) {
                out.push(origLines[oIdx]!)
                oIdx++
              }
              out.push(origLines[oIdx]!)
              oIdx++
            } else {
              throw new Error(
                "Patch context mismatch while applying diff (loose match)"
              )
            }
          }
        } else if (hl.startsWith("-")) {
          const content = hl.slice(1)
          const current = oIdx < origLines.length ? origLines[oIdx] : undefined
          if (looseEquals(current, content)) {
            // remove this line by advancing source without emitting
            oIdx++
          } else {
            const found = findForward(content, oIdx)
            if (found !== -1) {
              // keep everything up to the removal target, then remove that one
              while (oIdx < found) {
                out.push(origLines[oIdx]!)
                oIdx++
              }
              oIdx++ // skip the target line (removed)
            } else {
              // If we cannot find the line to remove, assume it was already removed
              // and continue without throwing.
            }
          }
        } else if (hl.startsWith("+")) {
          const content = hl.slice(1)
          out.push(content)
        } else if (hl.startsWith("\\ No newline at end of file")) {
          // ignore marker
        } else if (hl.length === 0) {
          // treat as context blank line
          const current = oIdx < origLines.length ? origLines[oIdx] : undefined
          if (current !== undefined) {
            out.push(current)
            oIdx++
          } else {
            out.push("")
          }
        }
        i++
      }
    } else if (
      line.startsWith("--- ") ||
      line.startsWith("+++ ") ||
      line.startsWith("diff ")
    ) {
      i++
    } else {
      i++
    }
  }

  while (oIdx < origLines.length) {
    out.push(origLines[oIdx]!)
    oIdx++
  }
  return out.join("\n")
}

async function writeFilesToTemp(
  files: { path: string; content: string }[]
): Promise<string> {
  const tmp = await fs.mkdtemp(path.join(os.tmpdir(), "tf-diff-"))
  for (const f of files) {
    const abs = path.join(tmp, f.path)
    await fs.mkdir(path.dirname(abs), { recursive: true })
    await fs.writeFile(abs, f.content, "utf8")
  }
  return tmp
}

async function runLintPipeline(tmpDir: string): Promise<{
  fmt: ExecResult
  init: ExecResult | null
  validate: ExecResult | null
  tflint: ExecResult | null
}> {
  const fmt = await execCmd(
    "terraform",
    ["fmt", "-no-color", "-diff", "-check"],
    tmpDir
  )
  let init: ExecResult | null = null
  let validate: ExecResult | null = null
  let tflint: ExecResult | null = null

  // terraform init best-effort
  init = await execCmd(
    "terraform",
    ["init", "-backend=false", "-input=false", "-lock=false", "-no-color"],
    tmpDir
  )
  if (init.code === 0) {
    validate = await execCmd("terraform", ["validate", "-no-color"], tmpDir)
  }

  // tflint (works without plugins for core rules)
  tflint = await execCmd("tflint", ["--no-color", "--format", "json"], tmpDir)

  return { fmt, init, validate, tflint }
}

export function getLocalTerraformFileTools(options?: {
  onDiffApplied?: (entries: { path: string; patch: string }[]) => void
  baseDir?: string
}): ToolSet {
  // FS-backed store (if baseDir provided) with in-memory fallback
  const memoryStore = new Map<string, string>()
  const usingFS =
    typeof options?.baseDir === "string" && options.baseDir.length > 0
  const baseDir = usingFS ? path.resolve(options.baseDir!) : null

  const ensureDir = async (dir: string): Promise<void> => {
    await fs.mkdir(dir, { recursive: true })
  }

  const withinBaseDir = (abs: string): boolean => {
    if (!baseDir) return false
    const normBase = baseDir.endsWith(path.sep) ? baseDir : baseDir + path.sep
    const normAbs = path.resolve(abs) + (abs.endsWith(path.sep) ? path.sep : "")
    return normAbs.startsWith(normBase)
  }

  const safeResolve = (rel: string): string => {
    if (!baseDir) throw new Error("No baseDir configured")
    const abs = path.resolve(baseDir, rel)
    if (!withinBaseDir(abs)) throw new Error(`Path escapes baseDir: ${rel}`)
    return abs
  }

  const resetStoreFS = async (): Promise<void> => {
    if (!baseDir) return
    await fs.rm(baseDir, { recursive: true, force: true })
    await ensureDir(baseDir)
  }

  const writeFilesFS = async (
    files: { path: string; content: string }[]
  ): Promise<void> => {
    if (!baseDir) return
    for (const f of files) {
      const abs = safeResolve(f.path)
      await fs.mkdir(path.dirname(abs), { recursive: true })
      await fs.writeFile(abs, f.content, "utf8")
    }
  }

  const readFilesFS = async (): Promise<
    { path: string; content: string }[]
  > => {
    if (!baseDir) return []
    const out: { path: string; content: string }[] = []
    const walk = async (dir: string) => {
      const entries = await fs.readdir(dir, { withFileTypes: true })
      for (const e of entries) {
        const abs = path.join(dir, e.name)
        if (!withinBaseDir(abs)) continue
        if (e.isDirectory()) {
          await walk(abs)
        } else if (e.isFile()) {
          const content = await fs.readFile(abs, "utf8")
          const rel = path.relative(baseDir, abs)
          out.push({ path: rel, content })
        }
      }
    }
    await ensureDir(baseDir)
    await walk(baseDir)
    return out
  }

  const tools: ToolSet = {
    files_load_stub: {
      description:
        "Initialize the in-memory store with a minimal valid Terraform workspace (single main.tf).",
      inputSchema: z.object({}),
      execute: async () => {
        const stub = [
          "terraform {",
          '  required_version = ">= 1.5.0"',
          "}",
          "",
          "locals {",
          '  project = "stub"',
          "}",
          ""
        ].join("\n")

        if (usingFS) {
          await ensureDir(baseDir!)
          const abs = safeResolve("main.tf")
          await fs.mkdir(path.dirname(abs), { recursive: true })
          await fs.writeFile(abs, stub, "utf8")
          return { status: "ok", files: [{ path: "main.tf", content: stub }] }
        }

        memoryStore.set("main.tf", stub)
        return { status: "ok", files: [{ path: "main.tf", content: stub }] }
      }
    },
    files_reset_store: {
      description: "Reset the in-memory Terraform file store to empty.",
      inputSchema: z.object({}),
      execute: async () => {
        if (usingFS) {
          await resetStoreFS()
        } else {
          memoryStore.clear()
        }
        return { status: "ok" }
      }
    },
    files_set_files: {
      description: "Set/replace files in the in-memory store.",
      inputSchema: z.object({
        files: z
          .array(z.object({ path: z.string(), content: z.string() }))
          .min(1)
      }),
      execute: async ({
        files
      }: {
        files: { path: string; content: string }[]
      }) => {
        if (usingFS) {
          await ensureDir(baseDir!)
          await writeFilesFS(files)
          return { status: "ok", count: files.length }
        } else {
          for (const f of files) memoryStore.set(f.path, f.content)
          return { status: "ok", count: files.length }
        }
      }
    },
    files_get_files: {
      description: "Get all files from the in-memory store.",
      inputSchema: z.object({}),
      execute: async () => {
        if (usingFS) {
          const files = await readFilesFS()
          return { status: "ok", files }
        } else {
          const files = Array.from(memoryStore.entries()).map(
            ([path, content]) => ({ path, content })
          )
          return { status: "ok", files }
        }
      }
    },
    terraform_lint_only: {
      description:
        "Run terraform fmt (check), init, validate, and tflint against current files without modifying them.",
      inputSchema: z.object({}),
      execute: async () => {
        if (usingFS) {
          await ensureDir(baseDir!)
          const { fmt, init, validate, tflint } = await runLintPipeline(
            baseDir!
          )
          const fmtIndicatesChanges =
            (fmt.code !== 0 && fmt.code !== null) ||
            /\+{3}|-{3}|@@/.test(fmt.stdout)
          const validateFailed =
            validate && validate.code !== null && validate.code !== 0
          const tflintFailed =
            tflint && tflint.code !== null && tflint.code !== 0
          const status =
            fmtIndicatesChanges || validateFailed || tflintFailed
              ? "lint_failed"
              : "ok"
          const files = await readFilesFS()
          return {
            status,
            files,
            fmt: { code: fmt.code, stdout: fmt.stdout, stderr: fmt.stderr },
            init: init
              ? { code: init.code, stdout: init.stdout, stderr: init.stderr }
              : null,
            validate: validate
              ? {
                  code: validate.code,
                  stdout: validate.stdout,
                  stderr: validate.stderr
                }
              : null,
            tflint: tflint
              ? {
                  code: tflint.code,
                  stdout: tflint.stdout,
                  stderr: tflint.stderr
                }
              : null
          }
        }

        // memory store: write to tmp and lint
        const files = Array.from(memoryStore.entries()).map(([p, c]) => ({
          path: p,
          content: c
        }))
        const tmpDir = await writeFilesToTemp(files)
        const { fmt, init, validate, tflint } = await runLintPipeline(tmpDir)
        const fmtIndicatesChanges =
          (fmt.code !== 0 && fmt.code !== null) ||
          /\+{3}|-{3}|@@/.test(fmt.stdout)
        const validateFailed =
          validate && validate.code !== null && validate.code !== 0
        const tflintFailed = tflint && tflint.code !== null && tflint.code !== 0
        const status =
          fmtIndicatesChanges || validateFailed || tflintFailed
            ? "lint_failed"
            : "ok"
        return {
          status,
          files,
          fmt: { code: fmt.code, stdout: fmt.stdout, stderr: fmt.stderr },
          init: init
            ? { code: init.code, stdout: init.stdout, stderr: init.stderr }
            : null,
          validate: validate
            ? {
                code: validate.code,
                stdout: validate.stdout,
                stderr: validate.stderr
              }
            : null,
          tflint: tflint
            ? {
                code: tflint.code,
                stdout: tflint.stdout,
                stderr: tflint.stderr
              }
            : null
        }
      }
    },
    files_apply_diff: {
      description:
        "Apply unified diffs to provided Terraform files in-memory, then run terraform fmt/validate and tflint. Returns diagnostics and updated contents.",
      inputSchema: z.object({
        changes: z
          .array(
            z.object({
              path: z
                .string()
                .describe("Relative file path in the memory store."),
              patch: z
                .string()
                .describe(
                  "Unified diff (git-style) against current stored content."
                )
            })
          )
          .min(1)
      }),
      execute: async ({
        changes
      }: {
        changes: { path: string; patch: string }[]
      }) => {
        if (options?.onDiffApplied) {
          try {
            options.onDiffApplied(changes)
          } catch {
            // ignore callback errors
          }
        }
        const updatedFiles: { path: string; updated: string }[] = []
        const patchChangedPaths = new Set<string>()
        const unchangedByPatchPaths: string[] = []

        if (usingFS) {
          await ensureDir(baseDir!)
          // apply diffs
          for (const ch of changes) {
            const abs = safeResolve(ch.path)
            const current = await fs.readFile(abs, "utf8").catch(() => "")
            const nextContent = applyUnifiedDiff(current, ch.patch)
            await fs.mkdir(path.dirname(abs), { recursive: true })
            await fs.writeFile(abs, nextContent, "utf8")
            updatedFiles.push({ path: ch.path, updated: nextContent })
            if (nextContent !== current) {
              patchChangedPaths.add(ch.path)
            } else {
              unchangedByPatchPaths.push(ch.path)
            }
          }

          // lint/format in-place within baseDir
          const lintResults = await runLintPipeline(baseDir!)
          let fmt = lintResults.fmt
          const { init, validate, tflint } = lintResults
          let fmtIndicatesChanges =
            fmt.code !== 0 || /\+{3}|-{3}|@@/.test(fmt.stdout)

          const fmtChangedPaths: string[] = []
          if (fmtIndicatesChanges) {
            const beforeMap = new Map<string, string>()
            for (const f of updatedFiles) {
              beforeMap.set(f.path, f.updated)
            }
            await execCmd("terraform", ["fmt", "-no-color"], baseDir!)
            const refreshed: { path: string; updated: string }[] = []
            for (const f of updatedFiles) {
              const abs = safeResolve(f.path)
              const content = await fs.readFile(abs, "utf8")
              refreshed.push({ path: f.path, updated: content })
              if (beforeMap.get(f.path) !== content)
                fmtChangedPaths.push(f.path)
            }
            ;(updatedFiles as { path: string; updated: string }[]).length = 0
            ;(updatedFiles as { path: string; updated: string }[]).push(
              ...refreshed
            )

            fmt = await execCmd(
              "terraform",
              ["fmt", "-no-color", "-check"],
              baseDir!
            )
            fmtIndicatesChanges = fmt.code !== 0
          }

          const validateFailed =
            validate && validate.code !== null && validate.code !== 0
          const tflintFailed =
            tflint && tflint.code !== null && tflint.code !== 0

          const totalChangedPaths = new Set<string>([
            ...patchChangedPaths,
            ...fmtChangedPaths
          ])
          const noChanges = totalChangedPaths.size === 0

          const status =
            fmtIndicatesChanges || validateFailed || tflintFailed
              ? "lint_failed"
              : "ok"

          return {
            status,
            files: updatedFiles,
            noChanges,
            changedCount: totalChangedPaths.size,
            patchChangedCount: patchChangedPaths.size,
            fmtChangedCount: fmtChangedPaths.length,
            changedPaths: Array.from(totalChangedPaths.values()),
            unchangedPaths: unchangedByPatchPaths,
            fmt: { code: fmt.code, stdout: fmt.stdout, stderr: fmt.stderr },
            init: init
              ? { code: init.code, stdout: init.stdout, stderr: init.stderr }
              : null,
            validate: validate
              ? {
                  code: validate.code,
                  stdout: validate.stdout,
                  stderr: validate.stderr
                }
              : null,
            tflint: tflint
              ? {
                  code: tflint.code,
                  stdout: tflint.stdout,
                  stderr: tflint.stderr
                }
              : null
          }
        } else {
          // In-memory fallback
          for (const ch of changes) {
            const current = memoryStore.get(ch.path) ?? ""
            const nextContent = applyUnifiedDiff(current, ch.patch)
            memoryStore.set(ch.path, nextContent)
            updatedFiles.push({ path: ch.path, updated: nextContent })
            if (nextContent !== current) {
              patchChangedPaths.add(ch.path)
            } else {
              unchangedByPatchPaths.push(ch.path)
            }
          }

          const tmpDir = await writeFilesToTemp(
            updatedFiles.map((f) => ({ path: f.path, content: f.updated }))
          )

          const lintResults = await runLintPipeline(tmpDir)
          let fmt = lintResults.fmt
          const { init, validate, tflint } = lintResults

          let fmtIndicatesChanges =
            fmt.code !== 0 || /\+{3}|-{3}|@@/.test(fmt.stdout)

          // if fmt indicates changes, apply them and refresh contents
          const fmtChangedPaths: string[] = []
          if (fmtIndicatesChanges) {
            await execCmd("terraform", ["fmt", "-no-color"], tmpDir)

            const refreshed: { path: string; updated: string }[] = []
            const beforeFmt = new Map<string, string>(
              updatedFiles.map((f) => [f.path, f.updated])
            )
            for (const f of updatedFiles) {
              const abs = path.join(tmpDir, f.path)
              const content = await fs.readFile(abs, "utf8")
              memoryStore.set(f.path, content)
              refreshed.push({ path: f.path, updated: content })
              if (beforeFmt.get(f.path) !== content) {
                fmtChangedPaths.push(f.path)
              }
            }

            ;(updatedFiles as { path: string; updated: string }[]).length = 0
            ;(updatedFiles as { path: string; updated: string }[]).push(
              ...refreshed
            )

            fmt = await execCmd(
              "terraform",
              ["fmt", "-no-color", "-check"],
              tmpDir
            )
            fmtIndicatesChanges = fmt.code !== 0
          }
          const validateFailed =
            validate && validate.code !== null && validate.code !== 0
          const tflintFailed =
            tflint && tflint.code !== null && tflint.code !== 0

          const totalChangedPaths = new Set<string>([
            ...patchChangedPaths,
            ...fmtChangedPaths
          ])
          const noChanges = totalChangedPaths.size === 0

          const status =
            fmtIndicatesChanges || validateFailed || tflintFailed
              ? "lint_failed"
              : "ok"

          return {
            status,
            files: updatedFiles,
            noChanges,
            changedCount: totalChangedPaths.size,
            patchChangedCount: patchChangedPaths.size,
            fmtChangedCount: fmtChangedPaths.length,
            changedPaths: Array.from(totalChangedPaths.values()),
            unchangedPaths: unchangedByPatchPaths,
            fmt: { code: fmt.code, stdout: fmt.stdout, stderr: fmt.stderr },
            init: init
              ? { code: init.code, stdout: init.stdout, stderr: init.stderr }
              : null,
            validate: validate
              ? {
                  code: validate.code,
                  stdout: validate.stdout,
                  stderr: validate.stderr
                }
              : null,
            tflint: tflint
              ? {
                  code: tflint.code,
                  stdout: tflint.stdout,
                  stderr: tflint.stderr
                }
              : null
          }
        }
      }
    }
  }
  return tools
}
