import type {
  AuthStatus,
  BackupRuleDTO,
  BackupRuleInput,
  DestinationCapabilityDTO,
  DoctorCheckDTO,
  DoctorReportDTO,
  DownloadInput,
  DownloadItemInput,
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
  UploadPlanDTO,
  UploadPlanInput,
  UsageBucketDTO,
  UsageCategoryDTO,
  UsageSummary,
} from "./mygo"

export type {
  AuthStatus,
  BackupRuleDTO,
  BackupRuleInput,
  DestinationCapabilityDTO,
  DoctorCheckDTO,
  DoctorReportDTO,
  DownloadInput,
  DownloadItemInput,
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
  UploadPlanDTO,
  UploadPlanInput,
  UsageBucketDTO,
  UsageCategoryDTO,
  UsageSummary,
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let val = bytes
  let idx = 0
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024
    idx++
  }
  return `${val >= 10 || idx === 0 ? val.toFixed(idx === 0 ? 0 : 1) : val.toFixed(2)} ${units[idx]}`
}

export function errorMessageFrom(err: unknown): string {
  if (
    typeof err === "object" &&
    err !== null &&
    "message" in err &&
    typeof err.message === "string"
  ) {
    return err.message
  }
  if (err instanceof Error) return err.message
  return String(err)
}

export interface BackendError {
  code: string
  category: string
  message: string
  retryAfterMs?: number
  details?: Record<string, unknown>
}

export function isTerminalStage(stage: string): boolean {
  return (
    stage === "completed" ||
    stage === "partial" ||
    stage === "failed" ||
    stage === "cancelled" ||
    stage === "unconfirmed"
  )
}

export interface Backend {
  auth: {
    status(): Promise<AuthStatus>
    login(region: string): Promise<AuthStatus>
    cancelLogin(): Promise<void>
    importCookies(input: ImportCookiesInput): Promise<AuthStatus>
    importCookieFile(path: string, region: string): Promise<AuthStatus>
    pickAndImportCookieFile(region: string): Promise<AuthStatus>
    logout(): Promise<AuthStatus>
    usage(): Promise<UsageSummary>
  }
  transfers: {
    probeDestination(id: string): Promise<DestinationCapabilityDTO>
    planUpload(input: UploadPlanInput): Promise<UploadPlanDTO>
    startUpload(input: StartUploadInput): Promise<TransferDTO>
    list(): Promise<TransferListDTO>
    getItems(transferID: string): Promise<TransferItemDTO[]>
    pause(transferID: string): Promise<TransferDTO>
    resume(transferID: string): Promise<TransferDTO>
    cancel(transferID: string): Promise<TransferDTO>
    retry(transferID: string): Promise<TransferDTO>
    clearHistory(): Promise<void>
    pickFiles(): Promise<string[]>
    pickDirectory(): Promise<string>
  }
  backups: {
    list(): Promise<BackupRuleDTO[]>
    add(input: BackupRuleInput): Promise<BackupRuleDTO>
    update(id: string, input: BackupRuleInput): Promise<BackupRuleDTO>
    pause(id: string): Promise<BackupRuleDTO>
    resume(id: string): Promise<BackupRuleDTO>
    remove(id: string): Promise<void>
    scanNow(id: string): Promise<TransferDTO | null>
    setAllStopped(stopped: boolean): Promise<void>
    pickFolder(): Promise<string>
  }
  downloads: {
    listDestinations(): Promise<RecoverySourcesDTO>
    startDownload(input: DownloadInput): Promise<TransferDTO>
    pickTargetDirectory(): Promise<string>
  }
  settings: {
    get(): Promise<SettingsDTO>
    update(input: SettingsInput): Promise<SettingsDTO>
    doctor(): Promise<DoctorReportDTO>
    doctorWithWriteProbe(confirm: boolean): Promise<DoctorReportDTO>
  }
  events: {
    onTransferStage(cb: (t: TransferDTO) => void): () => void
    onTransferProgress(cb: (e: TransferProgressEvent) => void): () => void
    onBackupStatus(cb: (r: BackupRuleDTO) => void): () => void
    onAuthChanged(cb: (s: AuthStatus) => void): () => void
    onFilesDropped(cb: (e: FilesDroppedEvent) => void): () => void
    onNotification(cb: (e: NotificationEvent) => void): () => void
  }
}
