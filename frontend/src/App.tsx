import { useCallback, useEffect, useState } from "react"

import type {
  AuthStatus,
  Backend,
  BackupRuleDTO,
  DoctorReportDTO,
  RecoverySourcesDTO,
  SettingsDTO,
  TransferDTO,
  TransferItemDTO,
  TransferListDTO,
  UploadPlanDTO,
  UsageSummary,
} from "@/backend"
import { isTerminalStage } from "@/backend"

export interface AppProps {
  backend: Backend
}

type ViewTab = "transfers" | "backups" | "downloads" | "settings"

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

function errorMessageFrom(err: unknown): string {
  if (typeof err === "object" && err !== null && "message" in err && typeof err.message === "string") {
    return err.message
  }
  if (err instanceof Error) return err.message
  return String(err)
}

export function App({ backend }: AppProps) {
  const [activeView, setActiveView] = useState<ViewTab>("transfers")
  const [auth, setAuth] = useState<AuthStatus | null>(null)
  const [usage, setUsage] = useState<UsageSummary | null>(null)
  const [globalError, setGlobalError] = useState<string | null>(null)

  // Transfers / Upload state
  const [transferList, setTransferList] = useState<TransferListDTO>({ active: [], history: [] })
  const [expandedTransfers, setExpandedTransfers] = useState<Record<string, boolean>>({})
  const [transferItemsMap, setTransferItemsMap] = useState<Record<string, TransferItemDTO[]>>({})
  const [isDragging, setIsDragging] = useState(false)

  // Preflight upload plan state
  const [pendingPaths, setPendingPaths] = useState<string[] | null>(null)
  const [planIncludeVideos, setPlanIncludeVideos] = useState<boolean>(true)
  const [planConflictPolicy, setPlanConflictPolicy] = useState<string>("skip")
  const [planDestNodeId, setPlanDestNodeId] = useState<string>("")
  const [uploadPlan, setUploadPlan] = useState<UploadPlanDTO | null>(null)
  const [planError, setPlanError] = useState<string | null>(null)

  // Backups state
  const [backupRules, setBackupRules] = useState<BackupRuleDTO[]>([])
  const [settingsState, setSettingsState] = useState<SettingsDTO | null>(null)
  const [backupModalOpen, setBackupModalOpen] = useState(false)
  const [editingBackupRule, setEditingBackupRule] = useState<BackupRuleDTO | null>(null)
  const [backupLocalPath, setBackupLocalPath] = useState("")
  const [backupIncludeVideos, setBackupIncludeVideos] = useState(true)
  const [backupScheduleMode, setBackupScheduleMode] = useState("immediate")
  const [backupIntervalMinutes, setBackupIntervalMinutes] = useState(30)
  const [backupConflictPolicy, setBackupConflictPolicy] = useState("skip")
  const [backupFormError, setBackupFormError] = useState<string | null>(null)

  // Downloads (Recovery) state
  const [recoverySources, setRecoverySources] = useState<RecoverySourcesDTO | null>(null)
  const [recoveryMode, setRecoveryMode] = useState<"history" | "container" | "nodes">("history")
  const [selectedHistoryNodes, setSelectedHistoryNodes] = useState<Record<string, boolean>>({})
  const [selectedContainerId, setSelectedContainerId] = useState<string>("")
  const [explicitNodeIds, setExplicitNodeIds] = useState<string>("")
  const [downloadDestDir, setDownloadDestDir] = useState<string>("")
  const [downloadConflictPolicy, setDownloadConflictPolicy] = useState<string>("skip")
  const [downloadError, setDownloadError] = useState<string | null>(null)
  const [latestDownloadTransfer, setLatestDownloadTransfer] = useState<TransferDTO | null>(null)
  const [latestDownloadItems, setLatestDownloadItems] = useState<TransferItemDTO[]>([])

  // Settings & Diagnostics state
  const [cookieInput, setCookieInput] = useState<string>("")
  const [selectedRegion, setSelectedRegion] = useState<string>("")
  const [authFormError, setAuthFormError] = useState<string | null>(null)
  const [metadataConcurrency, setMetadataConcurrency] = useState<number>(4)
  const [uploadConcurrency, setUploadConcurrency] = useState<number>(2)
  const [defaultConflictPolicy, setDefaultConflictPolicy] = useState<string>("skip")
  const [closeToTray, setCloseToTray] = useState<boolean>(true)
  const [launchAtLogin, setLaunchAtLogin] = useState<boolean>(false)
  const [notificationsEnabled, setNotificationsEnabled] = useState<boolean>(true)
  const [settingsSavedMsg, setSettingsSavedMsg] = useState<string | null>(null)
  const [doctorReport, setDoctorReport] = useState<DoctorReportDTO | null>(null)

  const refreshTransfers = useCallback(async () => {
    try {
      const list = await backend.transfers.list()
      setTransferList(list)
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }, [backend])

  const openPreflightForPaths = useCallback((paths: string[]) => {
    const cleaned = paths.map((p) => p.trim()).filter(Boolean)
    if (cleaned.length === 0) return
    setPendingPaths(cleaned)
    setPlanIncludeVideos(true)
    setPlanConflictPolicy("skip")
    setPlanDestNodeId("")
    setPlanError(null)
  }, [])

  useEffect(() => {
    if (!pendingPaths || pendingPaths.length === 0) {
      setUploadPlan(null)
      return
    }
    let live = true
    backend.transfers
      .planUpload({
        paths: pendingPaths,
        includeVideos: planIncludeVideos,
        conflictPolicy: planConflictPolicy,
        destinationNodeId: planDestNodeId || undefined,
        includePreviews: true,
      })
      .then(
        (plan) => {
          if (live) {
            setUploadPlan(plan)
            setPlanError(null)
          }
        },
        (err) => {
          if (live) {
            setPlanError(errorMessageFrom(err))
          }
        },
      )
    return () => {
      live = false
    }
  }, [backend, pendingPaths, planIncludeVideos, planConflictPolicy, planDestNodeId])

  useEffect(() => {
    let mounted = true
    const init = async () => {
      try {
        const status = await backend.auth.status()
        if (!mounted) return
        setAuth(status)
        setSelectedRegion(status.region || "")
        if (status.authenticated) {
          try {
            const u = await backend.auth.usage()
            if (mounted) setUsage(u)
          } catch {
            if (mounted) setUsage(null)
          }
        } else {
          setUsage(null)
        }
      } catch (err) {
        if (!mounted) return
        setAuth({
          configured: false,
          authenticated: false,
          region: "",
          errorMessage: errorMessageFrom(err),
        })
      }

      try {
        const list = await backend.transfers.list()
        if (mounted) setTransferList(list)
      } catch {
        // ignore initial load error if unconfigured
      }
      try {
        const [rules, cfg] = await Promise.all([
          backend.backups.list(),
          backend.settings.get(),
        ])
        if (mounted) {
          setBackupRules(rules)
          setSettingsState(cfg)
          if (cfg.region) setSelectedRegion(cfg.region)
          setMetadataConcurrency(cfg.maxMetadataConcurrency)
          setUploadConcurrency(cfg.maxUploadConcurrency)
          setDefaultConflictPolicy(cfg.defaultConflictPolicy)
          setCloseToTray(cfg.closeToTray)
          setLaunchAtLogin(cfg.launchAtLogin)
          setNotificationsEnabled(cfg.notificationsEnabled)
        }
      } catch {
        // ignore initial load error
      }
    }
    void init()

    const offAuth = backend.events.onAuthChanged((st) => {
      setAuth(st)
      if (st.region) setSelectedRegion(st.region)
      if (st.authenticated) {
        void backend.auth
          .usage()
          .then((u) => {
            if (mounted) setUsage(u)
          })
          .catch(() => {
            if (mounted) setUsage(null)
          })
      } else {
        setUsage(null)
      }
    })

    const upsertTransferState = (tr: TransferDTO) => {
      setTransferList((prev) => {
        const active = prev.active.filter((t) => t.id !== tr.id)
        const history = prev.history.filter((t) => t.id !== tr.id)
        if (isTerminalStage(tr.stage)) {
          return { active, history: [tr, ...history] }
        }
        return { active: [tr, ...active], history }
      })
    }

    const offStage = backend.events.onTransferStage((tr) => {
      upsertTransferState(tr)
    })

    const offProgress = backend.events.onTransferProgress((ev) => {
      upsertTransferState(ev.transfer)
      setTransferItemsMap((prev) => {
        const existing = prev[ev.transfer.id]
        if (!existing) return prev
        const idx = existing.findIndex((i) => i.id === ev.item.id)
        if (idx >= 0) {
          const next = [...existing]
          next[idx] = ev.item
          return { ...prev, [ev.transfer.id]: next }
        }
        return { ...prev, [ev.transfer.id]: [...existing, ev.item] }
      })
    })
    const offBackup = backend.events.onBackupStatus((rule) => {
      setBackupRules((prev) => {
        const idx = prev.findIndex((b) => b.id === rule.id)
        if (idx >= 0) {
          const next = [...prev]
          next[idx] = rule
          return next
        }
        return [...prev, rule]
      })
    })

    return () => {
      mounted = false
      offAuth()
      offStage()
      offProgress()
      offBackup()
    }
  }, [backend])

  useEffect(() => {
    const offDrop = backend.events.onFilesDropped((ev) => {
      if (ev.paths && ev.paths.length > 0) {
        setActiveView("transfers")
        void openPreflightForPaths(ev.paths)
      }
    })
    return () => offDrop()
  }, [backend, openPreflightForPaths])

  const handleSelectFiles = async () => {
    try {
      const files = await backend.transfers.pickFiles()
      if (files && files.length > 0) {
        await openPreflightForPaths(files)
      }
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }

  const handleSelectFolder = async () => {
    try {
      const dir = await backend.transfers.pickDirectory()
      if (dir && dir.trim()) {
        await openPreflightForPaths([dir.trim()])
      }
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }

  const handleConfirmUpload = async () => {
    if (!pendingPaths || pendingPaths.length === 0) return
    try {
      await backend.transfers.startUpload({
        paths: pendingPaths,
        includeVideos: planIncludeVideos,
        conflictPolicy: planConflictPolicy,
        destinationNodeId: planDestNodeId.trim() || undefined,
      })
      setPendingPaths(null)
      setUploadPlan(null)
      setPlanError(null)
      await refreshTransfers()
    } catch (err) {
      setPlanError(errorMessageFrom(err))
    }
  }

  const handleToggleDetails = async (transferId: string) => {
    const isExpanded = !!expandedTransfers[transferId]
    if (isExpanded) {
      setExpandedTransfers((prev) => ({ ...prev, [transferId]: false }))
      return
    }
    try {
      const items = await backend.transfers.getItems(transferId)
      setTransferItemsMap((prev) => ({ ...prev, [transferId]: items }))
      setExpandedTransfers((prev) => ({ ...prev, [transferId]: true }))
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }

  const openAddBackupModal = () => {
    setEditingBackupRule(null)
    setBackupLocalPath("")
    setBackupIncludeVideos(true)
    setBackupScheduleMode("immediate")
    setBackupIntervalMinutes(30)
    setBackupConflictPolicy(settingsState?.defaultConflictPolicy || "skip")
    setBackupFormError(null)
    setBackupModalOpen(true)
  }

  const openEditBackupModal = (rule: BackupRuleDTO) => {
    setEditingBackupRule(rule)
    setBackupLocalPath(rule.localPath)
    setBackupIncludeVideos(rule.includeVideos)
    setBackupScheduleMode(rule.scheduleMode || "immediate")
    setBackupIntervalMinutes(rule.intervalMinutes || 30)
    setBackupConflictPolicy(rule.conflictPolicy || "skip")
    setBackupFormError(null)
    setBackupModalOpen(true)
  }

  const handlePickBackupFolder = async () => {
    try {
      const folder = await backend.backups.pickFolder()
      if (folder && folder.trim()) {
        setBackupLocalPath(folder.trim())
      }
    } catch (err) {
      setBackupFormError(errorMessageFrom(err))
    }
  }

  const handleSaveBackupRule = async () => {
    setBackupFormError(null)
    try {
      const input = {
        localPath: backupLocalPath.trim(),
        includeVideos: backupIncludeVideos,
        scheduleMode: backupScheduleMode,
        intervalMinutes: backupIntervalMinutes,
        conflictPolicy: backupConflictPolicy,
      }
      if (editingBackupRule) {
        await backend.backups.update(editingBackupRule.id, input)
      } else {
        await backend.backups.add(input)
      }
      const rules = await backend.backups.list()
      setBackupRules(rules)
      setBackupModalOpen(false)
      setEditingBackupRule(null)
    } catch (err) {
      setBackupFormError(errorMessageFrom(err))
    }
  }

  const handleToggleAllBackupsStopped = async () => {
    const nextStopped = !(settingsState?.backupsStopped ?? false)
    try {
      await backend.backups.setAllStopped(nextStopped)
      const cfg = await backend.settings.get()
      setSettingsState(cfg)
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }

  useEffect(() => {
    if (activeView !== "downloads") return
    let live = true
    backend.downloads
      .listSources()
      .then((src) => {
        if (live) setRecoverySources(src)
      })
      .catch((err) => {
        if (live) setDownloadError(errorMessageFrom(err))
      })
    return () => {
      live = false
    }
  }, [backend, activeView])

  const handlePickDownloadDir = async () => {
    try {
      const dir = await backend.downloads.pickTargetDirectory()
      if (dir && dir.trim()) {
        setDownloadDestDir(dir.trim())
      }
    } catch (err) {
      setDownloadError(errorMessageFrom(err))
    }
  }

  const handleStartRecoveryDownload = async () => {
    setDownloadError(null)
    try {
      let tr: TransferDTO
      if (recoveryMode === "history") {
        const confirmed = recoverySources?.confirmedItems ?? []
        const chosen = confirmed.filter(
          (it) => it.remoteNodeId && selectedHistoryNodes[it.remoteNodeId],
        )
        tr = await backend.downloads.startDownload({
          items: chosen.map((it) => ({
            nodeId: it.remoteNodeId!,
            name: it.name,
            expectedSize: it.size,
            expectedMd5: it.md5,
            mediaType: it.mediaType,
          })),
          destDir: downloadDestDir.trim(),
        })
      } else if (recoveryMode === "container") {
        tr = await backend.downloads.startDownload({
          containerNodeId: selectedContainerId.trim(),
          destDir: downloadDestDir.trim(),
        })
      } else {
        const ids = explicitNodeIds
          .split(/[\s,]+/)
          .map((s) => s.trim())
          .filter(Boolean)
        tr = await backend.downloads.startDownload({
          nodeIds: ids,
          destDir: downloadDestDir.trim(),
        })
      }

      setLatestDownloadTransfer(tr)
      const items = await backend.transfers.getItems(tr.id)
      setLatestDownloadItems(items)
      await refreshTransfers()
    } catch (err) {
      setDownloadError(errorMessageFrom(err))
    }
  }

  const handleImportPastedCookies = async () => {
    setAuthFormError(null)
    try {
      const st = await backend.auth.importCookies({
        rawCookies: cookieInput,
        region: selectedRegion,
      })
      setAuth(st)
      if (st.region) setSelectedRegion(st.region)
      setCookieInput("")
    } catch (err) {
      setAuthFormError(errorMessageFrom(err))
    }
  }

  const handlePickAndImportCookieFile = async () => {
    setAuthFormError(null)
    try {
      const st = await backend.auth.pickAndImportCookieFile(selectedRegion)
      setAuth(st)
      if (st.region) setSelectedRegion(st.region)
    } catch (err) {
      setAuthFormError(errorMessageFrom(err))
    }
  }

  const handleLogout = async () => {
    setAuthFormError(null)
    try {
      const st = await backend.auth.logout()
      setAuth(st)
      setUsage(null)
    } catch (err) {
      setAuthFormError(errorMessageFrom(err))
    }
  }

  const handleSaveSettings = async () => {
    setSettingsSavedMsg(null)
    setGlobalError(null)
    try {
      const updated = await backend.settings.update({
        region: selectedRegion || undefined,
        maxMetadataConcurrency: metadataConcurrency,
        maxUploadConcurrency: uploadConcurrency,
        defaultConflictPolicy,
        closeToTray,
        launchAtLogin,
        notificationsEnabled,
      })
      setSettingsState(updated)
      setSettingsSavedMsg("Settings saved.")
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }

  const handleRunDoctor = async () => {
    setGlobalError(null)
    try {
      const report = await backend.settings.doctor()
      setDoctorReport(report)
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }

  const renderTransferRow = (tr: TransferDTO) => {
    const pct =
      tr.bytesTotal > 0 ? Math.min(100, Math.round((tr.bytesDone / tr.bytesTotal) * 100)) : 0
    const isExpanded = !!expandedTransfers[tr.id]
    const items = transferItemsMap[tr.id] ?? []

    return (
      <div
        key={tr.id}
        data-testid={`transfer-card-${tr.id}`}
        className="rounded-lg border border-slate-800 bg-slate-900/50 p-4 space-y-3"
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span className="font-medium text-sm text-slate-100">{tr.sourceSummary}</span>
            <span className="px-2 py-0.5 rounded bg-slate-800 text-xs uppercase font-mono text-slate-300">
              {tr.kind}
            </span>
            <span
              className={`px-2 py-0.5 rounded text-xs font-medium ${
                tr.stage === "completed"
                  ? "bg-emerald-500/20 text-emerald-300"
                  : tr.stage === "running" || tr.stage === "queued"
                    ? "bg-sky-500/20 text-sky-300"
                    : tr.stage === "paused"
                      ? "bg-amber-500/20 text-amber-300"
                      : tr.stage === "unconfirmed"
                        ? "bg-purple-500/20 text-purple-300"
                        : "bg-rose-500/20 text-rose-300"
              }`}
            >
              {tr.stage}
            </span>
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            {(tr.stage === "running" || tr.stage === "queued") && (
              <button
                type="button"
                onClick={() => void backend.transfers.pause(tr.id)}
                className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium"
              >
                Pause
              </button>
            )}
            {tr.stage === "paused" && (
              <button
                type="button"
                onClick={() => void backend.transfers.resume(tr.id)}
                className="px-2.5 py-1 rounded bg-emerald-600/30 hover:bg-emerald-600/40 text-emerald-200 text-xs font-medium"
              >
                Resume
              </button>
            )}
            {!isTerminalStage(tr.stage) && (
              <button
                type="button"
                onClick={() => void backend.transfers.cancel(tr.id)}
                className="px-2.5 py-1 rounded bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 text-xs font-medium"
              >
                Cancel
              </button>
            )}
            {isTerminalStage(tr.stage) && tr.stage !== "completed" && (
              <button
                type="button"
                onClick={() => void backend.transfers.retry(tr.id)}
                className="px-2.5 py-1 rounded bg-amber-600/25 hover:bg-amber-600/35 text-amber-200 text-xs font-medium"
              >
                Retry
              </button>
            )}
            <button
              type="button"
              onClick={() => void handleToggleDetails(tr.id)}
              className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300"
            >
              Details
            </button>
          </div>
        </div>

        {/* Progress Bar */}
        <div className="space-y-1">
          <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
            <div
              className="h-full bg-emerald-500 transition-all"
              style={{ width: `${pct}%` }}
            />
          </div>
          <div className="flex flex-wrap items-center justify-between text-xs text-slate-400 gap-2">
            <span>
              {formatBytes(tr.bytesDone)} / {formatBytes(tr.bytesTotal)} ({pct}%)
            </span>
            <div className="flex flex-wrap items-center gap-3">
              <span className="text-emerald-400">{tr.itemsDone} confirmed</span>
              <span>{tr.itemsSkipped} skipped</span>
              {tr.itemsFailed > 0 && <span className="text-rose-400">{tr.itemsFailed} failed</span>}
              {tr.itemsUnconfirmed > 0 && (
                <span className="text-purple-300">{tr.itemsUnconfirmed} unconfirmed</span>
              )}
              <span>Total: {tr.itemsTotal}</span>
            </div>
          </div>
        </div>

        {/* Error / Unconfirmed Banner */}
        {(tr.errorMessage || tr.stage === "unconfirmed") && (
          <div className="rounded bg-rose-950/50 border border-rose-800/60 px-3 py-2 text-xs text-rose-200">
            {tr.errorMessage ||
              "Upload commit unconfirmed: HTTP response did not return a verified node ID. Startup reconciliation will verify remote state before retrying."}
          </div>
        )}

        {/* Expanded Per-Item Details */}
        {isExpanded && (
          <div className="border-t border-slate-800 pt-2 mt-2 space-y-1.5">
            {items.length === 0 ? (
              <p className="text-xs text-slate-500">No item records.</p>
            ) : (
              items.map((it) => (
                <div
                  key={it.id}
                  className="flex flex-wrap items-center justify-between text-xs py-1 px-2 rounded bg-slate-950/60"
                >
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-slate-200">{it.name}</span>
                    <span className="text-slate-500">({formatBytes(it.size)})</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="px-1.5 py-0.5 rounded bg-slate-800 text-[11px] text-slate-300">
                      {it.status}
                    </span>
                    {it.remoteNodeId && (
                      <span className="font-mono text-[11px] text-emerald-400">
                        {it.remoteNodeId}
                      </span>
                    )}
                    {it.errorMessage && (
                      <span className="text-rose-400">{it.errorMessage}</span>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        )}
      </div>
    )
  }

  const navItems: { id: ViewTab; label: string }[] = [
    { id: "transfers", label: "Transfers / Upload" },
    { id: "backups", label: "Backups" },
    { id: "downloads", label: "Downloads" },
    { id: "settings", label: "Settings" },
  ]

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex">
      <aside className="w-60 border-r border-slate-800 bg-slate-900/60 p-4 flex flex-col justify-between shrink-0">
        <div>
          <div className="mb-6 px-2">
            <h1 className="text-lg font-bold tracking-tight text-emerald-400">Rainforest</h1>
            <p className="text-xs text-slate-400">Amazon Photos Transfer &amp; Backup</p>
          </div>
          <nav aria-label="Main navigation" className="space-y-1">
            {navItems.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setActiveView(item.id)}
                aria-current={activeView === item.id ? "page" : undefined}
                className={`w-full text-left px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                  activeView === item.id
                    ? "bg-emerald-500/15 text-emerald-300 border border-emerald-500/30"
                    : "text-slate-300 hover:bg-slate-800/70 hover:text-slate-100"
                }`}
              >
                {item.label}
              </button>
            ))}
          </nav>
        </div>
        <div className="px-2 pt-4 border-t border-slate-800 text-[11px] text-slate-500">
          Unofficial client · AGPL-3.0-only
        </div>
      </aside>

      <main className="flex-1 p-6 overflow-y-auto space-y-6">
        {globalError && (
          <div
            role="alert"
            className="rounded-lg border border-rose-500/40 bg-rose-950/60 p-3 flex items-center justify-between text-xs text-rose-200"
          >
            <span>{globalError}</span>
            <button
              type="button"
              onClick={() => setGlobalError(null)}
              className="px-2 py-0.5 rounded bg-rose-900/50 hover:bg-rose-900 text-rose-100"
            >
              Dismiss
            </button>
          </div>
        )}

        {activeView === "transfers" && (
          <div className="space-y-6">
            {/* Compact Account & Storage Banner */}
            <section
              data-testid="account-banner"
              className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 flex flex-wrap items-center justify-between gap-4"
            >
              {auth?.authenticated ? (
                <div className="flex flex-wrap items-center gap-3 text-sm">
                  <span className="font-medium text-emerald-400">Connected</span>
                  <span className="px-2 py-0.5 rounded bg-slate-800 text-xs font-mono uppercase">
                    {(auth.region || "us").toUpperCase()}
                  </span>
                  <span
                    className={`px-2 py-0.5 rounded text-xs font-medium ${
                      auth.evidence === "verified"
                        ? "bg-emerald-500/20 text-emerald-300"
                        : "bg-amber-500/20 text-amber-300"
                    }`}
                  >
                    {auth.evidence || "experimental"}
                  </span>
                  {usage && (
                    <span className="text-slate-300 text-xs">
                      {usage.photo.total.count.toLocaleString("en-US")} photos (
                      {formatBytes(usage.photo.total.bytes)}) ·{" "}
                      {usage.video.total.count.toLocaleString("en-US")} videos (
                      {formatBytes(usage.video.total.bytes)}) · Total{" "}
                      {formatBytes(usage.totalBytes)}
                    </span>
                  )}
                </div>
              ) : (
                <div className="flex flex-wrap items-center justify-between w-full gap-3">
                  <div className="text-sm text-amber-300">
                    {auth?.errorMessage ||
                      "Not signed in. Import your Amazon Photos browser session cookies in Settings to start transfers."}
                  </div>
                  <button
                    type="button"
                    onClick={() => setActiveView("settings")}
                    className="px-3 py-1.5 rounded bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 text-xs font-medium border border-amber-500/30"
                  >
                    Import Cookies in Settings
                  </button>
                </div>
              )}
            </section>

            {/* Prominent One-Shot Upload Dropzone */}
            <section
              data-testid="upload-dropzone"
              onDragOver={(e) => {
                e.preventDefault()
                setIsDragging(true)
              }}
              onDragLeave={() => setIsDragging(false)}
              onDrop={(e) => {
                e.preventDefault()
                setIsDragging(false)
                const droppedFiles = Array.from(e.dataTransfer?.files ?? [])
                  .map((f) => ("path" in f && typeof f.path === "string" ? f.path : f.name))
                  .filter(Boolean)
                if (droppedFiles.length > 0) {
                  void openPreflightForPaths(droppedFiles)
                }
              }}
              className={`rounded-xl border-2 border-dashed p-8 text-center transition-colors ${
                isDragging
                  ? "border-emerald-400 bg-emerald-950/20"
                  : "border-slate-700 bg-slate-900/40 hover:border-slate-600"
              }`}
            >
              <h2 className="text-base font-semibold text-slate-100">
                Drop photos, videos, or folders here for one-shot upload
              </h2>
              <p className="text-xs text-slate-400 mt-1">
                Preflight summary lets you review file count, size, destination, and conflict handling before uploading.
              </p>
              <div className="mt-4 flex items-center justify-center gap-3">
                <button
                  type="button"
                  onClick={() => void handleSelectFiles()}
                  className="px-4 py-2 rounded-md bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-medium"
                >
                  Select Files
                </button>
                <button
                  type="button"
                  onClick={() => void handleSelectFolder()}
                  className="px-4 py-2 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-100 text-sm font-medium border border-slate-700"
                >
                  Select Folder
                </button>
              </div>
            </section>

            {/* Pre-Upload Confirmation Sheet / Modal */}
            {(uploadPlan || planError) && pendingPaths && (
              <section
                data-testid="upload-plan-modal"
                role="dialog"
                aria-label="Pre-upload confirmation"
                className="rounded-xl border border-emerald-500/40 bg-slate-900 p-5 space-y-4"
              >
                <div className="flex items-center justify-between">
                  <h3 className="text-base font-semibold text-emerald-300">
                    Confirm Upload Plan: {uploadPlan?.sourceSummary}
                  </h3>
                  <span className="text-xs text-slate-400">
                    {uploadPlan ? formatBytes(uploadPlan.totalBytes) : ""}
                  </span>
                </div>

                {planError && (
                  <div className="rounded bg-rose-950/60 border border-rose-800 px-3 py-2 text-xs text-rose-200">
                    {planError}
                  </div>
                )}

                {uploadPlan && (
                  <>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                      <div className="rounded bg-slate-950/70 p-2.5 border border-slate-800">
                        <div className="text-slate-400">Total Selected</div>
                        <div className="text-sm font-semibold text-slate-100">
                          {uploadPlan.totalCount} files ({formatBytes(uploadPlan.totalBytes)})
                        </div>
                      </div>
                      <div className="rounded bg-slate-950/70 p-2.5 border border-slate-800">
                        <div className="text-slate-400">Photos</div>
                        <div className="text-sm font-semibold text-slate-100">
                          {uploadPlan.photoCount} {uploadPlan.photoCount === 1 ? "photo" : "photos"}
                        </div>
                      </div>
                      <div className="rounded bg-slate-950/70 p-2.5 border border-slate-800">
                        <div className="text-slate-400">Videos</div>
                        <div className="text-sm font-semibold text-slate-100">
                          {uploadPlan.videoCount} {uploadPlan.videoCount === 1 ? "video" : "videos"}
                        </div>
                      </div>
                      <div className="rounded bg-slate-950/70 p-2.5 border border-slate-800">
                        <div className="text-slate-400">Skipped</div>
                        <div className="text-sm font-semibold text-slate-100">
                          {uploadPlan.skippedUnsupported} skipped
                        </div>
                      </div>
                    </div>

                    <div className="rounded bg-slate-950/80 border border-slate-800 px-3 py-2 text-xs text-slate-300">
                      <span className="font-medium text-slate-200">Destination: </span>
                      {uploadPlan.destination.message}
                    </div>

                    <div className="flex flex-wrap items-center gap-4 text-xs">
                      <div className="flex items-center gap-2">
                        <span className="text-slate-400">Media Filter:</span>
                        <button
                          type="button"
                          onClick={() => setPlanIncludeVideos(true)}
                          className={`px-2.5 py-1 rounded font-medium ${
                            planIncludeVideos
                              ? "bg-emerald-600 text-white"
                              : "bg-slate-800 text-slate-300"
                          }`}
                        >
                          Photos + Videos
                        </button>
                        <button
                          type="button"
                          onClick={() => setPlanIncludeVideos(false)}
                          className={`px-2.5 py-1 rounded font-medium ${
                            !planIncludeVideos
                              ? "bg-emerald-600 text-white"
                              : "bg-slate-800 text-slate-300"
                          }`}
                        >
                          Photos Only
                        </button>
                      </div>

                      <div className="flex items-center gap-2">
                        <label htmlFor="plan-conflict-policy" className="text-slate-400">
                          Conflict Policy:
                        </label>
                        <select
                          id="plan-conflict-policy"
                          aria-label="Conflict Policy"
                          value={planConflictPolicy}
                          onChange={(e) => setPlanConflictPolicy(e.target.value)}
                          className="rounded bg-slate-800 border border-slate-700 px-2 py-1 text-slate-100"
                        >
                          <option value="skip">skip</option>
                          <option value="rename">rename</option>
                          <option value="fail">fail</option>
                        </select>
                      </div>
                    </div>

                    {/* Local File Preview List */}
                    <div className="max-h-48 overflow-y-auto space-y-1.5 border border-slate-800 rounded p-2 bg-slate-950/50">
                      {uploadPlan.items.map((item) => (
                        <div
                          key={item.localPath}
                          className="flex items-center justify-between text-xs px-2 py-1 rounded bg-slate-900/70"
                        >
                          <div className="flex items-center gap-2.5">
                            {item.thumbnailDataUrl && (
                              <img
                                src={item.thumbnailDataUrl}
                                alt={item.name}
                                className="w-7 h-7 rounded object-cover border border-slate-700"
                              />
                            )}
                            <div>
                              <div className="font-medium text-slate-200">{item.name}</div>
                              <div className="text-[11px] text-slate-500">{item.localPath}</div>
                            </div>
                          </div>
                          <div className="text-slate-400">
                            {item.mediaType} · {formatBytes(item.size)}
                          </div>
                        </div>
                      ))}
                    </div>
                  </>
                )}

                <div className="flex items-center justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={() => {
                      setPendingPaths(null)
                      setUploadPlan(null)
                      setPlanError(null)
                    }}
                    className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    disabled={!uploadPlan || uploadPlan.totalCount === 0}
                    onClick={() => void handleConfirmUpload()}
                    className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-xs font-medium text-white"
                  >
                    Confirm &amp; Start Upload
                  </button>
                </div>
              </section>
            )}

            {/* Active Transfer Queue */}
            <section data-testid="active-transfers" className="space-y-3">
              <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-400">
                Active Transfers ({transferList.active.length})
              </h3>
              {transferList.active.length === 0 ? (
                <div className="rounded-lg border border-slate-800/80 bg-slate-900/30 p-4 text-xs text-slate-500">
                  No active transfers.
                </div>
              ) : (
                <div className="space-y-3">{transferList.active.map(renderTransferRow)}</div>
              )}
            </section>

            {/* Transfer History */}
            <section data-testid="history-transfers" className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-400">
                  Transfer History ({transferList.history.length})
                </h3>
                {transferList.history.length > 0 && (
                  <button
                    type="button"
                    onClick={() =>
                      void backend.transfers.clearHistory().then(() => refreshTransfers())
                    }
                    className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs text-slate-300"
                  >
                    Clear History
                  </button>
                )}
              </div>
              {transferList.history.length === 0 ? (
                <div className="rounded-lg border border-slate-800/80 bg-slate-900/30 p-4 text-xs text-slate-500">
                  No completed or past transfers.
                </div>
              ) : (
                <div className="space-y-3">{transferList.history.map(renderTransferRow)}</div>
              )}
            </section>
          </div>
        )}

        {activeView === "backups" && (
          <div className="space-y-6">
            {/* One-Way Backup Policy Banner */}
            <section
              data-testid="one-way-backup-banner"
              className="rounded-lg border border-emerald-500/30 bg-emerald-950/20 p-4 text-xs text-emerald-200"
            >
              One-way incremental backup: deleting, moving, or renaming local files never deletes your Amazon Photos cloud copies.
            </section>

            {/* Header & Global Controls */}
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-slate-100">Automatic Folder Backups</h2>
                <p className="text-xs text-slate-400">
                  Configured local folders are scanned incrementally while Rainforest is running.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => void handleToggleAllBackupsStopped()}
                  className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 border border-slate-700"
                >
                  {settingsState?.backupsStopped ? "Resume All Backups" : "Stop All Backups"}
                </button>
                <button
                  type="button"
                  onClick={openAddBackupModal}
                  className="px-3.5 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-xs font-medium text-white"
                >
                  Add Backup Folder
                </button>
              </div>
            </div>

            {/* Add / Edit Backup Modal */}
            {backupModalOpen && (
              <section
                role="dialog"
                aria-label={editingBackupRule ? "Edit Backup Folder" : "Add Backup Folder"}
                className="rounded-xl border border-emerald-500/40 bg-slate-900 p-5 space-y-4"
              >
                <h3 className="text-base font-semibold text-emerald-300">
                  {editingBackupRule ? "Edit Backup Folder" : "Add Backup Folder"}
                </h3>

                {backupFormError && (
                  <div className="rounded bg-rose-950/60 border border-rose-800 px-3 py-2 text-xs text-rose-200">
                    {backupFormError}
                  </div>
                )}

                <div className="space-y-3 text-xs">
                  <div>
                    <label htmlFor="backup-folder-path" className="block text-slate-300 mb-1">
                      Local Folder Path
                    </label>
                    <div className="flex items-center gap-2">
                      <input
                        id="backup-folder-path"
                        aria-label="Local Folder Path"
                        type="text"
                        value={backupLocalPath}
                        onChange={(e) => setBackupLocalPath(e.target.value)}
                        placeholder="/Users/alice/Pictures"
                        className="flex-1 rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                      />
                      <button
                        type="button"
                        onClick={() => void handlePickBackupFolder()}
                        className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium border border-slate-700"
                      >
                        Pick Folder
                      </button>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-4">
                    <div className="flex items-center gap-2">
                      <span className="text-slate-400">Media Types:</span>
                      <button
                        type="button"
                        onClick={() => setBackupIncludeVideos(true)}
                        className={`px-2.5 py-1 rounded font-medium ${
                          backupIncludeVideos
                            ? "bg-emerald-600 text-white"
                            : "bg-slate-800 text-slate-300"
                        }`}
                      >
                        Photos + Videos
                      </button>
                      <button
                        type="button"
                        onClick={() => setBackupIncludeVideos(false)}
                        className={`px-2.5 py-1 rounded font-medium ${
                          !backupIncludeVideos
                            ? "bg-emerald-600 text-white"
                            : "bg-slate-800 text-slate-300"
                        }`}
                      >
                        Photos Only
                      </button>
                    </div>

                    <div className="flex items-center gap-2">
                      <span className="text-slate-400">Schedule:</span>
                      <button
                        type="button"
                        onClick={() => setBackupScheduleMode("immediate")}
                        className={`px-2.5 py-1 rounded font-medium ${
                          backupScheduleMode === "immediate"
                            ? "bg-emerald-600 text-white"
                            : "bg-slate-800 text-slate-300"
                        }`}
                      >
                        Immediate + Periodic
                      </button>
                      <button
                        type="button"
                        onClick={() => setBackupScheduleMode("interval")}
                        className={`px-2.5 py-1 rounded font-medium ${
                          backupScheduleMode === "interval"
                            ? "bg-emerald-600 text-white"
                            : "bg-slate-800 text-slate-300"
                        }`}
                      >
                        Scheduled Interval
                      </button>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-4">
                    <div className="flex items-center gap-2">
                      <label htmlFor="backup-interval-minutes" className="text-slate-400">
                        Interval (minutes):
                      </label>
                      <input
                        id="backup-interval-minutes"
                        aria-label="Interval (minutes)"
                        type="number"
                        min={1}
                        value={backupIntervalMinutes}
                        onChange={(e) => setBackupIntervalMinutes(Number(e.target.value) || 30)}
                        className="w-20 rounded bg-slate-950 border border-slate-700 px-2 py-1 text-slate-100"
                      />
                    </div>

                    <div className="flex items-center gap-2">
                      <label htmlFor="backup-conflict-policy" className="text-slate-400">
                        Conflict Policy:
                      </label>
                      <select
                        id="backup-conflict-policy"
                        aria-label="Conflict Policy"
                        value={backupConflictPolicy}
                        onChange={(e) => setBackupConflictPolicy(e.target.value)}
                        className="rounded bg-slate-800 border border-slate-700 px-2 py-1 text-slate-100"
                      >
                        <option value="skip">skip</option>
                        <option value="rename">rename</option>
                      </select>
                    </div>
                  </div>
                </div>

                <div className="flex items-center justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={() => {
                      setBackupModalOpen(false)
                      setEditingBackupRule(null)
                    }}
                    className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs text-slate-300"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleSaveBackupRule()}
                    className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-xs font-medium text-white"
                  >
                    Save Backup Rule
                  </button>
                </div>
              </section>
            )}

            {/* Backup Rules List */}
            <section className="space-y-3">
              {backupRules.length === 0 ? (
                <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-6 text-center text-xs text-slate-500">
                  No backup folders configured. Click &quot;Add Backup Folder&quot; to start one-way automatic backup.
                </div>
              ) : (
                backupRules.map((rule) => (
                  <div
                    key={rule.id}
                    data-testid={`backup-rule-${rule.id}`}
                    className="rounded-lg border border-slate-800 bg-slate-900/50 p-4 space-y-3"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-sm text-slate-100">{rule.localPath}</span>
                        <span
                          className={`px-2 py-0.5 rounded text-xs font-medium ${
                            rule.status === "active"
                              ? "bg-emerald-500/20 text-emerald-300"
                              : rule.status === "scanning"
                                ? "bg-sky-500/20 text-sky-300"
                                : rule.status === "paused"
                                  ? "bg-amber-500/20 text-amber-300"
                                  : "bg-rose-500/20 text-rose-300"
                          }`}
                        >
                          {rule.status}
                        </span>
                      </div>

                      <div className="flex flex-wrap items-center gap-1.5">
                        <button
                          type="button"
                          onClick={() => void backend.backups.scanNow(rule.id)}
                          className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
                        >
                          Scan Now
                        </button>
                        {rule.paused ? (
                          <button
                            type="button"
                            onClick={() => void backend.backups.resume(rule.id)}
                            className="px-2.5 py-1 rounded bg-emerald-600/30 hover:bg-emerald-600/40 text-xs font-medium text-emerald-200"
                          >
                            Resume
                          </button>
                        ) : (
                          <button
                            type="button"
                            onClick={() => void backend.backups.pause(rule.id)}
                            className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
                          >
                            Pause
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => openEditBackupModal(rule)}
                          className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300"
                        >
                          Edit
                        </button>
                        <button
                          type="button"
                          onClick={() =>
                            void backend.backups.remove(rule.id).then(async () => {
                              const updated = await backend.backups.list()
                              setBackupRules(updated)
                            })
                          }
                          className="px-2.5 py-1 rounded bg-rose-600/20 hover:bg-rose-600/30 text-xs font-medium text-rose-300"
                        >
                          Remove
                        </button>
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-4 text-xs text-slate-400">
                      <span>{rule.includeVideos ? "Photos + Videos" : "Photos Only"}</span>
                      <span>
                        Schedule: {rule.scheduleMode} ({rule.intervalMinutes}m)
                      </span>
                      <span>Conflict: {rule.conflictPolicy}</span>
                      {rule.lastScanAt && <span>Last scan: {rule.lastScanAt}</span>}
                    </div>

                    {rule.status === "source_unavailable" && (
                      <div className="rounded bg-amber-950/60 border border-amber-700/60 px-3 py-2 text-xs text-amber-200">
                        External drive or folder is currently offline — cloud files are untouched.
                      </div>
                    )}
                  </div>
                ))
              )}
            </section>
          </div>
        )}

        {activeView === "downloads" && (
          <div data-testid="downloads-view" className="space-y-6">
            <div>
              <h2 className="text-lg font-semibold text-slate-100">
                Recovery Download (Secondary)
              </h2>
              <p className="text-xs text-slate-400">
                Restore original files from confirmed transfer records, verified albums/folders, or explicit remote node IDs.
              </p>
            </div>

            {/* Regional Capability & Evidence Notice */}
            <section
              data-testid="recovery-capability-notice"
              className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 space-y-2"
            >
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <span className="font-medium text-slate-300">Region Capability:</span>
                <span className="px-2 py-0.5 rounded bg-slate-800 font-mono uppercase text-slate-200">
                  {(recoverySources?.region || auth?.region || "us").toUpperCase()}
                </span>
                <span
                  className={`px-2 py-0.5 rounded font-medium ${
                    (recoverySources?.evidence || auth?.evidence) === "verified"
                      ? "bg-emerald-500/20 text-emerald-300"
                      : "bg-amber-500/20 text-amber-300"
                  }`}
                >
                  {recoverySources?.evidence || auth?.evidence || "experimental"}
                </span>
              </div>
              <p className="text-xs text-slate-300">
                {recoverySources?.capabilityNotes ||
                  "Recovery download supports confirmed transfer history items, explicit remote node IDs, and verified folders or albums."}
              </p>
              {recoverySources?.warning && (
                <p className="text-xs text-amber-300">{recoverySources.warning}</p>
              )}
            </section>

            {downloadError && (
              <div className="rounded bg-rose-950/60 border border-rose-800 px-3 py-2 text-xs text-rose-200">
                {downloadError}
              </div>
            )}

            {/* Source Selector Tabs */}
            <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <button
                  type="button"
                  onClick={() => setRecoveryMode("history")}
                  className={`px-3 py-1.5 rounded font-medium ${
                    recoveryMode === "history"
                      ? "bg-emerald-600 text-white"
                      : "bg-slate-800 text-slate-300"
                  }`}
                >
                  Confirmed Transfer History
                </button>
                <button
                  type="button"
                  disabled={
                    recoverySources !== null &&
                    recoverySources.evidence !== "verified" &&
                    recoverySources.folders.length === 0 &&
                    recoverySources.albums.length === 0
                  }
                  onClick={() => setRecoveryMode("container")}
                  className={`px-3 py-1.5 rounded font-medium disabled:opacity-40 disabled:cursor-not-allowed ${
                    recoveryMode === "container"
                      ? "bg-emerald-600 text-white"
                      : "bg-slate-800 text-slate-300"
                  }`}
                >
                  Remote Albums / Folders
                </button>
                <button
                  type="button"
                  onClick={() => setRecoveryMode("nodes")}
                  className={`px-3 py-1.5 rounded font-medium ${
                    recoveryMode === "nodes"
                      ? "bg-emerald-600 text-white"
                      : "bg-slate-800 text-slate-300"
                  }`}
                >
                  Explicit Remote Node IDs
                </button>
              </div>

              {recoveryMode === "history" && (
                <div className="space-y-2">
                  <div className="text-xs text-slate-400">
                    Select confirmed uploaded files to download and verify:
                  </div>
                  {(recoverySources?.confirmedItems ?? []).length === 0 ? (
                    <div className="text-xs text-slate-500 py-2">
                      No confirmed transfer history items available yet.
                    </div>
                  ) : (
                    <div className="space-y-1.5 max-h-48 overflow-y-auto">
                      {(recoverySources?.confirmedItems ?? []).map((item) => {
                        const nodeId = item.remoteNodeId || item.id
                        return (
                          <label
                            key={item.id}
                            className="flex items-center justify-between text-xs p-2 rounded bg-slate-950/60 border border-slate-800 cursor-pointer"
                          >
                            <div className="flex items-center gap-2">
                              <input
                                type="checkbox"
                                checked={!!selectedHistoryNodes[nodeId]}
                                onChange={(e) =>
                                  setSelectedHistoryNodes((prev) => ({
                                    ...prev,
                                    [nodeId]: e.target.checked,
                                  }))
                                }
                              />
                              <span className="font-medium text-slate-200">{item.name}</span>
                              <span className="text-slate-500">({formatBytes(item.size)})</span>
                            </div>
                            <span className="font-mono text-[11px] text-emerald-400">
                              {item.remoteNodeId}
                            </span>
                          </label>
                        )
                      })}
                    </div>
                  )}
                </div>
              )}

              {recoveryMode === "container" && (
                <div className="space-y-2 text-xs">
                  <label htmlFor="recovery-container-select" className="block text-slate-300">
                    Select Remote Folder or Album
                  </label>
                  <select
                    id="recovery-container-select"
                    aria-label="Select Remote Folder or Album"
                    value={selectedContainerId}
                    onChange={(e) => setSelectedContainerId(e.target.value)}
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                  >
                    <option value="">-- Choose an album or folder --</option>
                    {(recoverySources?.folders ?? []).map((f) => (
                      <option key={f.id} value={f.id}>
                        [Folder] {f.name} ({f.id})
                      </option>
                    ))}
                    {(recoverySources?.albums ?? []).map((a) => (
                      <option key={a.id} value={a.id}>
                        [Album] {a.name} ({a.id})
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {recoveryMode === "nodes" && (
                <div className="space-y-2 text-xs">
                  <label htmlFor="recovery-node-ids" className="block text-slate-300">
                    Remote Node IDs (comma or space separated)
                  </label>
                  <input
                    id="recovery-node-ids"
                    aria-label="Remote Node IDs"
                    type="text"
                    value={explicitNodeIds}
                    onChange={(e) => setExplicitNodeIds(e.target.value)}
                    placeholder="node1, node2"
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                  />
                </div>
              )}

              {/* Target Directory & Conflict Policy */}
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-2 text-xs">
                <div className="sm:col-span-2">
                  <label htmlFor="download-target-dir" className="block text-slate-300 mb-1">
                    Target Local Directory
                  </label>
                  <div className="flex items-center gap-2">
                    <input
                      id="download-target-dir"
                      aria-label="Target Local Directory"
                      type="text"
                      value={downloadDestDir}
                      onChange={(e) => setDownloadDestDir(e.target.value)}
                      placeholder="/Users/alice/Restored"
                      className="flex-1 rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                    />
                    <button
                      type="button"
                      onClick={() => void handlePickDownloadDir()}
                      className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium border border-slate-700"
                    >
                      Pick Directory
                    </button>
                  </div>
                </div>

                <div>
                  <label htmlFor="download-conflict-policy" className="block text-slate-300 mb-1">
                    Conflict Policy
                  </label>
                  <select
                    id="download-conflict-policy"
                    aria-label="Download Conflict Policy"
                    value={downloadConflictPolicy}
                    onChange={(e) => setDownloadConflictPolicy(e.target.value)}
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                  >
                    <option value="skip">skip</option>
                    <option value="rename">rename</option>
                    <option value="fail">fail</option>
                  </select>
                </div>
              </div>

              <div className="flex justify-end pt-2">
                <button
                  type="button"
                  onClick={() => void handleStartRecoveryDownload()}
                  className="px-4 py-2 rounded bg-emerald-600 hover:bg-emerald-500 text-xs font-medium text-white"
                >
                  Start Recovery Download
                </button>
              </div>
            </section>

            {/* Created Download Transfer & Per-Item Verified Status */}
            {latestDownloadTransfer && (
              <section
                data-testid="recovery-download-results"
                className="rounded-xl border border-emerald-500/30 bg-slate-900/60 p-4 space-y-3"
              >
                <div className="flex items-center justify-between text-xs">
                  <span className="font-semibold text-emerald-300">
                    Recovery Transfer: {latestDownloadTransfer.sourceSummary} (
                    {latestDownloadTransfer.stage})
                  </span>
                  <span className="text-slate-400">
                    Destination: {latestDownloadTransfer.destLocalDir}
                  </span>
                </div>
                <div className="space-y-1.5">
                  {latestDownloadItems.map((it) => (
                    <div
                      key={it.id}
                      className="flex items-center justify-between text-xs px-3 py-1.5 rounded bg-slate-950/70 border border-slate-800"
                    >
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-slate-200">{it.name}</span>
                        <span className="text-slate-500">({formatBytes(it.size)})</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-[11px] text-slate-400">
                          {it.remoteNodeId}
                        </span>
                        <span className="px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 font-medium">
                          {it.status}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            )}
          </div>
        )}

        {activeView === "settings" && (
          <div className="space-y-6">
            {/* Account & Session */}
            <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <h2 className="text-lg font-semibold text-slate-100">Account &amp; Session</h2>
                  <p className="text-xs text-slate-400">
                    Import browser session cookies and choose your explicit Amazon marketplace region.
                  </p>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  {auth?.authenticated ? (
                    <>
                      <span className="px-2.5 py-1 rounded bg-emerald-500/20 text-emerald-300 font-medium">
                        Authenticated ({(auth.region || selectedRegion || "us").toUpperCase()} ·{" "}
                        {auth.evidence || "verified"})
                      </span>
                      <button
                        type="button"
                        onClick={() => void handleLogout()}
                        className="px-3 py-1 rounded bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 font-medium"
                      >
                        Sign Out
                      </button>
                    </>
                  ) : (
                    <span className="px-2.5 py-1 rounded bg-amber-500/20 text-amber-300 font-medium">
                      Not Authenticated
                    </span>
                  )}
                </div>
              </div>

              {authFormError && (
                <div className="rounded bg-rose-950/60 border border-rose-800 px-3 py-2 text-xs text-rose-200">
                  {authFormError}
                </div>
              )}

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
                <div>
                  <label htmlFor="settings-region-select" className="block text-slate-300 mb-1">
                    Marketplace Region (Never silent .com)
                  </label>
                  <select
                    id="settings-region-select"
                    aria-label="Marketplace Region"
                    value={selectedRegion}
                    onChange={(e) => setSelectedRegion(e.target.value)}
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                  >
                    <option value="">-- Select Region --</option>
                    <option value="us">US (amazon.com · verified)</option>
                    <option value="ca">CA (amazon.ca · experimental)</option>
                    <option value="uk">UK (amazon.co.uk · experimental)</option>
                    <option value="jp">JP (amazon.co.jp · experimental)</option>
                    <option value="de">DE (amazon.de · experimental)</option>
                    <option value="fr">FR (amazon.fr · experimental)</option>
                    <option value="it">IT (amazon.it · experimental)</option>
                    <option value="es">ES (amazon.es · experimental)</option>
                  </select>
                </div>

                <div className="sm:col-span-2">
                  <label htmlFor="settings-cookie-input" className="block text-slate-300 mb-1">
                    Session Cookies (Raw Cookie Header, Netscape cookies.txt, or JSON)
                  </label>
                  <textarea
                    id="settings-cookie-input"
                    aria-label="Session Cookies"
                    rows={2}
                    value={cookieInput}
                    onChange={(e) => setCookieInput(e.target.value)}
                    placeholder="session-id=...; ubid-main=...; at-main=..."
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100 font-mono"
                  />
                </div>
              </div>

              <div className="flex flex-wrap items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={() => void handlePickAndImportCookieFile()}
                  className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 border border-slate-700"
                >
                  Import Cookie File
                </button>
                <button
                  type="button"
                  onClick={() => void handleImportPastedCookies()}
                  className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-xs font-medium text-white"
                >
                  Import Pasted Cookies
                </button>
              </div>
            </section>

            {/* Storage Usage Breakdown */}
            <section
              data-testid="settings-storage-usage"
              className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-3"
            >
              <h2 className="text-lg font-semibold text-slate-100">Storage Usage Breakdown</h2>
              {usage ? (
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                  <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
                    <div className="text-slate-400">Photos</div>
                    <div className="text-sm font-semibold text-slate-100">
                      {usage.photo.total.count.toLocaleString("en-US")} items (
                      {formatBytes(usage.photo.total.bytes)})
                    </div>
                  </div>
                  <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
                    <div className="text-slate-400">Videos</div>
                    <div className="text-sm font-semibold text-slate-100">
                      {usage.video.total.count.toLocaleString("en-US")} items (
                      {formatBytes(usage.video.total.bytes)})
                    </div>
                  </div>
                  <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
                    <div className="text-slate-400">Total Used</div>
                    <div className="text-sm font-semibold text-slate-100">
                      {formatBytes(usage.totalBytes)}
                    </div>
                  </div>
                  <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
                    <div className="text-slate-400">Billable Used</div>
                    <div className="text-sm font-semibold text-slate-100">
                      {formatBytes(usage.totalBillableBytes)}
                    </div>
                  </div>
                </div>
              ) : (
                <p className="text-xs text-slate-400">
                  Sign in with valid Amazon Photos cookies to view storage usage breakdown.
                </p>
              )}
            </section>

            {/* Transfer & Desktop Behavior */}
            <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
              <h2 className="text-lg font-semibold text-slate-100">
                Transfer &amp; Desktop Behavior
              </h2>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
                <div>
                  <label htmlFor="settings-metadata-concurrency" className="block text-slate-300 mb-1">
                    Metadata Concurrency (1–16)
                  </label>
                  <input
                    id="settings-metadata-concurrency"
                    aria-label="Metadata Concurrency"
                    type="number"
                    min={1}
                    max={16}
                    value={metadataConcurrency}
                    onChange={(e) => setMetadataConcurrency(Number(e.target.value) || 4)}
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                  />
                </div>

                <div>
                  <label htmlFor="settings-upload-concurrency" className="block text-slate-300 mb-1">
                    Upload Concurrency (1–8)
                  </label>
                  <input
                    id="settings-upload-concurrency"
                    aria-label="Upload Concurrency"
                    type="number"
                    min={1}
                    max={8}
                    value={uploadConcurrency}
                    onChange={(e) => setUploadConcurrency(Number(e.target.value) || 2)}
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                  />
                </div>

                <div>
                  <label htmlFor="settings-default-conflict" className="block text-slate-300 mb-1">
                    Default Conflict Policy
                  </label>
                  <select
                    id="settings-default-conflict"
                    aria-label="Default Conflict Policy"
                    value={defaultConflictPolicy}
                    onChange={(e) => setDefaultConflictPolicy(e.target.value)}
                    className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
                  >
                    <option value="skip">skip</option>
                    <option value="rename">rename</option>
                    <option value="fail">fail</option>
                  </select>
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-6 text-xs pt-2">
                <label className="flex items-center gap-2 cursor-pointer text-slate-200">
                  <input
                    type="checkbox"
                    checked={closeToTray}
                    onChange={(e) => setCloseToTray(e.target.checked)}
                  />
                  <span>Close window to system tray</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer text-slate-200">
                  <input
                    type="checkbox"
                    checked={launchAtLogin}
                    onChange={(e) => setLaunchAtLogin(e.target.checked)}
                  />
                  <span>Launch at login</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer text-slate-200">
                  <input
                    type="checkbox"
                    checked={notificationsEnabled}
                    onChange={(e) => setNotificationsEnabled(e.target.checked)}
                  />
                  <span>Desktop notifications</span>
                </label>
              </div>

              {/* Clear Distinction Guide */}
              <div
                data-testid="lifecycle-guide"
                className="rounded-lg border border-slate-800 bg-slate-950/70 p-3.5 space-y-1.5 text-xs text-slate-300"
              >
                <div className="font-semibold text-slate-200">
                  Desktop Lifecycle &amp; Action Reference:
                </div>
                <ul className="space-y-1 text-slate-400 list-disc list-inside">
                  <li>
                    <strong className="text-slate-200">Pause Task</strong>: Pauses one active upload or download transfer in the queue; other transfers and backups continue.
                  </li>
                  <li>
                    <strong className="text-slate-200">Stop Backups</strong>: Halts background folder scans across all configured backup rules until resumed.
                  </li>
                  <li>
                    <strong className="text-slate-200">Close Window</strong>: Hides the window to the system tray (when enabled) while active transfers and backups continue running in the background.
                  </li>
                  <li>
                    <strong className="text-slate-200">Quit Rainforest</strong>: Safely stops background workers and exits the application completely.
                  </li>
                </ul>
              </div>

              <div className="flex items-center justify-end gap-3">
                {settingsSavedMsg && (
                  <span className="text-xs text-emerald-400">{settingsSavedMsg}</span>
                )}
                <button
                  type="button"
                  onClick={() => void handleSaveSettings()}
                  className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-xs font-medium text-white"
                >
                  Save Settings
                </button>
              </div>
            </section>

            {/* Doctor Diagnostics */}
            <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-semibold text-slate-100">Doctor Diagnostics</h2>
                  <p className="text-xs text-slate-400">
                    Graded connectivity, credential vault, region routing, and read-probe diagnostics.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => void handleRunDoctor()}
                  className="px-3.5 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-100 border border-slate-700"
                >
                  Run Doctor
                </button>
              </div>

              {doctorReport && (
                <div data-testid="doctor-report" className="space-y-2">
                  {doctorReport.checks.map((chk) => (
                    <div
                      key={`${chk.grade}-${chk.name}`}
                      className="rounded border border-slate-800 bg-slate-950/70 p-3 text-xs space-y-1"
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-mono font-medium text-slate-200">
                          [Grade {chk.grade}] {chk.name}
                        </span>
                        <span
                          className={`px-2 py-0.5 rounded uppercase font-semibold text-[11px] ${
                            chk.status === "pass"
                              ? "bg-emerald-500/20 text-emerald-300"
                              : chk.status === "warn"
                                ? "bg-amber-500/20 text-amber-300"
                                : chk.status === "skipped"
                                  ? "bg-slate-800 text-slate-400"
                                  : "bg-rose-500/20 text-rose-300"
                          }`}
                        >
                          {chk.status}
                        </span>
                      </div>
                      {chk.message && <p className="text-slate-300">{chk.message}</p>}
                      {chk.hint && <p className="text-amber-300">Hint: {chk.hint}</p>}
                    </div>
                  ))}
                </div>
              )}
            </section>

            {/* About / Disclaimer */}
            <section
              data-testid="about-disclaimer"
              className="rounded-xl border border-slate-800 bg-slate-900/30 p-4 text-xs text-slate-400 space-y-1"
            >
              <div className="font-semibold text-slate-300">About Rainforest</div>
              <p>
                Rainforest is an independent, unofficial client and is not affiliated with or endorsed by Amazon.com, Inc.
              </p>
              <p>License: AGPL-3.0-only</p>
            </section>
          </div>
        )}
      </main>
    </div>
  )
}
