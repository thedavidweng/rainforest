import { isCallError, isMyGo, onFileDrop } from "mygo-runtime"

import type { Backend, BackendError, FilesDroppedEvent } from "@/backend"
import { Auth, Backups, Downloads, events, Settings, Transfers } from "@/mygo"

function extractBackendError(val: unknown, fallbackMessage: string): BackendError | null {
  if (typeof val !== "object" || val === null || !("code" in val) || typeof val.code !== "string") {
    return null
  }
  const category = "category" in val && typeof val.category === "string" ? val.category : "internal"
  const message = "message" in val && typeof val.message === "string" ? val.message : fallbackMessage
  const retryAfterMs = "retryAfterMs" in val && typeof val.retryAfterMs === "number" ? val.retryAfterMs : undefined
  const details =
    "details" in val && typeof val.details === "object" && val.details !== null && !Array.isArray(val.details)
      ? (val.details as Record<string, unknown>)
      : undefined
  return {
    code: val.code,
    category,
    message,
    ...(retryAfterMs !== undefined ? { retryAfterMs } : {}),
    ...(details !== undefined ? { details } : {}),
  }
}

function tryParseErrorJSON(raw: string): BackendError | null {
  const trimmed = raw.trim()
  if (!trimmed.startsWith("{")) {
    return null
  }
  try {
    return extractBackendError(JSON.parse(trimmed), raw)
  } catch {
    return null
  }
}
export function toBackendError(err: unknown): BackendError {
  if (isCallError(err)) {
    const parsed = tryParseErrorJSON(err.message)
    if (parsed) return parsed
    return {
      code: "CALL_ERROR",
      category: "internal",
      message: err.message,
    }
  }
  if (!(err instanceof Error)) {
    const extracted = extractBackendError(err, "")
    if (extracted) return extracted
  }
  if (err instanceof Error) {
    const parsed = tryParseErrorJSON(err.message)
    if (parsed) return parsed
    return {
      code: "UNKNOWN_ERROR",
      category: "internal",
      message: err.message,
    }
  }
  if (typeof err === "string") {
    const parsed = tryParseErrorJSON(err)
    if (parsed) return parsed
    return {
      code: "UNKNOWN_ERROR",
      category: "internal",
      message: err,
    }
  }
  return {
    code: "UNKNOWN_ERROR",
    category: "internal",
    message: String(err),
  }
}

async function call<T>(promise: Promise<T>): Promise<T> {
  try {
    return await promise
  } catch (err) {
    throw toBackendError(err)
  }
}

async function callRequired<T>(promise: Promise<T | null>, label: string): Promise<T> {
  const res = await call(promise)
  if (res === null || res === undefined) {
    throw {
      code: "UNKNOWN_ERROR",
      category: "internal",
      message: `empty ${label} response`,
    } satisfies BackendError
  }
  return res
}

function safeSubscribe<T>(register: (cb: (payload: T) => void) => () => void, cb: (payload: T) => void): () => void {
  if (!isMyGo()) {
    return () => {}
  }
  try {
    return register(cb)
  } catch {
    return () => {}
  }
}

export const mygoBackend: Backend = {
  auth: {
    status: () => callRequired(Auth.status(), "auth status"),
    importCookies: (input) => callRequired(Auth.importCookies(input), "auth status"),
    importCookieFile: (path, region) => callRequired(Auth.importCookieFile(path, region), "auth status"),
    pickAndImportCookieFile: (region) => callRequired(Auth.pickAndImportCookieFile(region), "auth status"),
    logout: () => callRequired(Auth.logout(), "auth status"),
    usage: () => callRequired(Auth.usage(), "usage summary"),
  },
  transfers: {
    planUpload: (input) => callRequired(Transfers.planUpload(input), "upload plan"),
    startUpload: (input) => callRequired(Transfers.startUpload(input), "transfer"),
    list: async () => (await call(Transfers.list())) ?? { active: [], history: [] },
    getItems: async (transferID) => (await call(Transfers.getItems(transferID))) ?? [],
    pause: (transferID) => callRequired(Transfers.pause(transferID), "transfer"),
    resume: (transferID) => callRequired(Transfers.resume(transferID), "transfer"),
    cancel: (transferID) => callRequired(Transfers.cancel(transferID), "transfer"),
    retry: (transferID) => callRequired(Transfers.retry(transferID), "transfer"),
    clearHistory: () => call(Transfers.clearHistory()),
    pickFiles: async () => (await call(Transfers.pickFiles())) ?? [],
    pickDirectory: async () => (await call(Transfers.pickDirectory())) ?? "",
  },
  backups: {
    list: async () => (await call(Backups.list())) ?? [],
    add: (input) => callRequired(Backups.add(input), "backup rule"),
    update: (id, input) => callRequired(Backups.update(id, input), "backup rule"),
    pause: (id) => callRequired(Backups.pause(id), "backup rule"),
    resume: (id) => callRequired(Backups.resume(id), "backup rule"),
    remove: (id) => call(Backups.remove(id)),
    scanNow: (id) => call(Backups.scanNow(id)),
    setAllStopped: (stopped) => call(Backups.setAllStopped(stopped)),
    pickFolder: async () => (await call(Backups.pickFolder())) ?? "",
  },
  downloads: {
    listSources: () => callRequired(Downloads.listSources(), "recovery sources"),
    listDestinations: () => callRequired(Downloads.listDestinations(), "recovery destinations"),
    startDownload: (input) => callRequired(Downloads.startDownload(input), "download transfer"),
    pickTargetDirectory: async () => (await call(Downloads.pickTargetDirectory())) ?? "",
  },
  settings: {
    get: () => callRequired(Settings.get(), "settings"),
    update: (input) => callRequired(Settings.update(input), "settings"),
    doctor: () => callRequired(Settings.doctor(), "doctor report"),
  },
  events: {
    onTransferStage: (cb) => safeSubscribe(events.transferStage.on, cb),
    onTransferProgress: (cb) => safeSubscribe(events.transferProgress.on, cb),
    onBackupStatus: (cb) => safeSubscribe(events.backupStatus.on, cb),
    onAuthChanged: (cb) => safeSubscribe(events.authChanged.on, cb),
    onFilesDropped: (cb) => {
      if (!isMyGo()) {
        return () => {}
      }
      let lastKey = ""
      let lastAt = 0
      const deliver = (ev: FilesDroppedEvent) => {
        if (!ev.paths || ev.paths.length === 0) return
        const now = Date.now()
        const key = ev.paths.join("\0")
        if (key === lastKey && now - lastAt < 75) {
          return
        }
        lastKey = key
        lastAt = now
        cb(ev)
      }
      const offCustom = safeSubscribe(events.filesDropped.on, deliver)
      const offNative = safeSubscribe(onFileDrop, (drop) =>
        deliver({ paths: drop.paths, x: drop.x, y: drop.y }),
      )
      return () => {
        offCustom()
        offNative()
      }
    },
    onNotification: (cb) => safeSubscribe(events.notification.on, cb),
  },
}
