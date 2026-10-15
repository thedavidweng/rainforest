import { messages as m } from "@/locales"
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
  const [signingIn, setSigningIn] = useState(false)
  const [authMessage, setAuthMessage] = useState<string | null>(null)
  const [cookieInput, setCookieInput] = useState<string>("")
  const [authFormError, setAuthFormError] = useState<string | null>(null)
  const [settingsSavedMsg, setSettingsSavedMsg] = useState<string | null>(null)
  const [doctorReport, setDoctorReport] = useState<DoctorReportDTO | null>(null)
  const [writeProbeConfirm, setWriteProbeConfirm] = useState<boolean>(false)

  const handleLogin = async () => {
    setAuthFormError(null)
    setAuthMessage(null)
    setSigningIn(true)
    try {
      const st = await backend.auth.login(selectedRegion)
      onSetAuth(st)
      if (st.region) onSetSelectedRegion(st.region)
    } catch (err) {
      if (typeof err === "object" && err !== null && "code" in err && err.code === "INTERRUPTED") {
        setAuthMessage(m.signInCancelled())
      } else setAuthFormError(errorMessageFrom(err))
    } finally {
      setSigningIn(false)
    }
  }

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
      setSettingsSavedMsg(m.settingsSaved())
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
            <h2 className="text-lg font-semibold text-slate-100">{m.accountAndSession()}</h2>
            <p className="text-xs text-slate-400">
              {m.chooseYourAmazonMarketplaceThenSignInOnAmazonSWebsiteYourPasswordAndVerificationCodesStayWithAmazon()}
            </p>
          </div>
          <div className="flex items-center gap-2 text-xs">
            {auth?.authenticated ? (
              <>
                <span className="px-2.5 py-1 rounded bg-emerald-500/20 text-emerald-300 font-medium">
                  {m.signedIn()}
                  {(auth.region || selectedRegion || "us").toUpperCase()} ·{" "}
                  {auth.evidence || "verified"})
                </span>
                <button
                  type="button"
                  disabled={signingIn}
                  onClick={() => void handleLogout()}
                  className="px-3 py-1 rounded bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 font-medium"
                >
                  {m.signOut()}
                </button>
              </>
            ) : (
              <span className="px-2.5 py-1 rounded bg-amber-500/20 text-amber-300 font-medium">
                {m.notSignedIn()}
              </span>
            )}
          </div>
        </div>

        {authFormError && (
          <div
            role="alert"
            className="rounded bg-rose-950/60 border border-rose-800 px-3 py-2 text-xs text-rose-200"
          >
            {authFormError}
          </div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
          <div>
            <label htmlFor="settings-region-select" className="block text-slate-300 mb-1">
              {m.marketplaceRegion()}
            </label>
            <select
              disabled={signingIn}
              id="settings-region-select"
              aria-label="Marketplace Region"
              value={selectedRegion}
              onChange={(e) => onSetSelectedRegion(e.target.value)}
              className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
            >
              <option value="">{m.selectRegion()}</option>
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

          <div className="sm:col-span-2 flex items-end gap-3">
            {signingIn ? (
              <>
                <p role="status" className="text-slate-300">
                  {m.completeSignInInTheAmazonWindow()}
                </p>
                <button
                  type="button"
                  onClick={() =>
                    void backend.auth
                      .cancelLogin()
                      .catch((err) => setAuthFormError(errorMessageFrom(err)))
                  }
                  className="px-4 py-2 rounded bg-slate-800"
                >
                  {m.cancelSignIn()}
                </button>
              </>
            ) : (
              <button
                type="button"
                disabled={!selectedRegion}
                onClick={() => void handleLogin()}
                className="px-4 py-2 rounded bg-emerald-600 text-white disabled:opacity-40"
              >
                {m.signInToAmazon()}
              </button>
            )}
          </div>
        </div>
        {authMessage && (
          <p role="status" className="text-xs text-slate-300">
            {authMessage}
          </p>
        )}
        {auth?.evidence === "experimental" && (
          <p className="text-xs text-amber-300">{m.experimental()}</p>
        )}
        <details className="text-xs text-slate-400">
          <summary className="cursor-pointer">{m.importSessionCookiesAdvanced()}</summary>
          <fieldset disabled={signingIn} className="space-y-3 pt-3">
            <label htmlFor="settings-cookie-input">
              {m.sessionCookiesRawCookieHeaderNetscapeCookiesTxtOrJson()}
            </label>
            <textarea
              id="settings-cookie-input"
              aria-label="Session Cookies"
              rows={2}
              value={cookieInput}
              onChange={(e) => setCookieInput(e.target.value)}
              className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 font-mono"
            />
            <button
              type="button"
              onClick={() => void handlePickAndImportCookieFile()}
              className="px-3 py-2 rounded bg-slate-800 mr-2"
            >
              {m.importCookieFile()}
            </button>
            <button
              type="button"
              onClick={() => void handleImportPastedCookies()}
              className="px-3 py-2 rounded bg-slate-800"
            >
              {m.importPastedCookies()}
            </button>
          </fieldset>
        </details>
      </section>

      <section
        data-testid="settings-storage-usage"
        className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-3"
      >
        <h2 className="text-lg font-semibold text-slate-100">{m.storageUsageBreakdown()}</h2>
        {usage ? (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
            <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
              <div className="text-slate-400">{m.photos()}</div>
              <div className="text-sm font-semibold text-slate-100">
                {m.itemCount(usage.photo.total.count, formatBytes(usage.photo.total.bytes))}
              </div>
            </div>
            <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
              <div className="text-slate-400">{m.videos()}</div>
              <div className="text-sm font-semibold text-slate-100">
                {m.itemCount(usage.video.total.count, formatBytes(usage.video.total.bytes))}
              </div>
            </div>
            <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
              <div className="text-slate-400">{m.totalUsed()}</div>
              <div className="text-sm font-semibold text-slate-100">
                {formatBytes(usage.totalBytes)}
              </div>
            </div>
            <div className="rounded bg-slate-950/70 p-3 border border-slate-800">
              <div className="text-slate-400">{m.billableUsed()}</div>
              <div className="text-sm font-semibold text-slate-100">
                {formatBytes(usage.totalBillableBytes)}
              </div>
            </div>
          </div>
        ) : (
          <p className="text-xs text-slate-400">{m.signInToAmazonToViewStorageUsageBreakdown()}</p>
        )}
      </section>

      <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-100">{m.transferAndDesktopBehavior()}</h2>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
          <div>
            <label htmlFor="settings-metadata-concurrency" className="block text-slate-300 mb-1">
              {m.metadataConcurrency116()}
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
              {m.uploadConcurrency18()}
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
              {m.defaultConflictPolicy()}
            </label>
            <select
              id="settings-default-conflict"
              aria-label="Default Conflict Policy"
              value={defaultConflictPolicy}
              onChange={(e) => onSetDefaultConflictPolicy(e.target.value)}
              className="w-full rounded bg-slate-950 border border-slate-700 px-3 py-1.5 text-slate-100"
            >
              <option value="skip">{m.skip()}</option>
              <option value="rename">{m.rename()}</option>
              <option value="fail">{m.fail()}</option>
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
            <span>{m.closeWindowToSystemTray()}</span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer text-slate-200">
            <input
              type="checkbox"
              checked={launchAtLogin}
              onChange={(e) => onSetLaunchAtLogin(e.target.checked)}
            />
            <span>{m.launchAtLogin()}</span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer text-slate-200">
            <input
              type="checkbox"
              checked={notificationsEnabled}
              onChange={(e) => onSetNotificationsEnabled(e.target.checked)}
            />
            <span>{m.desktopNotifications()}</span>
          </label>
        </div>

        <div
          data-testid="lifecycle-guide"
          className="rounded-lg border border-slate-800 bg-slate-950/70 p-3.5 space-y-1.5 text-xs text-slate-300"
        >
          <div className="font-semibold text-slate-200">
            {m.desktopLifecycleAndActionReference()}
          </div>
          <ul className="space-y-1 text-slate-400 list-disc list-inside">
            <li>
              <strong className="text-slate-200">{m.pauseTask()}</strong>
              {m.pausesOneActiveUploadOrDownloadTransferInTheQueueOtherTransfersAndBackupsContinue()}
            </li>
            <li>
              <strong className="text-slate-200">{m.stopBackups()}</strong>
              {m.haltsBackgroundFolderScansAcrossAllConfiguredBackupRulesUntilResumed()}
            </li>
            <li>
              <strong className="text-slate-200">{m.closeWindow()}</strong>
              {m.hidesTheWindowToTheSystemTrayWhenEnabledWhileActiveTransfersAndBackupsContinueRunningInTheBackground()}
            </li>
            <li>
              <strong className="text-slate-200">{m.quitRainforest()}</strong>
              {m.safelyStopsBackgroundWorkersAndExitsTheApplicationCompletely()}
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
            {m.saveSettings()}
          </button>
        </div>
      </section>

      <section className="rounded-xl border border-slate-800 bg-slate-900/50 p-5 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold text-slate-100">{m.doctorDiagnostics()}</h2>
            <p className="text-xs text-slate-400">
              {m.gradedConnectivityCredentialVaultRegionRoutingReadProbeAndOptInWriteProbeDiagnostics()}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-2 text-xs text-slate-300 cursor-pointer">
              <input
                type="checkbox"
                checked={writeProbeConfirm}
                onChange={(e) => setWriteProbeConfirm(e.target.checked)}
              />
              <span>{m.confirmTemporaryFolderWriteProbeGrade5()}</span>
            </label>
            <button
              type="button"
              onClick={() => void handleRunDoctor()}
              className="px-3.5 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-100 border border-slate-700"
            >
              {m.runDoctor()}
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
                    {m.grade()}
                    {chk.grade}] {chk.name}
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
                {chk.hint && (
                  <p className="text-amber-300">
                    {m.hint()}
                    {chk.hint}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      <section
        data-testid="about-disclaimer"
        className="rounded-xl border border-slate-800 bg-slate-900/30 p-4 text-xs text-slate-400 space-y-1"
      >
        <div className="font-semibold text-slate-300">{m.aboutRainforest()}</div>
        <p>
          {m.rainforestIsAnIndependentUnofficialClientAndIsNotAffiliatedWithOrEndorsedByAmazonComInc()}
        </p>
        <p>{m.licenseAgpl30Only()}</p>
      </section>
    </div>
  )
}
