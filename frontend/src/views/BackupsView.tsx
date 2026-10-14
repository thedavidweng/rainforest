import { useState } from "react"

import type { Backend, BackupRuleDTO, SettingsDTO } from "@/backend"
import { errorMessageFrom } from "@/backend"

export interface BackupsViewProps {
  backend: Backend
  backupRules: BackupRuleDTO[]
  settingsState: SettingsDTO | null
  onSetBackupRules: (rules: BackupRuleDTO[]) => void
  onToggleAllStopped: () => Promise<void>
}

export function BackupsView({
  backend,
  backupRules,
  settingsState,
  onSetBackupRules,
  onToggleAllStopped,
}: BackupsViewProps) {
  const [backupModalOpen, setBackupModalOpen] = useState(false)
  const [editingBackupRule, setEditingBackupRule] = useState<BackupRuleDTO | null>(null)
  const [backupLocalPath, setBackupLocalPath] = useState("")
  const [backupIncludeVideos, setBackupIncludeVideos] = useState(true)
  const [backupScheduleMode, setBackupScheduleMode] = useState("immediate")
  const [backupIntervalMinutes, setBackupIntervalMinutes] = useState(30)
  const [backupConflictPolicy, setBackupConflictPolicy] = useState("skip")
  const [backupFormError, setBackupFormError] = useState<string | null>(null)

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
      onSetBackupRules(rules)
      setBackupModalOpen(false)
      setEditingBackupRule(null)
    } catch (err) {
      setBackupFormError(errorMessageFrom(err))
    }
  }

  return (
    <div className="space-y-6">
      <section
        data-testid="one-way-backup-banner"
        className="rounded-lg border border-emerald-500/30 bg-emerald-950/20 p-4 text-xs text-emerald-200"
      >
        One-way incremental backup: deleting, moving, or renaming local files never deletes your
        Amazon Photos cloud copies.
      </section>

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
            onClick={() => void onToggleAllStopped()}
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

      <section className="space-y-3">
        {backupRules.length === 0 ? (
          <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-6 text-center text-xs text-slate-500">
            No backup folders configured. Click &quot;Add Backup Folder&quot; to start one-way
            automatic backup.
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
                        onSetBackupRules(updated)
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
  )
}
