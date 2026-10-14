import { describe, expect, test } from "bun:test"

import { isTerminalStage } from "@/backend"
import { toBackendError } from "@/mygo-backend"
import { failingBackend, MemoryBackend, memoryBackend } from "@/testing/memory-backend"

describe("toBackendError", () => {
  test("parses structured JSON from MyGo CallError", () => {
    const callErr = new Error(
      JSON.stringify({
        code: "RATE_LIMITED",
        category: "network",
        message: "rate limited by Amazon Photos",
        retryAfterMs: 3000,
        details: { statusCode: 429, endpoint: "/drive/v1/search" },
      }),
    )
    callErr.name = "CallError"

    const parsed = toBackendError(callErr)
    expect(parsed).toEqual({
      code: "RATE_LIMITED",
      category: "network",
      message: "rate limited by Amazon Photos",
      retryAfterMs: 3000,
      details: { statusCode: 429, endpoint: "/drive/v1/search" },
    })
  })

  test("preserves existing BackendError objects", () => {
    const existing = {
      code: "AUTH_FAILED",
      category: "auth",
      message: "session cookies expired",
    }
    expect(toBackendError(existing)).toEqual(existing)
  })

  test("falls back cleanly for plain Error and string messages", () => {
    expect(toBackendError(new Error("socket hang up"))).toEqual({
      code: "UNKNOWN_ERROR",
      category: "internal",
      message: "socket hang up",
    })
    expect(toBackendError("unexpected string error")).toEqual({
      code: "UNKNOWN_ERROR",
      category: "internal",
      message: "unexpected string error",
    })
  })
})

describe("isTerminalStage", () => {
  test("identifies terminal and non-terminal stages", () => {
    expect(isTerminalStage("completed")).toBe(true)
    expect(isTerminalStage("partial")).toBe(true)
    expect(isTerminalStage("failed")).toBe(true)
    expect(isTerminalStage("cancelled")).toBe(true)
    expect(isTerminalStage("unconfirmed")).toBe(true)
    expect(isTerminalStage("queued")).toBe(false)
    expect(isTerminalStage("running")).toBe(false)
    expect(isTerminalStage("paused")).toBe(false)
  })
})

describe("MemoryBackend", () => {
  test("manages auth status, cookie import, logout, usage, and emits authChanged", async () => {
    const backend = memoryBackend({
      auth: { configured: false, authenticated: false, region: "" },
    })
    const events: string[] = []
    const off = backend.events.onAuthChanged((st) => {
      events.push(`${st.configured}:${st.authenticated}:${st.region}`)
    })

    const initial = await backend.auth.status()
    expect(initial.configured).toBe(false)
    expect(initial.authenticated).toBe(false)

    await expect(
      backend.auth.importCookies({
        rawCookies: "session-id=111; ubid-main=222; at-main=333",
        region: "",
      }),
    ).resolves.toMatchObject({
      configured: true,
      authenticated: true,
      region: "us",
    })

    expect(events).toEqual(["true:true:us"])

    const usage = await backend.auth.usage()
    expect(usage.totalItems).toBeGreaterThan(0)

    const loggedOut = await backend.auth.logout()
    expect(loggedOut.configured).toBe(false)
    expect(loggedOut.authenticated).toBe(false)
    expect(events).toEqual(["true:true:us", "false:false:us"])
    off()
  })

  test("rejects ambiguous cookie import without explicit region", async () => {
    const backend = new MemoryBackend({
      auth: { configured: false, authenticated: false, region: "" },
    })
    await expect(
      backend.auth.importCookies({
        rawCookies: "session-id=111",
        region: "",
      }),
    ).rejects.toMatchObject({
      code: "CONFIG_ERROR",
      category: "config",
    })
  })

  test("plans uploads, starts transfers, transitions stages, and clears history", async () => {
    const backend = new MemoryBackend()
    const stages: string[] = []
    backend.events.onTransferStage((tr) => stages.push(`${tr.id}:${tr.stage}`))

    const plan = await backend.transfers.planUpload({
      paths: ["/Users/alice/Photos/IMG_0001.JPG", "/Users/alice/Photos/clip.mp4", "/Users/alice/Photos/notes.txt"],
      conflictPolicy: "skip",
    })
    expect(plan.photoCount).toBe(1)
    expect(plan.videoCount).toBe(1)
    expect(plan.totalCount).toBe(2)
    expect(plan.skippedUnsupported).toBe(1)

    const started = await backend.transfers.startUpload({
      paths: ["/Users/alice/Photos/IMG_0001.JPG", "/Users/alice/Photos/clip.mp4"],
      conflictPolicy: "skip",
    })
    expect(started.stage).toBe("running")

    const paused = await backend.transfers.pause(started.id)
    expect(paused.stage).toBe("paused")

    const resumed = await backend.transfers.resume(started.id)
    expect(resumed.stage).toBe("running")

    const cancelled = await backend.transfers.cancel(started.id)
    expect(cancelled.stage).toBe("cancelled")

    const retried = await backend.transfers.retry(started.id)
    expect(retried.stage).toBe("running")

    backend.completeTransferForTest(retried.id)
    const listAfterComplete = await backend.transfers.list()
    expect(listAfterComplete.active).toHaveLength(0)
    expect(listAfterComplete.history).toHaveLength(1)
    expect(listAfterComplete.history[0]?.stage).toBe("completed")

    await backend.transfers.clearHistory()
    const listAfterClear = await backend.transfers.list()
    expect(listAfterClear.history).toHaveLength(0)
    expect(stages.length).toBeGreaterThanOrEqual(5)
  })

  test("manages one-way backup rules and scanNow", async () => {
    const backend = new MemoryBackend()
    const statuses: string[] = []
    backend.events.onBackupStatus((r) => statuses.push(`${r.id}:${r.status}`))

    const added = await backend.backups.add({
      localPath: "/Volumes/External/Photos",
      includeVideos: true,
      scheduleMode: "interval",
      intervalMinutes: 60,
      conflictPolicy: "skip",
    })
    expect(added.status).toBe("active")

    const paused = await backend.backups.pause(added.id)
    expect(paused.paused).toBe(true)
    expect(paused.status).toBe("paused")

    const resumed = await backend.backups.resume(added.id)
    expect(resumed.paused).toBe(false)
    expect(resumed.status).toBe("active")

    const scanTransfer = await backend.backups.scanNow(added.id)
    expect(scanTransfer).not.toBeNull()
    expect(scanTransfer?.kind).toBe("backup")

    await backend.backups.remove(added.id)
    expect(await backend.backups.list()).toHaveLength(0)
    expect(statuses.length).toBeGreaterThanOrEqual(3)
  })

  test("supports recovery downloads and settings updates with validation", async () => {
    const backend = new MemoryBackend()
    const sources = await backend.downloads.listDestinations()
    expect(sources.confirmedItems.length).toBeGreaterThan(0)

    const dl = await backend.downloads.startDownload({
      nodeIds: [sources.confirmedItems[0]!.remoteNodeId ?? "node-1"],
      destDir: "/Users/alice/Restored",
      conflictPolicy: "rename",
    })
    expect(dl.kind).toBe("download")
    expect(dl.conflictPolicy).toBe("rename")
    const updated = await backend.settings.update({
      region: "ca",
      maxMetadataConcurrency: 4,
      maxUploadConcurrency: 2,
      defaultConflictPolicy: "rename",
      closeToTray: false,
    })
    expect(updated.region).toBe("ca")
    expect(updated.defaultConflictPolicy).toBe("rename")
    expect(updated.closeToTray).toBe(false)

    await expect(
      backend.settings.update({ maxUploadConcurrency: 99 }),
    ).rejects.toMatchObject({
      code: "VALIDATION_FAILED",
      category: "validation",
    })

    const doctor = await backend.settings.doctor()
    expect(doctor.healthy).toBe(true)
    expect(doctor.checks).toHaveLength(5)

    const doctorProbe = await backend.settings.doctorWithWriteProbe(true)
    expect(doctorProbe.healthy).toBe(true)
    expect(doctorProbe.checks.find((c) => c.grade === 5)?.status).toBe("pass")
  })
})

describe("failingBackend", () => {
  test("rejects every service call with the configured BackendError", async () => {
    const err = {
      code: "NETWORK_ERROR",
      category: "network",
      message: "connection refused",
    }
    const backend = failingBackend(err)

    await expect(backend.auth.status()).rejects.toEqual(err)
    await expect(backend.transfers.list()).rejects.toEqual(err)
    await expect(backend.backups.list()).rejects.toEqual(err)
    await expect(backend.downloads.listDestinations()).rejects.toEqual(err)
    await expect(backend.settings.get()).rejects.toEqual(err)
  })
})
