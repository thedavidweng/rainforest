import { useState } from "react"

import type {
  AuthStatus,
  Backend,
  TransferDTO,
  TransferItemDTO,
  TransferListDTO,
  UploadPlanDTO,
  UsageSummary,
} from "@/backend"
import { formatBytes, isTerminalStage } from "@/backend"

export interface TransfersViewProps {
  backend: Backend
  auth: AuthStatus | null
  usage: UsageSummary | null
  transferList: TransferListDTO
  expandedTransfers: Record<string, boolean>
  transferItemsMap: Record<string, TransferItemDTO[]>
  pendingPaths: string[] | null
  planIncludeVideos: boolean
  planDestNodeId: string
  onSetPlanDestNodeId: (id: string) => void
  planConflictPolicy: string
  uploadPlan: UploadPlanDTO | null
  planError: string | null
  onOpenSettings: () => void
  onOpenPreflight: (paths: string[]) => Promise<void> | void
  onSelectFiles: () => Promise<void>
  onSelectFolder: () => Promise<void>
  onSetPlanIncludeVideos: (includeVideos: boolean) => void
  onSetPlanConflictPolicy: (policy: string) => void
  onCancelPlan: () => void
  onConfirmUpload: () => Promise<void>
  onToggleDetails: (transferId: string) => Promise<void>
  onRefreshTransfers: () => Promise<void>
}

export function TransfersView({
  backend,
  auth,
  usage,
  transferList,
  expandedTransfers,
  transferItemsMap,
  pendingPaths,
  planIncludeVideos,
  planConflictPolicy,
  planDestNodeId,
  onSetPlanDestNodeId,
  uploadPlan,
  planError,
  onOpenSettings,
  onOpenPreflight,
  onSelectFiles,
  onSelectFolder,
  onSetPlanIncludeVideos,
  onSetPlanConflictPolicy,
  onCancelPlan,
  onConfirmUpload,
  onToggleDetails,
  onRefreshTransfers,
}: TransfersViewProps) {
  const [isDragging, setIsDragging] = useState(false)

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
              onClick={() => void onToggleDetails(tr.id)}
              className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300"
            >
              Details
            </button>
          </div>
        </div>

        <div className="space-y-1">
          <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
            <div className="h-full bg-emerald-500 transition-all" style={{ width: `${pct}%` }} />
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

        {(tr.errorMessage || tr.stage === "unconfirmed") && (
          <div className="rounded bg-rose-950/50 border border-rose-800/60 px-3 py-2 text-xs text-rose-200">
            {tr.errorMessage ||
              "Upload commit unconfirmed: HTTP response did not return a verified node ID. Startup reconciliation will verify remote state before retrying."}
          </div>
        )}

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
                    {it.errorMessage && <span className="text-rose-400">{it.errorMessage}</span>}
                  </div>
                </div>
              ))
            )}
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-6">
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
                {formatBytes(usage.video.total.bytes)}) · Total {formatBytes(usage.totalBytes)}
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
              onClick={onOpenSettings}
              className="px-3 py-1.5 rounded bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 text-xs font-medium border border-amber-500/30"
            >
              Import Cookies in Settings
            </button>
          </div>
        )}
      </section>

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
            void onOpenPreflight(droppedFiles)
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
          Preflight summary lets you review file count, size, destination, and conflict handling
          before uploading.
        </p>
        <div className="mt-4 flex items-center justify-center gap-3">
          <button
            type="button"
            onClick={() => void onSelectFiles()}
            className="px-4 py-2 rounded-md bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-medium"
          >
            Select Files
          </button>
          <button
            type="button"
            onClick={() => void onSelectFolder()}
            className="px-4 py-2 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-100 text-sm font-medium border border-slate-700"
          >
            Select Folder
          </button>
        </div>
      </section>

      {pendingPaths && (
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

          <label className="block text-xs text-slate-300">
            Destination Node ID
            <input
              aria-label="Destination Node ID"
              value={planDestNodeId}
              onChange={(e) => onSetPlanDestNodeId(e.target.value)}
              placeholder="Leave blank for account root"
              className="block w-full rounded bg-slate-950 border border-slate-700 px-3 py-2 mt-1"
            />
          </label>
          {!uploadPlan && !planError && <p className="text-xs">pending — checking destination</p>}
          {planError && (
            <div className="rounded bg-rose-950/60 border border-rose-800 px-3 py-2 text-xs text-rose-200">
              {planError}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-4 text-xs">
            <div className="flex items-center gap-2">
              <span className="text-slate-400">Media Filter:</span>
              <button
                type="button"
                onClick={() => onSetPlanIncludeVideos(true)}
                className={`px-2.5 py-1 rounded font-medium ${
                  planIncludeVideos ? "bg-emerald-600 text-white" : "bg-slate-800 text-slate-300"
                }`}
              >
                Photos + Videos
              </button>
              <button
                type="button"
                onClick={() => onSetPlanIncludeVideos(false)}
                className={`px-2.5 py-1 rounded font-medium ${
                  !planIncludeVideos ? "bg-emerald-600 text-white" : "bg-slate-800 text-slate-300"
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
                onChange={(e) => onSetPlanConflictPolicy(e.target.value)}
                className="rounded bg-slate-800 border border-slate-700 px-2 py-1 text-slate-100"
              >
                <option value="skip">skip</option>
                <option value="rename">rename</option>
                <option value="fail">fail</option>
              </select>
            </div>
          </div>

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
                {uploadPlan.destination.name} · {uploadPlan.destination.kind} ·{" "}
                {uploadPlan.destination.status}. {uploadPlan.destination.message}
              </div>

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
              onClick={onCancelPlan}
              className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300"
            >
              Cancel
            </button>
            <button
              type="button"
              disabled={
                !uploadPlan ||
                uploadPlan.totalCount === 0 ||
                uploadPlan.destination.status !== "readable"
              }
              onClick={() => void onConfirmUpload()}
              className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-xs font-medium text-white"
            >
              Confirm &amp; Start Upload
            </button>
          </div>
        </section>
      )}

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

      <section data-testid="history-transfers" className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-400">
            Transfer History ({transferList.history.length})
          </h3>
          {transferList.history.length > 0 && (
            <button
              type="button"
              onClick={() => void backend.transfers.clearHistory().then(() => onRefreshTransfers())}
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
  )
}
