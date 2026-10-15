import type {
  AuthStatus,
  Backend,
  BackendError,
  BackupRuleDTO,
  BackupRuleInput,
  DoctorCheckDTO,
  DoctorReportDTO,
  DownloadInput,
  FilesDroppedEvent,
  ImportCookiesInput,
  NotificationEvent,
  PlanItemDTO,
  RecoveryContainerDTO,
  RecoverySourcesDTO,
  SettingsDTO,
  SettingsInput,
  StartUploadInput,
  TransferDTO,
  TransferItemDTO,
  TransferListDTO,
  TransferProgressEvent,
  UploadPlanInput,
  UsageSummary,
} from "@/backend"
import { isTerminalStage } from "@/backend"

export function eventEmitter<T>() {
  const subs = new Set<(v: T) => void>()
  return {
    subscribe(cb: (v: T) => void): () => void {
      subs.add(cb)
      return () => subs.delete(cb)
    },
    emit(v: T) {
      for (const cb of subs) cb(v)
    },
  }
}

export interface LocalFileFixture {
  path: string
  name?: string
  mediaType?: "photo" | "video" | "other"
  size?: number
  width?: number
  height?: number
  thumbnailDataUrl?: string
  md5?: string
}

export interface MemoryBackendOptions {
  auth?: Partial<AuthStatus>
  usage?: Partial<UsageSummary>
  settings?: Partial<SettingsDTO>
  doctor?: DoctorReportDTO
  destinations?: Record<string, { name: string; kind: string; status?: string }>
  backups?: BackupRuleDTO[]
  transfers?: TransferDTO[]
  transferItems?: Record<string, TransferItemDTO[]>
  recoverySources?: Partial<RecoverySourcesDTO>
  localFiles?: LocalFileFixture[]
  pickedFiles?: string[]
  pickedDirectory?: string
  pickedCookieFile?: string
  pickedCookieContent?: string
  unavailableBackupPaths?: string[]
}

const PHOTO_EXTS: Record<string, true> = {
  ".jpg": true,
  ".jpeg": true,
  ".png": true,
  ".heic": true,
  ".heif": true,
  ".webp": true,
  ".gif": true,
  ".tif": true,
  ".tiff": true,
  ".bmp": true,
  ".raw": true,
  ".cr2": true,
  ".cr3": true,
  ".nef": true,
  ".arw": true,
  ".dng": true,
  ".orf": true,
  ".rw2": true,
}

const VIDEO_EXTS: Record<string, true> = {
  ".mp4": true,
  ".mov": true,
  ".m4v": true,
  ".avi": true,
  ".mkv": true,
  ".webm": true,
  ".mts": true,
  ".m2ts": true,
  ".3gp": true,
}

function classifyPathMediaType(filePath: string): "photo" | "video" | "other" {
  const slash = Math.max(filePath.lastIndexOf("/"), filePath.lastIndexOf("\\"))
  const base = slash >= 0 ? filePath.slice(slash + 1) : filePath
  const dot = base.lastIndexOf(".")
  if (dot < 0) return "other"
  const ext = base.slice(dot).toLowerCase()
  if (PHOTO_EXTS[ext]) return "photo"
  if (VIDEO_EXTS[ext]) return "video"
  return "other"
}

function fileBaseName(filePath: string): string {
  const trimmed = filePath.replace(/[\\/]+$/, "")
  const slash = Math.max(trimmed.lastIndexOf("/"), trimmed.lastIndexOf("\\"))
  return slash >= 0 ? trimmed.slice(slash + 1) : trimmed
}

const VALID_REGIONS: Record<string, true> = {
  us: true,
  ca: true,
  uk: true,
  jp: true,
  de: true,
  fr: true,
  it: true,
  es: true,
}

function evidenceForRegion(region: string): string {
  const r = region.trim().toLowerCase()
  if (r === "us") return "verified"
  if (VALID_REGIONS[r]) return "experimental"
  return ""
}

function inferRegionFromCookies(raw: string): string {
  const lower = raw.toLowerCase()
  if (
    lower.includes("ubid-main") ||
    lower.includes("ubid_main") ||
    lower.includes("at-main") ||
    lower.includes("at_main")
  ) {
    return "us"
  }
  for (const r of ["ca", "uk", "jp", "de", "fr", "it", "es"]) {
    if (
      lower.includes(`ubid-acb${r}`) ||
      lower.includes(`ubid_acb${r}`) ||
      lower.includes(`at-acb${r}`) ||
      lower.includes(`at_acb${r}`)
    ) {
      return r
    }
  }
  return ""
}
const VALID_POLICIES: Record<string, true> = { skip: true, rename: true, fail: true }

const SAMPLE_THUMBNAIL =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

export class MemoryBackend implements Backend {
  private authState: AuthStatus
  private usageState: UsageSummary
  private settingsState: SettingsDTO
  private customDoctor?: DoctorReportDTO
  private backupRules: BackupRuleDTO[]
  private transferRecords: TransferDTO[]
  private itemsByTransfer = new Map<string, TransferItemDTO[]>()
  private recoveryState: RecoverySourcesDTO
  private localFilesByPath = new Map<string, LocalFileFixture>()
  private unavailablePaths = new Set<string>()
  private pickedFiles: string[]
  private pickedDirectory: string
  private pickedCookieFile: string
  private pickedCookieContent: string
  private destinations: NonNullable<MemoryBackendOptions["destinations"]>
  private seq = 1

  private readonly transferStageEvents = eventEmitter<TransferDTO>()
  private readonly transferProgressEvents = eventEmitter<TransferProgressEvent>()
  private readonly backupStatusEvents = eventEmitter<BackupRuleDTO>()
  private readonly authChangedEvents = eventEmitter<AuthStatus>()
  private readonly filesDroppedEvents = eventEmitter<FilesDroppedEvent>()
  private readonly notificationEvents = eventEmitter<NotificationEvent>()

  constructor(opts: MemoryBackendOptions = {}) {
    this.destinations = opts.destinations ?? {}
    const defaultRegion = opts.auth?.region ?? opts.settings?.region ?? "us"
    const configured = opts.auth?.configured ?? true
    const authenticated = opts.auth?.authenticated ?? configured

    this.authState = {
      configured,
      authenticated,
      region: defaultRegion,
      evidence: opts.auth?.evidence ?? evidenceForRegion(defaultRegion),
      rootNodeId: opts.auth?.rootNodeId ?? (authenticated ? "root-node-us" : undefined),
      ownerId: opts.auth?.ownerId ?? (authenticated ? "owner-1" : undefined),
      cookieKeys:
        opts.auth?.cookieKeys ?? (configured ? ["at-main", "session-id", "ubid-main"] : undefined),
      vaultPath: opts.auth?.vaultPath ?? "/Users/alice/.config/rainforest/credentials.json",
      errorCode: opts.auth?.errorCode,
      errorMessage: opts.auth?.errorMessage,
    }

    this.settingsState = {
      region: opts.settings?.region ?? defaultRegion,
      evidence:
        opts.settings?.evidence ?? evidenceForRegion(opts.settings?.region ?? defaultRegion),
      maxMetadataConcurrency: opts.settings?.maxMetadataConcurrency ?? 4,
      maxUploadConcurrency: opts.settings?.maxUploadConcurrency ?? 2,
      defaultConflictPolicy: opts.settings?.defaultConflictPolicy ?? "skip",
      launchAtLogin: opts.settings?.launchAtLogin ?? false,
      closeToTray: opts.settings?.closeToTray ?? true,
      notificationsEnabled: opts.settings?.notificationsEnabled ?? true,
      backupsStopped: opts.settings?.backupsStopped ?? false,
    }

    this.usageState = {
      lastCalculated: opts.usage?.lastCalculated ?? "2026-10-14T12:00:00Z",
      photo: opts.usage?.photo ?? {
        billable: { bytes: 0, count: 0 },
        total: { bytes: 4_200_000_000, count: 1420 },
      },
      video: opts.usage?.video ?? {
        billable: { bytes: 1_800_000_000, count: 48 },
        total: { bytes: 1_800_000_000, count: 48 },
      },
      other: opts.usage?.other ?? {
        billable: { bytes: 0, count: 0 },
        total: { bytes: 0, count: 0 },
      },
      doc: opts.usage?.doc ?? {
        billable: { bytes: 0, count: 0 },
        total: { bytes: 0, count: 0 },
      },
      totalBytes: opts.usage?.totalBytes ?? 6_000_000_000,
      totalBillableBytes: opts.usage?.totalBillableBytes ?? 1_800_000_000,
      totalItems: opts.usage?.totalItems ?? 1468,
    }

    this.customDoctor = opts.doctor
    this.backupRules = (opts.backups ?? []).map((b) => ({ ...b }))
    this.transferRecords = (opts.transfers ?? []).map((t) => ({ ...t }))

    if (opts.transferItems) {
      for (const [tid, items] of Object.entries(opts.transferItems)) {
        this.itemsByTransfer.set(
          tid,
          items.map((it) => ({ ...it })),
        )
      }
    }

    for (const f of opts.localFiles ?? []) {
      this.localFilesByPath.set(f.path, { ...f })
    }
    for (const p of opts.unavailableBackupPaths ?? []) {
      this.unavailablePaths.add(p)
    }

    this.pickedFiles = opts.pickedFiles ?? ["/Users/alice/Photos/IMG_1001.JPG"]
    this.pickedDirectory = opts.pickedDirectory ?? "/Users/alice/Photos/Vacation"
    this.pickedCookieFile = opts.pickedCookieFile ?? "/Users/alice/Downloads/cookies.txt"
    this.pickedCookieContent =
      opts.pickedCookieContent ??
      "session-id=135-1111111-1111111; ubid-main=133-2222222-2222222; at-main=Atza|test"

    const defaultConfirmed: TransferItemDTO[] = [
      {
        id: "item-confirmed-1",
        transferId: "tr-history-1",
        localPath: "/Users/alice/Photos/IMG_0042.JPG",
        name: "IMG_0042.JPG",
        mediaType: "photo",
        size: 3_145_728,
        md5: "d41d8cd98f00b204e9800998ecf8427e",
        remoteNodeId: "node-confirmed-1",
        status: "confirmed",
        bytesDone: 3_145_728,
        updatedAt: "2026-10-14T10:00:00Z",
      },
    ]

    const defaultFolders: RecoveryContainerDTO[] = [
      { id: "folder-2026", name: "2026 Backups", kind: "FOLDER" },
    ]
    const defaultAlbums: RecoveryContainerDTO[] = [
      { id: "album-family", name: "Family Trip", kind: "VISUAL_COLLECTION" },
    ]

    this.recoveryState = {
      region: opts.recoverySources?.region ?? (defaultRegion || "us"),
      evidence:
        opts.recoverySources?.evidence ?? (evidenceForRegion(defaultRegion || "us") || "verified"),
      rootNodeId: opts.recoverySources?.rootNodeId ?? "root-node-us",
      confirmedItems: opts.recoverySources?.confirmedItems
        ? opts.recoverySources.confirmedItems.map((i) => ({ ...i }))
        : defaultConfirmed,
      folders: opts.recoverySources?.folders
        ? opts.recoverySources.folders.map((f) => ({ ...f }))
        : defaultFolders,
      albums: opts.recoverySources?.albums
        ? opts.recoverySources.albums.map((a) => ({ ...a }))
        : defaultAlbums,
      capabilityNotes:
        opts.recoverySources?.capabilityNotes ??
        "Recovery download supports confirmed transfer history items, explicit remote node IDs, and verified folders or albums.",
      warning: opts.recoverySources?.warning,
    }
  }

  readonly auth: Backend["auth"] = {
    status: async () => ({ ...this.authState }),

    importCookies: async (input: ImportCookiesInput) => {
      const raw = input.rawCookies.trim()
      if (!raw) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "cookie input is empty",
        } satisfies BackendError
      }
      if (!raw.toLowerCase().includes("session-id") && !raw.toLowerCase().includes("session_id")) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "missing required session-id cookie",
        } satisfies BackendError
      }

      const explicit = (input.region ?? "").trim().toLowerCase()
      const inferred = inferRegionFromCookies(raw)
      const resolved = explicit || inferred
      if (!resolved) {
        throw {
          code: "CONFIG_ERROR",
          category: "config",
          message:
            "region cannot be determined from cookies; select an explicit Amazon marketplace region",
          details: { field: "region" },
        } satisfies BackendError
      }
      if (!VALID_REGIONS[resolved]) {
        throw {
          code: "CONFIG_ERROR",
          category: "config",
          message: `unsupported region "${resolved}"`,
          details: { field: "region" },
        } satisfies BackendError
      }

      const evidence = evidenceForRegion(resolved)
      this.authState = {
        configured: true,
        authenticated: true,
        region: resolved,
        evidence,
        rootNodeId: `root-node-${resolved}`,
        ownerId: "owner-1",
        cookieKeys:
          resolved === "us"
            ? ["at-main", "session-id", "ubid-main"]
            : [`at-acb${resolved}`, "session-id", `ubid-acb${resolved}`],
        vaultPath: this.authState.vaultPath ?? "/Users/alice/.config/rainforest/credentials.json",
      }
      this.settingsState = {
        ...this.settingsState,
        region: resolved,
        evidence,
      }
      this.recoveryState = {
        ...this.recoveryState,
        region: resolved,
        evidence,
      }
      this.authChangedEvents.emit({ ...this.authState })
      return { ...this.authState }
    },

    importCookieFile: async (path: string, region: string) => {
      if (!path.trim()) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "cookie file path is required",
        } satisfies BackendError
      }
      return this.auth.importCookies({
        rawCookies: this.pickedCookieContent,
        region,
      })
    },

    pickAndImportCookieFile: async (region: string) => {
      if (!this.pickedCookieFile) {
        return { ...this.authState }
      }
      return this.auth.importCookieFile(this.pickedCookieFile, region)
    },

    logout: async () => {
      this.authState = {
        configured: false,
        authenticated: false,
        region: this.settingsState.region,
        evidence: this.settingsState.evidence,
        vaultPath: this.authState.vaultPath,
      }
      this.authChangedEvents.emit({ ...this.authState })
      return { ...this.authState }
    },

    usage: async () => {
      if (!this.authState.authenticated) {
        throw {
          code: "AUTH_REQUIRED",
          category: "auth",
          message: "Amazon Photos credentials are not configured",
        } satisfies BackendError
      }
      return structuredClone(this.usageState)
    },
  }

  readonly transfers: Backend["transfers"] = {
    probeDestination: async (id: string) => {
      const region = this.settingsState.region
      const node = id ? this.destinations[id] : { name: "Account root", kind: "FOLDER" }
      const authenticated = this.authState.authenticated
      const status = !authenticated
        ? "pending"
        : node &&
            (!node.status || node.status === "AVAILABLE") &&
            ["FOLDER", "VISUAL_COLLECTION"].includes(node.kind)
          ? "readable"
          : "failed"
      return {
        region,
        evidence: evidenceForRegion(region),
        destinationNodeId: id,
        name: node?.name ?? "",
        kind: node?.kind ?? "",
        status,
        message:
          status === "pending"
            ? "Credentials unavailable; authenticate before uploading."
            : status === "readable"
              ? "Target is readable. Upload and album membership are confirmed separately."
              : "Destination unavailable or unsupported.",
      }
    },
    planUpload: async (input: UploadPlanInput) => {
      const rawPaths = (input.paths ?? []).map((p) => p.trim()).filter(Boolean)
      if (rawPaths.length === 0) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "at least one file or folder path is required",
        } satisfies BackendError
      }

      const includeVideos = input.includeVideos ?? true
      const includePreviews = input.includePreviews ?? true
      const policy =
        (input.conflictPolicy ?? "").trim() || this.settingsState.defaultConflictPolicy || "skip"

      const expanded = this.expandCandidatePaths(rawPaths)
      const items: PlanItemDTO[] = []
      let photoCount = 0
      let videoCount = 0
      let totalBytes = 0
      let skippedUnsupported = 0

      for (const candidate of expanded) {
        if (candidate.mediaType === "photo") {
          photoCount++
        } else if (candidate.mediaType === "video") {
          if (!includeVideos) {
            skippedUnsupported++
            continue
          }
          videoCount++
        } else {
          skippedUnsupported++
          continue
        }

        const size = candidate.size ?? 2_097_152
        totalBytes += size
        items.push({
          localPath: candidate.path,
          name: candidate.name ?? fileBaseName(candidate.path),
          mediaType: candidate.mediaType,
          size,
          width: candidate.width ?? (candidate.mediaType === "photo" ? 4032 : 1920),
          height: candidate.height ?? (candidate.mediaType === "photo" ? 3024 : 1080),
          ...(includePreviews && candidate.mediaType === "photo"
            ? { thumbnailDataUrl: candidate.thumbnailDataUrl ?? SAMPLE_THUMBNAIL }
            : {}),
        })
      }

      return {
        sourceSummary:
          rawPaths.length === 1
            ? fileBaseName(rawPaths[0]!)
            : `${fileBaseName(rawPaths[0]!)} +${rawPaths.length - 1} more`,
        photoCount,
        videoCount,
        totalCount: items.length,
        totalBytes,
        skippedUnsupported,
        conflictPolicy: policy,
        destination: await this.transfers.probeDestination((input.destinationNodeId ?? "").trim()),
        items,
      }
    },

    startUpload: async (input: StartUploadInput) => {
      const plan = await this.transfers.planUpload({
        paths: input.paths,
        destinationNodeId: input.destinationNodeId,
        conflictPolicy: input.conflictPolicy,
        includeVideos: input.includeVideos,
        includePreviews: false,
      })
      if (plan.destination.status !== "readable") throw new Error(plan.destination.message)
      if (plan.totalCount === 0) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "no supported photo or video files found in selection",
        } satisfies BackendError
      }

      const id = `tr-${this.seq++}`
      const now = new Date().toISOString()
      const tr: TransferDTO = {
        id,
        kind: "upload",
        stage: "running",
        sourceSummary: plan.sourceSummary,
        ...(input.destinationNodeId ? { destNodeId: input.destinationNodeId } : {}),
        conflictPolicy: plan.conflictPolicy,
        bytesDone: 0,
        bytesTotal: plan.totalBytes,
        itemsDone: 0,
        itemsSkipped: 0,
        itemsFailed: 0,
        itemsUnconfirmed: 0,
        itemsTotal: plan.totalCount,
        createdAt: now,
        updatedAt: now,
      }

      const items: TransferItemDTO[] = plan.items.map((p, idx) => ({
        id: `${id}-item-${idx + 1}`,
        transferId: id,
        localPath: p.localPath,
        name: p.name,
        mediaType: p.mediaType,
        size: p.size,
        status: "queued",
        bytesDone: 0,
        updatedAt: now,
      }))

      this.transferRecords.unshift(tr)
      this.itemsByTransfer.set(id, items)
      this.transferStageEvents.emit({ ...tr })
      return { ...tr }
    },

    list: async (): Promise<TransferListDTO> => {
      const active: TransferDTO[] = []
      const history: TransferDTO[] = []
      for (const t of this.transferRecords) {
        if (isTerminalStage(t.stage)) {
          history.push({ ...t })
        } else {
          active.push({ ...t })
        }
      }
      return { active, history }
    },

    getItems: async (transferID: string) => {
      const items = this.itemsByTransfer.get(transferID.trim()) ?? []
      return items.map((i) => ({ ...i }))
    },

    pause: async (transferID: string) => {
      const tr = this.requireTransfer(transferID)
      tr.stage = "paused"
      tr.updatedAt = new Date().toISOString()
      this.transferStageEvents.emit({ ...tr })
      return { ...tr }
    },

    resume: async (transferID: string) => {
      const tr = this.requireTransfer(transferID)
      tr.stage = "running"
      tr.updatedAt = new Date().toISOString()
      this.transferStageEvents.emit({ ...tr })
      return { ...tr }
    },

    cancel: async (transferID: string) => {
      const tr = this.requireTransfer(transferID)
      const now = new Date().toISOString()
      tr.stage = "cancelled"
      tr.updatedAt = now
      tr.finishedAt = now
      const items = this.itemsByTransfer.get(tr.id) ?? []
      for (const it of items) {
        if (it.status === "queued" || it.status === "running") {
          it.status = "cancelled"
          it.updatedAt = now
        }
      }
      this.transferStageEvents.emit({ ...tr })
      return { ...tr }
    },

    retry: async (transferID: string) => {
      const tr = this.requireTransfer(transferID)
      const now = new Date().toISOString()
      tr.stage = "running"
      tr.errorCode = undefined
      tr.errorMessage = undefined
      tr.finishedAt = undefined
      tr.itemsFailed = 0
      tr.itemsUnconfirmed = 0
      tr.updatedAt = now
      const items = this.itemsByTransfer.get(tr.id) ?? []
      for (const it of items) {
        if (it.status !== "confirmed" && it.status !== "skipped") {
          it.status = "queued"
          it.errorMessage = undefined
          it.updatedAt = now
        }
      }
      this.transferStageEvents.emit({ ...tr })
      return { ...tr }
    },

    clearHistory: async () => {
      const kept: TransferDTO[] = []
      for (const tr of this.transferRecords) {
        if (isTerminalStage(tr.stage)) {
          this.itemsByTransfer.delete(tr.id)
        } else {
          kept.push(tr)
        }
      }
      this.transferRecords = kept
    },

    pickFiles: async () => [...this.pickedFiles],
    pickDirectory: async () => this.pickedDirectory,
  }

  readonly backups: Backend["backups"] = {
    list: async () => this.backupRules.map((b) => ({ ...b })),

    add: async (input: BackupRuleInput) => {
      const localPath = input.localPath.trim()
      if (!localPath) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "backup folder path is required",
        } satisfies BackendError
      }
      const now = new Date().toISOString()
      const unavailable = this.unavailablePaths.has(localPath)
      const paused = input.paused ?? false
      const rule: BackupRuleDTO = {
        id: `backup-${this.seq++}`,
        localPath,
        includeVideos: input.includeVideos,
        scheduleMode: input.scheduleMode?.trim() || "immediate",
        intervalMinutes:
          input.intervalMinutes && input.intervalMinutes > 0 ? input.intervalMinutes : 30,
        conflictPolicy:
          input.conflictPolicy?.trim() || this.settingsState.defaultConflictPolicy || "skip",
        ...(input.destinationNodeId ? { destinationNodeId: input.destinationNodeId.trim() } : {}),
        paused,
        status: unavailable ? "source_unavailable" : paused ? "paused" : "active",
        ...(unavailable ? { lastError: "source directory is unavailable or unmounted" } : {}),
        createdAt: now,
        updatedAt: now,
      }
      this.backupRules.push(rule)
      this.backupStatusEvents.emit({ ...rule })
      return { ...rule }
    },

    update: async (id: string, input: BackupRuleInput) => {
      const rule = this.requireBackupRule(id)
      const localPath = input.localPath.trim()
      if (!localPath) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "backup folder path is required",
        } satisfies BackendError
      }
      const unavailable = this.unavailablePaths.has(localPath)
      rule.localPath = localPath
      rule.includeVideos = input.includeVideos
      if (input.scheduleMode) rule.scheduleMode = input.scheduleMode.trim()
      if (input.intervalMinutes && input.intervalMinutes > 0) {
        rule.intervalMinutes = input.intervalMinutes
      }
      if (input.conflictPolicy) rule.conflictPolicy = input.conflictPolicy.trim()
      rule.destinationNodeId = input.destinationNodeId?.trim() || undefined
      if (input.paused !== undefined) {
        rule.paused = input.paused
      }
      rule.status = unavailable ? "source_unavailable" : rule.paused ? "paused" : "active"
      rule.lastError = unavailable ? "source directory is unavailable or unmounted" : undefined
      rule.updatedAt = new Date().toISOString()
      this.backupStatusEvents.emit({ ...rule })
      return { ...rule }
    },

    pause: async (id: string) => {
      const rule = this.requireBackupRule(id)
      rule.paused = true
      rule.status = "paused"
      rule.updatedAt = new Date().toISOString()
      this.backupStatusEvents.emit({ ...rule })
      return { ...rule }
    },

    resume: async (id: string) => {
      const rule = this.requireBackupRule(id)
      rule.paused = false
      rule.status = this.unavailablePaths.has(rule.localPath) ? "source_unavailable" : "active"
      rule.updatedAt = new Date().toISOString()
      this.backupStatusEvents.emit({ ...rule })
      return { ...rule }
    },

    remove: async (id: string) => {
      const idx = this.backupRules.findIndex((b) => b.id === id.trim())
      if (idx < 0) {
        throw {
          code: "RESOURCE_NOT_FOUND",
          category: "api",
          message: `backup rule ${id} not found`,
        } satisfies BackendError
      }
      this.backupRules.splice(idx, 1)
    },

    scanNow: async (id: string) => {
      const rule = this.requireBackupRule(id)
      const now = new Date().toISOString()
      if (this.unavailablePaths.has(rule.localPath)) {
        rule.status = "source_unavailable"
        rule.lastScanAt = now
        rule.lastError = "source directory is unavailable or unmounted"
        rule.updatedAt = now
        this.backupStatusEvents.emit({ ...rule })
        return null
      }

      rule.status = "scanning"
      rule.lastScanAt = now
      rule.updatedAt = now
      this.backupStatusEvents.emit({ ...rule })

      const trID = `tr-${this.seq++}`
      const tr: TransferDTO = {
        id: trID,
        kind: "backup",
        stage: "completed",
        sourceSummary: fileBaseName(rule.localPath),
        conflictPolicy: rule.conflictPolicy,
        bytesDone: 2_097_152,
        bytesTotal: 2_097_152,
        itemsDone: 1,
        itemsSkipped: 0,
        itemsFailed: 0,
        itemsUnconfirmed: 0,
        itemsTotal: 1,
        createdAt: now,
        updatedAt: now,
        finishedAt: now,
      }
      const item: TransferItemDTO = {
        id: `${trID}-item-1`,
        transferId: trID,
        localPath: `${rule.localPath.replace(/\/+$/, "")}/IMG_BACKUP_01.JPG`,
        name: "IMG_BACKUP_01.JPG",
        mediaType: "photo",
        size: 2_097_152,
        remoteNodeId: `node-${this.seq}`,
        status: "confirmed",
        bytesDone: 2_097_152,
        updatedAt: now,
      }
      this.transferRecords.unshift(tr)
      this.itemsByTransfer.set(trID, [item])
      this.transferStageEvents.emit({ ...tr })

      rule.status = rule.paused ? "paused" : "active"
      rule.lastError = undefined
      rule.updatedAt = now
      this.backupStatusEvents.emit({ ...rule })
      return { ...tr }
    },

    setAllStopped: async (stopped: boolean) => {
      this.settingsState = {
        ...this.settingsState,
        backupsStopped: stopped,
      }
    },

    pickFolder: async () => this.pickedDirectory,
  }

  readonly downloads: Backend["downloads"] = {
    listDestinations: async () => structuredClone(this.recoveryState),
    startDownload: async (input: DownloadInput) => {
      const destDir = input.destDir.trim()
      if (!destDir) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "download destination directory is required",
        } satisfies BackendError
      }

      const items: TransferItemDTO[] = []
      const now = new Date().toISOString()
      const id = `tr-${this.seq++}`

      if (input.items && input.items.length > 0) {
        for (let i = 0; i < input.items.length; i++) {
          const spec = input.items[i]!
          const name = spec.name?.trim() || `${spec.nodeId}.jpg`
          const size = spec.expectedSize && spec.expectedSize > 0 ? spec.expectedSize : 2_097_152
          items.push({
            id: `${id}-item-${i + 1}`,
            transferId: id,
            localPath: `${destDir.replace(/\/+$/, "")}/${name}`,
            name,
            mediaType: spec.mediaType || "photo",
            size,
            md5: spec.expectedMd5,
            remoteNodeId: spec.nodeId,
            status: "confirmed",
            bytesDone: size,
            updatedAt: now,
          })
        }
      } else if (input.nodeIds && input.nodeIds.length > 0) {
        for (let i = 0; i < input.nodeIds.length; i++) {
          const nodeID = input.nodeIds[i]!.trim()
          if (!nodeID) continue
          const known = this.recoveryState.confirmedItems.find((c) => c.remoteNodeId === nodeID)
          const name = known?.name ?? `${nodeID}.jpg`
          const size = known?.size ?? 2_097_152
          items.push({
            id: `${id}-item-${i + 1}`,
            transferId: id,
            localPath: `${destDir.replace(/\/+$/, "")}/${name}`,
            name,
            mediaType: known?.mediaType ?? "photo",
            size,
            md5: known?.md5,
            remoteNodeId: nodeID,
            status: "confirmed",
            bytesDone: size,
            updatedAt: now,
          })
        }
      } else if (input.containerNodeId?.trim()) {
        const containerID = input.containerNodeId.trim()
        items.push({
          id: `${id}-item-1`,
          transferId: id,
          localPath: `${destDir.replace(/\/+$/, "")}/recovered_${containerID}.jpg`,
          name: `recovered_${containerID}.jpg`,
          mediaType: "photo",
          size: 3_145_728,
          remoteNodeId: `${containerID}-child-1`,
          status: "confirmed",
          bytesDone: 3_145_728,
          updatedAt: now,
        })
      }

      if (items.length === 0) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message: "select at least one recovery item, remote node ID, or container",
        } satisfies BackendError
      }

      const policy =
        (input.conflictPolicy ?? "").trim() || this.settingsState.defaultConflictPolicy || "skip"
      const bytesTotal = items.reduce((sum, it) => sum + it.size, 0)
      const tr: TransferDTO = {
        id,
        kind: "download",
        stage: "completed",
        sourceSummary:
          items.length === 1 ? items[0]!.name : `${items[0]!.name} +${items.length - 1} more`,
        destLocalDir: destDir,
        conflictPolicy: policy,
        bytesDone: bytesTotal,
        bytesTotal,
        itemsDone: items.length,
        itemsSkipped: 0,
        itemsFailed: 0,
        itemsUnconfirmed: 0,
        itemsTotal: items.length,
        createdAt: now,
        updatedAt: now,
        finishedAt: now,
      }

      this.transferRecords.unshift(tr)
      this.itemsByTransfer.set(id, items)
      this.transferStageEvents.emit({ ...tr })
      return { ...tr }
    },

    pickTargetDirectory: async () => this.pickedDirectory,
  }

  readonly settings: Backend["settings"] = {
    get: async () => ({ ...this.settingsState }),

    update: async (input: SettingsInput) => {
      if (input.region !== undefined) {
        const r = input.region.trim().toLowerCase()
        if (r !== "" && !VALID_REGIONS[r]) {
          throw {
            code: "CONFIG_ERROR",
            category: "config",
            message: `unsupported region "${input.region}"`,
            details: { field: "region" },
          } satisfies BackendError
        }
        this.settingsState.region = r
        this.settingsState.evidence = evidenceForRegion(r)
        this.authState.region = r
        this.authState.evidence = evidenceForRegion(r)
        this.recoveryState.region = r
        this.recoveryState.evidence = evidenceForRegion(r) || "experimental"
      }

      if (input.maxMetadataConcurrency !== undefined) {
        if (input.maxMetadataConcurrency < 1 || input.maxMetadataConcurrency > 16) {
          throw {
            code: "VALIDATION_FAILED",
            category: "validation",
            message: "maxMetadataConcurrency must be between 1 and 16",
            details: { field: "maxMetadataConcurrency" },
          } satisfies BackendError
        }
        this.settingsState.maxMetadataConcurrency = input.maxMetadataConcurrency
      }

      if (input.maxUploadConcurrency !== undefined) {
        if (input.maxUploadConcurrency < 1 || input.maxUploadConcurrency > 8) {
          throw {
            code: "VALIDATION_FAILED",
            category: "validation",
            message: "maxUploadConcurrency must be between 1 and 8",
            details: { field: "maxUploadConcurrency" },
          } satisfies BackendError
        }
        this.settingsState.maxUploadConcurrency = input.maxUploadConcurrency
      }

      if (input.defaultConflictPolicy !== undefined) {
        const pol = input.defaultConflictPolicy.trim().toLowerCase()
        if (!VALID_POLICIES[pol]) {
          throw {
            code: "VALIDATION_FAILED",
            category: "validation",
            message: `invalid conflict policy "${input.defaultConflictPolicy}"`,
            details: { field: "defaultConflictPolicy" },
          } satisfies BackendError
        }
        this.settingsState.defaultConflictPolicy = pol
      }

      if (input.launchAtLogin !== undefined) {
        this.settingsState.launchAtLogin = input.launchAtLogin
      }
      if (input.closeToTray !== undefined) {
        this.settingsState.closeToTray = input.closeToTray
      }
      if (input.notificationsEnabled !== undefined) {
        this.settingsState.notificationsEnabled = input.notificationsEnabled
      }
      if (input.backupsStopped !== undefined) {
        this.settingsState.backupsStopped = input.backupsStopped
      }

      return { ...this.settingsState }
    },

    doctor: async (): Promise<DoctorReportDTO> => {
      if (this.customDoctor) {
        return structuredClone(this.customDoctor)
      }
      if (!this.authState.configured) {
        const checks: DoctorCheckDTO[] = [
          {
            grade: 1,
            name: "vault_permissions",
            status: "skipped",
            ok: true,
            message: "credential vault not yet created",
          },
          {
            grade: 2,
            name: "credentials_and_region",
            status: "warn",
            ok: false,
            message: "No Amazon Photos credentials configured.",
            hint: "Import browser cookies in Settings -> Account to connect your account.",
          },
          {
            grade: 3,
            name: "account_root_and_usage",
            status: "skipped",
            ok: true,
            message: "Skipped while offline.",
          },
          {
            grade: 4,
            name: "search_read_probe",
            status: "skipped",
            ok: true,
            message: "Skipped while offline.",
          },
          {
            grade: 5,
            name: "write_probe",
            status: "skipped",
            ok: true,
            message: "Skipped while offline.",
          },
        ]
        return {
          healthy: false,
          unconfigured: true,
          region: this.settingsState.region,
          checks,
        }
      }

      const region = this.authState.region || this.settingsState.region || "us"
      const evidence = evidenceForRegion(region) || "verified"
      return {
        healthy: true,
        region,
        evidence,
        checks: [
          {
            grade: 1,
            name: "vault_permissions",
            status: "pass",
            ok: true,
            message: "Vault file permissions are restricted (0600).",
          },
          {
            grade: 2,
            name: "credentials_and_region",
            status: "pass",
            ok: true,
            message: `Region ${region} (${evidence}) configured with 3 required cookies.`,
          },
          {
            grade: 3,
            name: "account_root_and_usage",
            status: "pass",
            ok: true,
            message: `Root node ${this.authState.rootNodeId ?? "root-node-us"} verified.`,
          },
          {
            grade: 4,
            name: "search_read_probe",
            status: "pass",
            ok: true,
            message: "Search read probe succeeded.",
          },
          {
            grade: 5,
            name: "write_probe",
            status: "skipped",
            ok: true,
            message:
              "Write probe skipped (enable opt-in write probe with confirmation to test remote folder creation and trash).",
          },
        ],
      }
    },

    doctorWithWriteProbe: async (confirm: boolean): Promise<DoctorReportDTO> => {
      if (!confirm) {
        throw {
          code: "VALIDATION_FAILED",
          category: "validation",
          message:
            "write probe requires explicit confirmation before creating and trashing a temporary remote folder",
          details: { field: "confirm" },
        } satisfies BackendError
      }
      const base = await this.settings.doctor()
      if (base.unconfigured) {
        return base
      }
      const checks = base.checks.filter((c) => c.grade !== 5)
      checks.push({
        grade: 5,
        name: "write_probe",
        status: "pass",
        ok: true,
        message: "Created and trashed temporary probe folder probe-node-1.",
      })
      return {
        ...base,
        checks,
      }
    },
  }

  readonly events: Backend["events"] = {
    onTransferStage: (cb) => this.transferStageEvents.subscribe(cb),
    onTransferProgress: (cb) => this.transferProgressEvents.subscribe(cb),
    onBackupStatus: (cb) => this.backupStatusEvents.subscribe(cb),
    onAuthChanged: (cb) => this.authChangedEvents.subscribe(cb),
    onFilesDropped: (cb) => this.filesDroppedEvents.subscribe(cb),
    onNotification: (cb) => this.notificationEvents.subscribe(cb),
  }

  emitFilesDropped(paths: string[], x = 120, y = 120): void {
    this.filesDroppedEvents.emit({ paths, x, y })
  }

  emitTransferStage(tr: TransferDTO): void {
    const idx = this.transferRecords.findIndex((existing) => existing.id === tr.id)
    if (idx >= 0) {
      this.transferRecords[idx] = { ...tr }
    } else {
      this.transferRecords.unshift({ ...tr })
    }
    this.transferStageEvents.emit({ ...tr })
  }

  emitTransferProgress(ev: TransferProgressEvent): void {
    const idx = this.transferRecords.findIndex((existing) => existing.id === ev.transfer.id)
    if (idx >= 0) {
      this.transferRecords[idx] = { ...ev.transfer }
    } else {
      this.transferRecords.unshift({ ...ev.transfer })
    }
    const items = this.itemsByTransfer.get(ev.transfer.id) ?? []
    const itemIdx = items.findIndex((it) => it.id === ev.item.id)
    if (itemIdx >= 0) {
      items[itemIdx] = { ...ev.item }
    } else {
      items.push({ ...ev.item })
    }
    this.itemsByTransfer.set(ev.transfer.id, items)
    this.transferProgressEvents.emit({
      transfer: { ...ev.transfer },
      item: { ...ev.item },
    })
  }

  emitBackupStatus(rule: BackupRuleDTO): void {
    const idx = this.backupRules.findIndex((existing) => existing.id === rule.id)
    if (idx >= 0) {
      this.backupRules[idx] = { ...rule }
    } else {
      this.backupRules.push({ ...rule })
    }
    this.backupStatusEvents.emit({ ...rule })
  }

  emitNotification(ev: NotificationEvent): void {
    this.notificationEvents.emit({ ...ev })
  }

  setBackupPathUnavailableForTest(localPath: string, unavailable: boolean): void {
    if (unavailable) {
      this.unavailablePaths.add(localPath)
    } else {
      this.unavailablePaths.delete(localPath)
    }
    for (const rule of this.backupRules) {
      if (rule.localPath === localPath) {
        rule.status = unavailable ? "source_unavailable" : rule.paused ? "paused" : "active"
        rule.lastError = unavailable ? "source directory is unavailable or unmounted" : undefined
        this.backupStatusEvents.emit({ ...rule })
      }
    }
  }

  completeTransferForTest(
    transferID: string,
    stage: "completed" | "partial" | "failed" | "unconfirmed" = "completed",
  ): TransferDTO {
    const tr = this.requireTransfer(transferID)
    const now = new Date().toISOString()
    tr.stage = stage
    tr.updatedAt = now
    tr.finishedAt = now
    const items = this.itemsByTransfer.get(tr.id) ?? []
    if (stage === "completed") {
      tr.bytesDone = tr.bytesTotal
      tr.itemsDone = tr.itemsTotal
      for (const it of items) {
        it.status = "confirmed"
        it.bytesDone = it.size
        it.remoteNodeId = it.remoteNodeId ?? `node-${it.id}`
        it.updatedAt = now
      }
    } else if (stage === "unconfirmed") {
      tr.itemsUnconfirmed = Math.max(1, tr.itemsTotal - tr.itemsDone)
      for (const it of items) {
        if (it.status !== "confirmed") {
          it.status = "unconfirmed"
          it.errorMessage = "upload commit unconfirmed; awaiting startup reconciliation"
          it.updatedAt = now
        }
      }
    } else if (stage === "failed") {
      tr.itemsFailed = Math.max(1, tr.itemsTotal - tr.itemsDone)
      tr.errorCode = "NETWORK_ERROR"
      tr.errorMessage = "upload connection interrupted"
      for (const it of items) {
        if (it.status !== "confirmed") {
          it.status = "failed"
          it.errorMessage = "upload connection interrupted"
          it.updatedAt = now
        }
      }
    }
    this.transferStageEvents.emit({ ...tr })
    return { ...tr }
  }

  private expandCandidatePaths(paths: string[]): LocalFileFixture[] {
    const out: LocalFileFixture[] = []
    for (const p of paths) {
      const exact = this.localFilesByPath.get(p)
      if (exact) {
        out.push(exact)
        continue
      }
      const prefix = p.endsWith("/") ? p : `${p}/`
      const children: LocalFileFixture[] = []
      for (const [k, v] of this.localFilesByPath.entries()) {
        if (k.startsWith(prefix)) {
          children.push(v)
        }
      }
      if (children.length > 0) {
        out.push(...children)
        continue
      }
      const mediaType = classifyPathMediaType(p)
      const base = fileBaseName(p)
      if (!base.includes(".")) {
        out.push(
          {
            path: `${p.replace(/\/+$/, "")}/IMG_0001.JPG`,
            name: "IMG_0001.JPG",
            mediaType: "photo",
            size: 2_621_440,
          },
          {
            path: `${p.replace(/\/+$/, "")}/MOV_0002.MP4`,
            name: "MOV_0002.MP4",
            mediaType: "video",
            size: 8_388_608,
          },
        )
      } else {
        out.push({
          path: p,
          name: base,
          mediaType,
          size: mediaType === "video" ? 8_388_608 : 2_621_440,
        })
      }
    }
    return out
  }

  private requireTransfer(id: string): TransferDTO {
    const found = this.transferRecords.find((t) => t.id === id.trim())
    if (!found) {
      throw {
        code: "RESOURCE_NOT_FOUND",
        category: "api",
        message: `transfer ${id} not found`,
      } satisfies BackendError
    }
    return found
  }

  private requireBackupRule(id: string): BackupRuleDTO {
    const found = this.backupRules.find((b) => b.id === id.trim())
    if (!found) {
      throw {
        code: "RESOURCE_NOT_FOUND",
        category: "api",
        message: `backup rule ${id} not found`,
      } satisfies BackendError
    }
    return found
  }
}

export function memoryBackend(opts: MemoryBackendOptions = {}): MemoryBackend {
  return new MemoryBackend(opts)
}

export function failingBackend(err: BackendError): Backend {
  const fail = (): Promise<never> => Promise.reject(err)
  return {
    auth: {
      status: fail,
      importCookies: fail,
      importCookieFile: fail,
      pickAndImportCookieFile: fail,
      logout: fail,
      usage: fail,
    },
    transfers: {
      probeDestination: fail,
      planUpload: fail,
      startUpload: fail,
      list: fail,
      getItems: fail,
      pause: fail,
      resume: fail,
      cancel: fail,
      retry: fail,
      clearHistory: fail,
      pickFiles: fail,
      pickDirectory: fail,
    },
    backups: {
      list: fail,
      add: fail,
      update: fail,
      pause: fail,
      resume: fail,
      remove: fail,
      scanNow: fail,
      setAllStopped: fail,
      pickFolder: fail,
    },
    downloads: {
      listDestinations: fail,
      startDownload: fail,
      pickTargetDirectory: fail,
    },
    settings: {
      get: fail,
      update: fail,
      doctor: fail,
      doctorWithWriteProbe: fail,
    },
    events: {
      onTransferStage: () => () => {},
      onTransferProgress: () => () => {},
      onBackupStatus: () => () => {},
      onAuthChanged: () => () => {},
      onFilesDropped: () => () => {},
      onNotification: () => () => {},
    },
  }
}
