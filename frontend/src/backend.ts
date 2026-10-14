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
    importCookies(input: ImportCookiesInput): Promise<AuthStatus>
    importCookieFile(path: string, region: string): Promise<AuthStatus>
    pickAndImportCookieFile(region: string): Promise<AuthStatus>
    logout(): Promise<AuthStatus>
    usage(): Promise<UsageSummary>
  }
  transfers: {
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
    listSources(): Promise<RecoverySourcesDTO>
    listDestinations(): Promise<RecoverySourcesDTO>
    startDownload(input: DownloadInput): Promise<TransferDTO>
    pickTargetDirectory(): Promise<string>
  }
  settings: {
    get(): Promise<SettingsDTO>
    update(input: SettingsInput): Promise<SettingsDTO>
    doctor(): Promise<DoctorReportDTO>
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
