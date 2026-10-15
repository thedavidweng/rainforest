import { afterEach, describe, expect, test } from "bun:test"
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"

import { App } from "@/App"
import { memoryBackend } from "@/testing/memory-backend"

afterEach(() => {
  cleanup()
})

describe("Rainforest App — Scope & Navigation (Spec §H.2 & §H.3)", () => {
  test("renders strictly the 4 navigation views and excludes cloud gallery, AI search, album CRUD, and trash UI", async () => {
    const backend = memoryBackend()
    render(<App backend={backend} />)

    await waitFor(() => {
      expect(screen.getByRole("navigation", { name: /main navigation/i })).toBeTruthy()
    })

    const navButtons = screen
      .getByRole("navigation", { name: /main navigation/i })
      .querySelectorAll("button")
    const labels = Array.from(navButtons).map((b) => b.textContent?.trim())
    expect(labels).toEqual(["Transfers / Upload", "Backups", "Downloads", "Settings"])

    // Verify no out-of-scope gallery, timeline, AI search, people/places, album CRUD, or trash tabs exist
    expect(screen.queryByRole("button", { name: /photo wall|gallery|timeline/i })).toBeNull()
    expect(screen.queryByRole("button", { name: /ai search|people|places|memories/i })).toBeNull()
    expect(screen.queryByRole("button", { name: /trash|recycle bin|purge/i })).toBeNull()
    expect(screen.queryByRole("button", { name: /create album|manage albums/i })).toBeNull()

    // Compact account status & storage summary banner in default Transfers / Upload view
    await waitFor(() => {
      expect(screen.getByTestId("account-banner")).toBeTruthy()
      expect(screen.getByTestId("account-banner").textContent).toContain("US")
      expect(screen.getByTestId("account-banner").textContent).toContain("verified")
      expect(screen.getByTestId("account-banner").textContent).toContain("1,420 photos")
      expect(screen.getByTestId("account-banner").textContent).toContain("48 videos")
    })
  })

  test("prompts unauthenticated users to import session cookies in Settings", async () => {
    const backend = memoryBackend({
      auth: {
        configured: false,
        authenticated: false,
        region: "",
        errorMessage: "Session cookies expired or not configured",
      },
    })
    render(<App backend={backend} />)

    await waitFor(() => {
      expect(screen.getByTestId("account-banner").textContent).toMatch(/import.*cookie/i)
    })

    fireEvent.click(screen.getByRole("button", { name: /open settings|import cookies/i }))

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /account & session/i })).toBeTruthy()
    })
  })
})

describe("View 1: Transfers / Upload — Dropzone, Preflight Confirmation, and Transfer Queue", () => {
  test("handles drag-and-drop -> preflight summary -> media filter & conflict policy -> confirm upload -> active progress -> pause/resume/cancel/retry/unconfirmed/clear", async () => {
    const backend = memoryBackend({
      localFiles: [
        {
          path: "/Users/alice/Photos/IMG_2001.JPG",
          name: "IMG_2001.JPG",
          mediaType: "photo",
          size: 3_145_728,
          width: 4032,
          height: 3024,
        },
        {
          path: "/Users/alice/Photos/CLIP_2002.MP4",
          name: "CLIP_2002.MP4",
          mediaType: "video",
          size: 10_485_760,
          width: 1920,
          height: 1080,
        },
        {
          path: "/Users/alice/Photos/readme.txt",
          name: "readme.txt",
          mediaType: "other",
          size: 1024,
        },
      ],
    })

    render(<App backend={backend} />)

    await waitFor(() => {
      expect(screen.getByTestId("account-banner").textContent).toContain("Connected")
      expect(screen.getByTestId("upload-dropzone")).toBeTruthy()
      expect(screen.getByRole("button", { name: /select files/i })).toBeTruthy()
      expect(screen.getByRole("button", { name: /select folder/i })).toBeTruthy()
    })

    // Emit native file drop event
    act(() => {
      backend.emitFilesDropped([
        "/Users/alice/Photos/IMG_2001.JPG",
        "/Users/alice/Photos/CLIP_2002.MP4",
        "/Users/alice/Photos/readme.txt",
      ])
    })

    // Pre-upload confirmation modal appears
    const modal = await screen.findByTestId("upload-plan-modal")
    expect(modal.textContent).toContain("IMG_2001.JPG")
    expect(modal.textContent).toContain("CLIP_2002.MP4")
    expect(modal.textContent).toContain("1 photo")
    expect(modal.textContent).toContain("1 video")
    expect(modal.textContent).toContain("1 skipped")
    expect(modal.textContent).toContain("Account root")
    // Local thumbnail preview rendered for photo
    expect(modal.querySelector("img[alt='IMG_2001.JPG']")).toBeTruthy()

    // Toggle media filter to Photos Only -> replans without video
    fireEvent.click(screen.getByRole("button", { name: /photos only/i }))
    await screen.findByText("0 videos")
    expect(screen.getByTestId("upload-plan-modal").textContent).toContain("2 skipped")

    // Toggle back to Photos + Videos and change conflict policy to rename
    fireEvent.click(screen.getByRole("button", { name: /photos \+ videos/i }))
    await screen.findByText("1 video")
    fireEvent.change(screen.getByLabelText(/conflict policy/i), {
      target: { value: "rename" },
    })
    expect((screen.getByLabelText(/conflict policy/i) as HTMLSelectElement).value).toBe("rename")

    // Confirm & Start Upload
    await waitFor(() =>
      expect(
        (screen.getByRole("button", { name: /confirm & start upload/i }) as HTMLButtonElement)
          .disabled,
      ).toBe(false),
    )
    fireEvent.click(screen.getByRole("button", { name: /confirm & start upload/i }))

    await screen.findByTestId("transfer-card-tr-1")
    expect(screen.queryByTestId("upload-plan-modal")).toBeNull()
    expect(screen.getByTestId("active-transfers").textContent).toContain("running")

    // Expand per-item details
    fireEvent.click(screen.getByRole("button", { name: /details/i }))
    await screen.findByText("IMG_2001.JPG")
    expect(screen.getByText("CLIP_2002.MP4")).toBeTruthy()

    // Live byte progress event updates progress bar and item status
    act(() => {
      backend.emitTransferProgress({
        transfer: {
          id: "tr-1",
          kind: "upload",
          stage: "running",
          sourceSummary: "IMG_2001.JPG +2 more",
          conflictPolicy: "rename",
          bytesDone: 6_815_744,
          bytesTotal: 13_631_488,
          itemsDone: 1,
          itemsSkipped: 0,
          itemsFailed: 0,
          itemsUnconfirmed: 0,
          itemsTotal: 2,
          createdAt: "2026-10-14T12:00:00Z",
          updatedAt: "2026-10-14T12:00:05Z",
        },
        item: {
          id: "tr-1-item-1",
          transferId: "tr-1",
          localPath: "/Users/alice/Photos/IMG_2001.JPG",
          name: "IMG_2001.JPG",
          mediaType: "photo",
          size: 3_145_728,
          remoteNodeId: "node-img-2001",
          status: "confirmed",
          bytesDone: 3_145_728,
          updatedAt: "2026-10-14T12:00:05Z",
        },
      })
    })
    await screen.findByText(/50%/)
    expect(screen.getByTestId("active-transfers").textContent).toContain("1 confirmed")

    // Pause -> Resume -> Cancel -> Retry
    fireEvent.click(screen.getByRole("button", { name: /^pause$/i }))
    await screen.findByText("paused")
    expect(screen.getByTestId("active-transfers").textContent).toContain("paused")

    fireEvent.click(screen.getByRole("button", { name: /^resume$/i }))
    await screen.findByText("running")

    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }))
    await screen.findByText("cancelled")
    expect(screen.getByTestId("history-transfers").textContent).toContain("cancelled")

    fireEvent.click(screen.getByRole("button", { name: /^retry$/i }))
    await screen.findByText("running")
    expect(screen.getByTestId("active-transfers").textContent).toContain("running")

    // Simulate transition to unconfirmed state
    act(() => {
      backend.completeTransferForTest("tr-1", "unconfirmed")
    })
    await screen.findByText("1 unconfirmed")
    expect(screen.getByTestId("history-transfers").textContent).toContain("unconfirmed")

    // Simulate transition to failed state with error banner
    act(() => {
      backend.completeTransferForTest("tr-1", "failed")
    })
    await screen.findByText("upload connection interrupted")
    expect(screen.getByTestId("history-transfers").textContent).toContain("failed")

    // Clear History removes terminal transfers
    fireEvent.click(screen.getByRole("button", { name: /clear history/i }))
    await screen.findByText("No completed or past transfers.")
  })

  test("supports Select Files and Select Folder buttons and Cancel on preflight modal", async () => {
    const backend = memoryBackend({
      pickedFiles: ["/Users/alice/Photos/IMG_1001.JPG"],
    })
    render(<App backend={backend} />)

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /select files/i })).toBeTruthy()
    })

    fireEvent.click(screen.getByRole("button", { name: /select files/i }))
    await waitFor(() => {
      expect(screen.getByTestId("upload-plan-modal")).toBeTruthy()
      expect(screen.getByTestId("upload-plan-modal").textContent).toContain("IMG_1001.JPG")
    })

    // Cancel dismisses the preflight modal without starting a transfer
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }))
    await waitFor(() => {
      expect(screen.queryByTestId("upload-plan-modal")).toBeNull()
    })
    // Select Folder button also opens preflight modal
    fireEvent.click(screen.getByRole("button", { name: /select folder/i }))
    await screen.findByTestId("upload-plan-modal")
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }))
    const list = await backend.transfers.list()
    expect(list.active).toHaveLength(0)
  })
})

describe("View 2: Backups — One-Way Automatic Folder Backup", () => {
  test("renders one-way banner, supports Add/Edit/Pause/Resume/Scan Now/Remove, Stop All toggle, and source_unavailable warning", async () => {
    const backend = memoryBackend({
      pickedDirectory: "/Volumes/SDCard/DCIM",
    })
    render(<App backend={backend} />)

    await screen.findByRole("button", { name: "Backups" })
    fireEvent.click(screen.getByRole("button", { name: "Backups" }))

    // Explicit one-way backup banner
    const banner = await screen.findByTestId("one-way-backup-banner")
    expect(banner.textContent).toContain(
      "One-way incremental backup: deleting, moving, or renaming local files never deletes your Amazon Photos cloud copies.",
    )

    // Stop All Backups / Resume All Backups toggle
    const stopAllBtn = screen.getByRole("button", { name: /stop all backups/i })
    fireEvent.click(stopAllBtn)
    await screen.findByRole("button", { name: /resume all backups/i })
    fireEvent.click(screen.getByRole("button", { name: /resume all backups/i }))
    await screen.findByRole("button", { name: /stop all backups/i })

    // Open Add Backup Folder modal
    fireEvent.click(screen.getByRole("button", { name: /add backup folder/i }))
    const dialog = await screen.findByRole("dialog", { name: /add backup folder/i })
    expect(dialog).toBeTruthy()

    // Use Pick Folder button
    fireEvent.click(screen.getByRole("button", { name: /pick folder/i }))
    await waitFor(() => {
      expect((screen.getByLabelText(/local folder path/i) as HTMLInputElement).value).toBe(
        "/Volumes/SDCard/DCIM",
      )
    })

    // Configure Photos Only, Scheduled Interval (45 min), conflict policy rename
    fireEvent.click(screen.getByRole("button", { name: /photos only/i }))
    fireEvent.click(screen.getByRole("button", { name: /scheduled interval/i }))
    fireEvent.change(screen.getByLabelText(/interval \(minutes\)/i), {
      target: { value: "45" },
    })
    fireEvent.change(screen.getByLabelText(/conflict policy/i), {
      target: { value: "rename" },
    })

    fireEvent.click(screen.getByRole("button", { name: /save backup rule/i }))

    // Verify added rule appears in Backup Rules List
    const ruleCard = await screen.findByTestId("backup-rule-backup-1")
    expect(ruleCard.textContent).toContain("/Volumes/SDCard/DCIM")
    expect(ruleCard.textContent).toContain("active")
    expect(ruleCard.textContent).toContain("Photos Only")
    expect(ruleCard.textContent).toContain("45m")

    // Pause -> Resume
    fireEvent.click(screen.getByRole("button", { name: /^pause$/i }))
    await screen.findByText("paused")
    fireEvent.click(screen.getByRole("button", { name: /^resume$/i }))
    await screen.findByText("active")

    // Edit rule -> change to Photos + Videos
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }))
    await screen.findByRole("dialog", { name: /edit backup folder/i })
    fireEvent.click(screen.getByRole("button", { name: /photos \+ videos/i }))
    fireEvent.click(screen.getByRole("button", { name: /save backup rule/i }))
    await screen.findByText("Photos + Videos")

    // Scan Now
    fireEvent.click(screen.getByRole("button", { name: /scan now/i }))
    await waitFor(() => {
      expect(screen.getByTestId("backup-rule-backup-1").textContent).toContain("Last scan:")
    })

    // Simulate external drive going offline -> source_unavailable warning
    act(() => {
      backend.setBackupPathUnavailableForTest("/Volumes/SDCard/DCIM", true)
    })
    await screen.findByText("source_unavailable")
    expect(screen.getByTestId("backup-rule-backup-1").textContent).toContain(
      "External drive or folder is currently offline — cloud files are untouched",
    )

    // Remove rule
    fireEvent.click(screen.getByRole("button", { name: /^remove$/i }))
    await screen.findByText(/no backup folders configured/i)
  })
})

describe("View 3: Downloads — Lightweight Recovery Download", () => {
  test("supports recovery download from confirmed history, remote album/folder, and explicit node IDs with verified status", async () => {
    const backend = memoryBackend({
      pickedDirectory: "/Users/alice/RestoredPhotos",
    })
    render(<App backend={backend} />)

    await screen.findByRole("button", { name: "Downloads" })
    fireEvent.click(screen.getByRole("button", { name: "Downloads" }))

    // Regional capability notice
    const notice = await screen.findByTestId("recovery-capability-notice")
    expect(notice.textContent).toContain("US")
    expect(notice.textContent).toContain("verified")
    expect(notice.textContent).toContain(
      "Recovery download supports confirmed transfer history items",
    )

    // Pick target directory
    fireEvent.click(screen.getByRole("button", { name: /pick directory/i }))
    await waitFor(() => {
      expect((screen.getByLabelText(/target local directory/i) as HTMLInputElement).value).toBe(
        "/Users/alice/RestoredPhotos",
      )
    })

    // Select confirmed transfer history item (IMG_0042.JPG)
    const checkbox = await screen.findByRole("checkbox", { name: /IMG_0042\.JPG/i })
    fireEvent.click(checkbox)

    fireEvent.click(screen.getByRole("button", { name: /start recovery download/i }))

    const resultSection = await screen.findByTestId("recovery-download-results")
    expect(resultSection.textContent).toContain("IMG_0042.JPG")
    expect(resultSection.textContent).toContain("confirmed")
    expect(resultSection.textContent).toContain("node-confirmed-1")

    // Switch to Explicit Remote Node IDs mode and download node1, node2
    fireEvent.click(screen.getByRole("button", { name: /explicit remote node ids/i }))
    fireEvent.change(screen.getByLabelText(/remote node ids/i), {
      target: { value: "node-alpha, node-beta" },
    })
    fireEvent.click(screen.getByRole("button", { name: /start recovery download/i }))

    await screen.findByText("node-alpha.jpg")
    expect(screen.getByTestId("recovery-download-results").textContent).toContain("node-beta.jpg")

    // Switch to Remote Albums / Folders mode and recover album-family
    fireEvent.click(screen.getByRole("button", { name: /remote albums \/ folders/i }))
    fireEvent.change(screen.getByLabelText(/select remote folder or album/i), {
      target: { value: "album-family" },
    })
    fireEvent.click(screen.getByRole("button", { name: /start recovery download/i }))
    await screen.findByText("recovered_album-family.jpg")
  })
  test("disables an empty remote selector and retains experimental evidence notice", async () => {
    const backend = memoryBackend({
      recoverySources: {
        region: "uk",
        evidence: "experimental",
        folders: [],
        albums: [],
        capabilityNotes:
          "Experimental region: remote folder/album listing is unverified; recovery from confirmed transfer history or explicit node IDs is recommended.",
      },
    })
    render(<App backend={backend} />)

    await screen.findByRole("button", { name: "Downloads" })
    fireEvent.click(screen.getByRole("button", { name: "Downloads" }))

    const notice = await screen.findByTestId("recovery-capability-notice")
    expect(notice.textContent).toContain("UK")
    expect(notice.textContent).toContain("experimental")

    const containerBtn = screen.getByRole("button", { name: /remote albums \/ folders/i })
    expect((containerBtn as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByTestId("downloads-view").textContent).toMatch(
      /remote folder\/album listing is unverified/i,
    )
  })
})

describe("View 4: Settings & Diagnostics", () => {
  test("supports cookie import, explicit region selection (never silent .com), sign out, desktop behavior controls, doctor diagnostics, and disclaimer", async () => {
    const backend = memoryBackend({
      auth: { configured: false, authenticated: false, region: "" },
      settings: { region: "" },
      doctor: {
        healthy: false,
        region: "ca",
        evidence: "experimental",
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
            message: "Region ca (experimental) configured.",
          },
          {
            grade: 3,
            name: "account_root_and_usage",
            status: "warn",
            ok: false,
            message: "Rate limited by Amazon Photos (HTTP 429).",
            hint: "Rate limited by Amazon Photos (HTTP 429). Wait for the Retry-After window and reduce concurrency.",
          },
          {
            grade: 4,
            name: "search_read_probe",
            status: "fail",
            ok: false,
            message: "Session cookies rejected (HTTP 401).",
            hint: "Session cookies were rejected (HTTP 401). Export fresh cookies from an active browser session and import them in Settings.",
          },
        ],
      },
    })

    render(<App backend={backend} />)

    await screen.findByRole("button", { name: "Settings" })
    fireEvent.click(screen.getByRole("button", { name: "Settings" }))

    await screen.findByRole("heading", { name: /account & session/i })

    // Verify explicit region options include US, CA, UK, JP and never auto-default silently to US when unconfigured
    const regionSelect = screen.getByLabelText(/marketplace region/i) as HTMLSelectElement
    expect(regionSelect.value).toBe("")
    const optionValues = Array.from(regionSelect.options).map((o) => o.value)
    expect(optionValues).toEqual(["", "us", "ca", "uk", "jp"])

    // Attempt cookie import with ambiguous cookies and no region selected -> shows explicit region error
    fireEvent.change(screen.getByLabelText(/session cookies/i), {
      target: { value: "session-id=135-9999999-9999999" },
    })
    fireEvent.click(screen.getByRole("button", { name: /import pasted cookies/i }))
    await screen.findByText(/region cannot be determined from cookies/i)

    // Select explicit region CA and import cookies
    fireEvent.change(regionSelect, { target: { value: "ca" } })
    fireEvent.change(screen.getByLabelText(/session cookies/i), {
      target: {
        value: "session-id=135-9999999-9999999; ubid-acbca=133-111; at-acbca=Atza|ca",
      },
    })
    fireEvent.click(screen.getByRole("button", { name: /import pasted cookies/i }))

    await screen.findByText(/authenticated \(CA/i)

    // Sign Out
    fireEvent.click(screen.getByRole("button", { name: /sign out/i }))
    await screen.findByText(/not authenticated/i)

    // Import Cookie File button
    fireEvent.change(regionSelect, { target: { value: "us" } })
    fireEvent.click(screen.getByRole("button", { name: /import cookie file/i }))
    await screen.findByText(/authenticated \(US/i)
    await screen.findByText(/1,420 items/i)

    // Transfer & Desktop Behavior settings update
    fireEvent.change(screen.getByLabelText(/metadata concurrency/i), {
      target: { value: "6" },
    })
    fireEvent.change(screen.getByLabelText(/upload concurrency/i), {
      target: { value: "3" },
    })
    fireEvent.change(screen.getByLabelText(/default conflict policy/i), {
      target: { value: "rename" },
    })
    fireEvent.click(screen.getByLabelText(/close window to system tray/i))
    fireEvent.click(screen.getByLabelText(/launch at login/i))
    fireEvent.click(screen.getByLabelText(/desktop notifications/i))
    fireEvent.click(screen.getByRole("button", { name: /save settings/i }))
    await screen.findByText("Settings saved.")

    const savedCfg = await backend.settings.get()
    expect(savedCfg.maxMetadataConcurrency).toBe(6)
    expect(savedCfg.maxUploadConcurrency).toBe(3)
    expect(savedCfg.defaultConflictPolicy).toBe("rename")
    expect(savedCfg.closeToTray).toBe(false)
    expect(savedCfg.launchAtLogin).toBe(true)
    expect(savedCfg.notificationsEnabled).toBe(false)

    // Verify lifecycle distinction explanation (Pause Task vs Stop Backups vs Close Window vs Quit Rainforest)
    const lifecycleBox = screen.getByTestId("lifecycle-guide")
    expect(lifecycleBox.textContent).toContain("Pause Task")
    expect(lifecycleBox.textContent).toContain("Stop Backups")
    expect(lifecycleBox.textContent).toContain("Close Window")
    expect(lifecycleBox.textContent).toContain("Quit Rainforest")

    // Run Doctor diagnostics
    fireEvent.click(screen.getByRole("button", { name: /run doctor/i }))
    const doctorSection = await screen.findByTestId("doctor-report")
    expect(doctorSection.textContent).toContain("vault_permissions")
    expect(doctorSection.textContent).toContain("account_root_and_usage")
    expect(doctorSection.textContent).toContain("HTTP 429")
    expect(doctorSection.textContent).toContain("HTTP 401")

    // Verify About / Disclaimer
    const disclaimer = screen.getByTestId("about-disclaimer")
    expect(disclaimer.textContent).toContain(
      "Rainforest is an independent, unofficial client and is not affiliated with or endorsed by Amazon.com, Inc.",
    )
    expect(disclaimer.textContent).toContain("AGPL-3.0-only")
  })
})

describe("Runtime destination probe", () => {
  test("preserves custom destinations in experimental regions and displays read-only status", async () => {
    const backend = memoryBackend({
      auth: { region: "ca", evidence: "experimental" },
      settings: { region: "ca" },
      destinations: { "folder-ca": { name: "Canadian photos", kind: "FOLDER" } },
    })
    render(<App backend={backend} />)
    await screen.findByTestId("account-banner")
    act(() => backend.emitFilesDropped(["/Pictures/photo.jpg"]))
    await screen.findByTestId("upload-plan-modal")
    fireEvent.change(screen.getByLabelText("Destination Node ID"), {
      target: { value: "folder-ca" },
    })
    await waitFor(() =>
      expect(screen.getByTestId("upload-plan-modal").textContent).toContain("readable"),
    )
    expect(screen.getByTestId("upload-plan-modal").textContent).toContain("FOLDER")
    expect(screen.getByTestId("account-banner").textContent).toContain("experimental")
    fireEvent.click(screen.getByRole("button", { name: /confirm & start upload/i }))
    await waitFor(async () =>
      expect((await backend.transfers.list()).active[0]?.destNodeId).toBe("folder-ca"),
    )
  })
  test("rechecks a pending upload target after reconnecting", async () => {
    const backend = memoryBackend()
    const planUpload = backend.transfers.planUpload
    let connected = false
    backend.transfers.planUpload = async (input) => {
      const plan = await planUpload(input)
      return connected
        ? plan
        : {
            ...plan,
            destination: { ...plan.destination, status: "pending", message: "Network unavailable" },
          }
    }
    render(<App backend={backend} />)
    await screen.findByTestId("account-banner")
    act(() => backend.emitFilesDropped(["/Pictures/photo.jpg"]))
    await waitFor(() =>
      expect(screen.getByTestId("upload-plan-modal").textContent).toContain("Network unavailable"),
    )
    const confirm = screen.getByRole("button", {
      name: /confirm & start upload/i,
    }) as HTMLButtonElement
    expect(confirm.disabled).toBe(true)
    connected = true
    fireEvent.click(screen.getByRole("button", { name: /check destination/i }))
    await waitFor(() => expect(confirm.disabled).toBe(false))
  })
  test("invalidates readable targets when the ID or account changes", async () => {
    const backend = memoryBackend({ destinations: { folder: { name: "Folder", kind: "FOLDER" } } })
    render(<App backend={backend} />)
    await screen.findByTestId("account-banner")
    act(() => backend.emitFilesDropped(["/Pictures/photo.jpg"]))
    const confirm = await screen.findByRole("button", { name: /confirm & start upload/i })
    await waitFor(() => expect((confirm as HTMLButtonElement).disabled).toBe(false))
    fireEvent.change(screen.getByLabelText("Destination Node ID"), { target: { value: "missing" } })
    expect((confirm as HTMLButtonElement).disabled).toBe(true)
    await waitFor(() =>
      expect(screen.getByTestId("upload-plan-modal").textContent).toContain("failed"),
    )
    fireEvent.change(screen.getByLabelText("Destination Node ID"), { target: { value: "folder" } })
    await waitFor(() => expect((confirm as HTMLButtonElement).disabled).toBe(false))
    await act(async () => {
      await backend.auth.logout()
    })
    expect((confirm as HTMLButtonElement).disabled).toBe(true)
    await waitFor(() =>
      expect(screen.getByTestId("upload-plan-modal").textContent).toContain("pending"),
    )
  })
  test("saves and edits a pending backup destination without credentials", async () => {
    const backend = memoryBackend({ auth: { configured: false, authenticated: false } })
    render(<App backend={backend} />)
    fireEvent.click(await screen.findByRole("button", { name: "Backups" }))
    fireEvent.click(screen.getByRole("button", { name: /add backup folder/i }))
    fireEvent.change(screen.getByLabelText("Local Folder Path"), { target: { value: "/Pictures" } })
    fireEvent.change(screen.getByLabelText("Destination Node ID"), {
      target: { value: "album-ca" },
    })
    fireEvent.click(screen.getByRole("button", { name: /check destination/i }))
    await waitFor(() => expect(screen.getByRole("dialog").textContent).toContain("pending"))
    fireEvent.click(screen.getByRole("button", { name: /save backup rule/i }))
    await screen.findByTestId("backup-rule-backup-1")
    expect((await backend.backups.list())[0]?.destinationNodeId).toBe("album-ca")
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }))
    expect((screen.getByLabelText("Destination Node ID") as HTMLInputElement).value).toBe(
      "album-ca",
    )
    await waitFor(() => expect(screen.getByRole("dialog").textContent).toContain("pending"))
  })
})
