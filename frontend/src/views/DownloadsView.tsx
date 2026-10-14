import { useEffect, useState } from "react"

import type {
  AuthStatus,
  Backend,
  RecoverySourcesDTO,
  TransferDTO,
  TransferItemDTO,
} from "@/backend"
import { errorMessageFrom, formatBytes } from "@/backend"

export interface DownloadsViewProps {
  backend: Backend
  auth: AuthStatus | null
  onRefreshTransfers: () => Promise<void>
}

export function DownloadsView({ backend, auth, onRefreshTransfers }: DownloadsViewProps) {
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

  useEffect(() => {
    let live = true
    backend.downloads
      .listDestinations()
      .then((src) => {
        if (live) setRecoverySources(src)
      })
      .catch((err) => {
        if (live) setDownloadError(errorMessageFrom(err))
      })
    return () => {
      live = false
    }
  }, [backend])

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
          conflictPolicy: downloadConflictPolicy,
        })
      } else if (recoveryMode === "container") {
        tr = await backend.downloads.startDownload({
          containerNodeId: selectedContainerId.trim(),
          destDir: downloadDestDir.trim(),
          conflictPolicy: downloadConflictPolicy,
        })
      } else {
        const ids = explicitNodeIds
          .split(/[\s,]+/)
          .map((s) => s.trim())
          .filter(Boolean)
        tr = await backend.downloads.startDownload({
          nodeIds: ids,
          destDir: downloadDestDir.trim(),
          conflictPolicy: downloadConflictPolicy,
        })
      }

      setLatestDownloadTransfer(tr)
      const items = await backend.transfers.getItems(tr.id)
      setLatestDownloadItems(items)
      await onRefreshTransfers()
    } catch (err) {
      setDownloadError(errorMessageFrom(err))
    }
  }

  return (
    <div data-testid="downloads-view" className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-slate-100">
          Recovery Download (Secondary)
        </h2>
        <p className="text-xs text-slate-400">
          Restore original files from confirmed transfer records, verified albums/folders, or explicit remote node IDs.
        </p>
      </div>

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
  )
}
