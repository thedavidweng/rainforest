import { useCallback, useEffect, useState } from "react"

import type {
  AuthStatus,
  Backend,
  BackupRuleDTO,
  SettingsDTO,
  TransferDTO,
  TransferItemDTO,
  TransferListDTO,
  UploadPlanDTO,
  UsageSummary,
} from "@/backend"
import { errorMessageFrom, formatBytes, isTerminalStage } from "@/backend"
import { BackupsView } from "@/views/BackupsView"
import { DownloadsView } from "@/views/DownloadsView"
import { SettingsView } from "@/views/SettingsView"
import { TransfersView } from "@/views/TransfersView"

export { formatBytes }

export interface AppProps {
  backend: Backend
}

type ViewTab = "transfers" | "backups" | "downloads" | "settings"

export function App({ backend }: AppProps) {
  const [activeView, setActiveView] = useState<ViewTab>("transfers")
  const [auth, setAuth] = useState<AuthStatus | null>(null)
  const [usage, setUsage] = useState<UsageSummary | null>(null)
  const [globalError, setGlobalError] = useState<string | null>(null)

  const [transferList, setTransferList] = useState<TransferListDTO>({ active: [], history: [] })
  const [expandedTransfers, setExpandedTransfers] = useState<Record<string, boolean>>({})
  const [transferItemsMap, setTransferItemsMap] = useState<Record<string, TransferItemDTO[]>>({})

  const [pendingPaths, setPendingPaths] = useState<string[] | null>(null)
  const [planIncludeVideos, setPlanIncludeVideos] = useState<boolean>(true)
  const [planConflictPolicy, setPlanConflictPolicy] = useState<string>("skip")
  const [planDestNodeId, setPlanDestNodeId] = useState<string>("")
  const [uploadPlan, setUploadPlan] = useState<UploadPlanDTO | null>(null)
  const [planError, setPlanError] = useState<string | null>(null)

  const [backupRules, setBackupRules] = useState<BackupRuleDTO[]>([])
  const [settingsState, setSettingsState] = useState<SettingsDTO | null>(null)

  const [selectedRegion, setSelectedRegion] = useState<string>("")
  const [metadataConcurrency, setMetadataConcurrency] = useState<number>(4)
  const [uploadConcurrency, setUploadConcurrency] = useState<number>(2)
  const [defaultConflictPolicy, setDefaultConflictPolicy] = useState<string>("skip")
  const [closeToTray, setCloseToTray] = useState<boolean>(true)
  const [launchAtLogin, setLaunchAtLogin] = useState<boolean>(false)
  const [notificationsEnabled, setNotificationsEnabled] = useState<boolean>(true)

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
        // Offline or unconfigured state is handled via auth status banner
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
        // Ignore initial settings fetch failure when offline
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
        openPreflightForPaths(files)
      }
    } catch (err) {
      setGlobalError(errorMessageFrom(err))
    }
  }

  const handleSelectFolder = async () => {
    try {
      const dir = await backend.transfers.pickDirectory()
      if (dir && dir.trim()) {
        openPreflightForPaths([dir.trim()])
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
          <TransfersView
            backend={backend}
            auth={auth}
            usage={usage}
            transferList={transferList}
            expandedTransfers={expandedTransfers}
            transferItemsMap={transferItemsMap}
            pendingPaths={pendingPaths}
            planIncludeVideos={planIncludeVideos}
            planConflictPolicy={planConflictPolicy}
            uploadPlan={uploadPlan}
            planError={planError}
            onOpenSettings={() => setActiveView("settings")}
            onOpenPreflight={openPreflightForPaths}
            onSelectFiles={handleSelectFiles}
            onSelectFolder={handleSelectFolder}
            onSetPlanIncludeVideos={setPlanIncludeVideos}
            onSetPlanConflictPolicy={setPlanConflictPolicy}
            onCancelPlan={() => {
              setPendingPaths(null)
              setUploadPlan(null)
              setPlanError(null)
            }}
            onConfirmUpload={handleConfirmUpload}
            onToggleDetails={handleToggleDetails}
            onRefreshTransfers={refreshTransfers}
          />
        )}

        {activeView === "backups" && (
          <BackupsView
            backend={backend}
            backupRules={backupRules}
            settingsState={settingsState}
            onSetBackupRules={setBackupRules}
            onToggleAllStopped={handleToggleAllBackupsStopped}
          />
        )}

        {activeView === "downloads" && (
          <DownloadsView
            backend={backend}
            auth={auth}
            onRefreshTransfers={refreshTransfers}
          />
        )}

        {activeView === "settings" && (
          <SettingsView
            backend={backend}
            auth={auth}
            usage={usage}
            selectedRegion={selectedRegion}
            metadataConcurrency={metadataConcurrency}
            uploadConcurrency={uploadConcurrency}
            defaultConflictPolicy={defaultConflictPolicy}
            closeToTray={closeToTray}
            launchAtLogin={launchAtLogin}
            notificationsEnabled={notificationsEnabled}
            onSetAuth={setAuth}
            onSetUsage={setUsage}
            onSetSelectedRegion={setSelectedRegion}
            onSetMetadataConcurrency={setMetadataConcurrency}
            onSetUploadConcurrency={setUploadConcurrency}
            onSetDefaultConflictPolicy={setDefaultConflictPolicy}
            onSetCloseToTray={setCloseToTray}
            onSetLaunchAtLogin={setLaunchAtLogin}
            onSetNotificationsEnabled={setNotificationsEnabled}
            onSetSettingsState={setSettingsState}
            onSetGlobalError={setGlobalError}
          />
        )}
      </main>
    </div>
  )
}
