import { useState } from "react"

import type { AuthStatus, Backend, DoctorReportDTO, SettingsDTO, UsageSummary } from "@/backend"
import { errorMessageFrom, formatBytes } from "@/backend"

export interface SettingsViewProps {
  backend: Backend
  auth: AuthStatus | null
  usage: UsageSummary | null
  selectedRegion: string
  metadataConcurrency: number
  uploadConcurrency: number
  defaultConflictPolicy: string
  closeToTray: boolean
  launchAtLogin: boolean
  notificationsEnabled: boolean
  onSetAuth: (auth: AuthStatus | null) => void
  onSetUsage: (usage: UsageSummary | null) => void
  onSetSelectedRegion: (region: string) => void
  onSetMetadataConcurrency: (v: number) => void
  onSetUploadConcurrency: (v: number) => void
  onSetDefaultConflictPolicy: (v: string) => void
  onSetCloseToTray: (v: boolean) => void
  onSetLaunchAtLogin: (v: boolean) => void
  onSetNotificationsEnabled: (v: boolean) => void
  onSetSettingsState: (cfg: SettingsDTO) => void
  onSetGlobalError: (msg: string | null) => void
}

export function SettingsView({
  backend,
  auth,
  usage,
  selectedRegion,
  metadataConcurrency,
  uploadConcurrency,
  defaultConflictPolicy,
  closeToTray,
  launchAtLogin,
  notificationsEnabled,
  onSetAuth,
  onSetUsage,
  onSetSelectedRegion,
  onSetMetadataConcurrency,
  onSetUploadConcurrency,
  onSetDefaultConflictPolicy,
  onSetCloseToTray,
  onSetLaunchAtLogin,
  onSetNotificationsEnabled,
  onSetSettingsState,
  onSetGlobalError,
}: SettingsViewProps) {
  const [cookieInput, setCookieInput] = useState<string>("")
  const [authFormError, setAuthFormError] = useState<string | null>(null)
  const [settingsSavedMsg, setSettingsSavedMsg] = useState<string | null>(null)
  const [doctorReport, setDoctorReport] = useState<DoctorReportDTO | null>(null)
  const [writeProbeConfirm, setWriteProbeConfirm] = useState<boolean>(false)

  const handleImportPastedCookies = async () => {
    setAuthFormError(null)
    try {
      const st = await backend.auth.importCookies({
        rawCookies: cookieInput,
        region: selectedRegion,
      })
      onSetAuth(st)
      if (st.region) onSetSelectedRegion(st.region)
      setCookieInput("")
    } catch (err) {
      setAuthFormError(errorMessageFrom(err))
    }
  }

  const handlePickAndImportCookieFile = async () => {
    setAuthFormError(null)
    try {
      const st = await backend.auth.pickAndImportCookieFile(selectedRegion)
      onSetAuth(st)
      if (st.region) onSetSelectedRegion(st.region)
    } catch (err) {
      setAuthFormError(errorMessageFrom(err))
    }
  }

  const handleLogout = async () => {
    setAuthFormError(null)
    try {
      const st = await backend.auth.logout()
      onSetAuth(st)
      onSetUsage(null)
    } catch (err) {
      setAuthFormError(errorMessageFrom(err))
    }
  }

  const handleSaveSettings = async () => {
    setSettingsSavedMsg(null)
    onSetGlobalError(null)
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
      onSetSettingsState(updated)
      setSettingsSavedMsg("Settings saved.")
    } catch (err) {
      onSetGlobalError(errorMessageFrom(err))
    }
  }

  const handleRunDoctor = async () => {
    onSetGlobalError(null)
    try {
      const report = writeProbeConfirm
        ? await backend.settings.doctorWithWriteProbe(true)
        : await backend.settings.doctor()
      setDoctorReport(report)
    } catch (err) {
      onSetGlobalError(errorMessageFrom(err))
    }
  }

  return (
    <div className="space-y-6">
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
              onChange={(e) => onSetSelectedRegion(e.target.value)}
              className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
            >
              <option value="">-- Select Region --</option>
              <option value="us">US (amazon.com · verified)</option>
              <option value="ca">CA (amazon.ca · experimental)</option>
              <option value="uk">UK (amazon.co.uk · experimental)</option>
              <option value="jp">JP (amazon.co.jp · experimental)</option>
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

      <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-100">Transfer &amp; Desktop Behavior</h2>

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
              onChange={(e) => onSetMetadataConcurrency(Number(e.target.value) || 4)}
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
              onChange={(e) => onSetUploadConcurrency(Number(e.target.value) || 2)}
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
              onChange={(e) => onSetDefaultConflictPolicy(e.target.value)}
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
              onChange={(e) => onSetCloseToTray(e.target.checked)}
            />
            <span>Close window to system tray</span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer text-slate-200">
            <input
              type="checkbox"
              checked={launchAtLogin}
              onChange={(e) => onSetLaunchAtLogin(e.target.checked)}
            />
            <span>Launch at login</span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer text-slate-200">
            <input
              type="checkbox"
              checked={notificationsEnabled}
              onChange={(e) => onSetNotificationsEnabled(e.target.checked)}
            />
            <span>Desktop notifications</span>
          </label>
        </div>

        <div
          data-testid="lifecycle-guide"
          className="rounded-lg border border-slate-800 bg-slate-950/70 p-3.5 space-y-1.5 text-xs text-slate-300"
        >
          <div className="font-semibold text-slate-200">
            Desktop Lifecycle &amp; Action Reference:
          </div>
          <ul className="space-y-1 text-slate-400 list-disc list-inside">
            <li>
              <strong className="text-slate-200">Pause Task</strong>: Pauses one active upload or
              download transfer in the queue; other transfers and backups continue.
            </li>
            <li>
              <strong className="text-slate-200">Stop Backups</strong>: Halts background folder
              scans across all configured backup rules until resumed.
            </li>
            <li>
              <strong className="text-slate-200">Close Window</strong>: Hides the window to the
              system tray (when enabled) while active transfers and backups continue running in the
              background.
            </li>
            <li>
              <strong className="text-slate-200">Quit Rainforest</strong>: Safely stops background
              workers and exits the application completely.
            </li>
          </ul>
        </div>

        <div className="flex items-center justify-end gap-3">
          {settingsSavedMsg && <span className="text-xs text-emerald-400">{settingsSavedMsg}</span>}
          <button
            type="button"
            onClick={() => void handleSaveSettings()}
            className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-xs font-medium text-white"
          >
            Save Settings
          </button>
        </div>
      </section>

      <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-slate-100">Doctor Diagnostics</h2>
            <p className="text-xs text-slate-400">
              Graded connectivity, credential vault, region routing, read-probe, and opt-in
              write-probe diagnostics.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-2 text-xs text-slate-300 cursor-pointer">
              <input
                type="checkbox"
                checked={writeProbeConfirm}
                onChange={(e) => setWriteProbeConfirm(e.target.checked)}
              />
              <span>Confirm temporary folder write probe (Grade 5)</span>
            </label>
            <button
              type="button"
              onClick={() => void handleRunDoctor()}
              className="px-3.5 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-100 border border-slate-700"
            >
              Run Doctor
            </button>
          </div>
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

      <section
        data-testid="about-disclaimer"
        className="rounded-xl border border-slate-800 bg-slate-900/30 p-4 text-xs text-slate-400 space-y-1"
      >
        <div className="font-semibold text-slate-300">About Rainforest</div>
        <p>
          Rainforest is an independent, unofficial client and is not affiliated with or endorsed by
          Amazon.com, Inc.
        </p>
        <p>License: AGPL-3.0-only</p>
      </section>
    </div>
  )
}
