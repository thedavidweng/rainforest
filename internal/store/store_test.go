package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/store"
	_ "modernc.org/sqlite"
)

func TestStoreSettingsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "state.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()

	def, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings default: %v", err)
	}
	if def.MaxMetadataConcurrency != 4 || def.MaxUploadConcurrency != 2 {
		t.Errorf("default concurrencies = (%d, %d), want (4, 2)", def.MaxMetadataConcurrency, def.MaxUploadConcurrency)
	}
	if def.DefaultConflictPolicy != aphotos.ConflictSkip {
		t.Errorf("default conflict policy = %q, want %q", def.DefaultConflictPolicy, aphotos.ConflictSkip)
	}
	if !def.CloseToTray || !def.NotificationsEnabled || def.BackupsStopped {
		t.Errorf("unexpected default flags: %+v", def)
	}

	updated := store.Settings{
		Region:                 "ca",
		MaxMetadataConcurrency: 3,
		MaxUploadConcurrency:   1,
		DefaultConflictPolicy:  aphotos.ConflictRename,
		LaunchAtLogin:          true,
		CloseToTray:            false,
		NotificationsEnabled:   false,
		BackupsStopped:         true,
	}
	if err := st.UpdateSettings(ctx, updated); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	got, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}
	if got != updated {
		t.Errorf("got settings %+v, want %+v", got, updated)
	}
}

func TestStoreBackupRulesAndTrackedFiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "state.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()

	now := time.Now().UTC().Truncate(time.Second)
	rule, err := st.CreateBackupRule(ctx, store.BackupRule{
		ID:                "rule-1",
		LocalPath:         "/Users/test/Pictures",
		IncludeVideos:     true,
		ScheduleMode:      store.ScheduleInterval,
		IntervalMinutes:   30,
		ConflictPolicy:    aphotos.ConflictSkip,
		DestinationNodeID: "root-id",
		Paused:            false,
		Status:            store.BackupStatusActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateBackupRule: %v", err)
	}
	if rule.ID != "rule-1" || !rule.IncludeVideos || rule.IntervalMinutes != 30 {
		t.Fatalf("unexpected created rule: %+v", rule)
	}

	tf := store.TrackedFile{
		BackupRuleID: "rule-1",
		RelPath:      "2026/IMG_0001.JPG",
		AbsPath:      "/Users/test/Pictures/2026/IMG_0001.JPG",
		Size:         123456,
		MtimeNs:      1700000000123456789,
		MD5:          "d41d8cd98f00b204e9800998ecf8427e",
		RemoteNodeID: "node-abc",
		Status:       store.ItemStatusConfirmed,
		LastSyncedAt: now,
	}
	if err := st.UpsertTrackedFile(ctx, tf); err != nil {
		t.Fatalf("UpsertTrackedFile: %v", err)
	}

	gotTF, err := st.GetTrackedFile(ctx, "rule-1", "2026/IMG_0001.JPG")
	if err != nil {
		t.Fatalf("GetTrackedFile: %v", err)
	}
	if gotTF.RemoteNodeID != "node-abc" || gotTF.Size != 123456 || gotTF.MtimeNs != 1700000000123456789 {
		t.Errorf("unexpected tracked file: %+v", gotTF)
	}

	files, err := st.ListTrackedFiles(ctx, "rule-1")
	if err != nil {
		t.Fatalf("ListTrackedFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}

	scanTime := now.Add(5 * time.Minute)
	if err := st.SetBackupRuleStatus(ctx, "rule-1", store.BackupStatusSourceUnavailable, "mount missing", &scanTime); err != nil {
		t.Fatalf("SetBackupRuleStatus: %v", err)
	}
	updatedRule, err := st.GetBackupRule(ctx, "rule-1")
	if err != nil {
		t.Fatalf("GetBackupRule: %v", err)
	}
	if updatedRule.Status != store.BackupStatusSourceUnavailable || updatedRule.LastError != "mount missing" {
		t.Errorf("unexpected rule status/error: %+v", updatedRule)
	}
	if updatedRule.LastScanAt == nil || !updatedRule.LastScanAt.Equal(scanTime) {
		t.Errorf("LastScanAt = %v, want %v", updatedRule.LastScanAt, scanTime)
	}

	if err := st.DeleteBackupRule(ctx, "rule-1"); err != nil {
		t.Fatalf("DeleteBackupRule: %v", err)
	}
	remaining, err := st.ListTrackedFiles(ctx, "rule-1")
	if err != nil {
		t.Fatalf("ListTrackedFiles after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected tracked files cascade deleted, got %d", len(remaining))
	}
}

func TestStoreTransfersAndItemsLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "state.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()

	now := time.Now().UTC().Truncate(time.Second)
	tr := store.Transfer{
		ID:             "tr-1",
		Kind:           store.TransferKindUpload,
		Stage:          store.StageRunning,
		SourceSummary:  "2 files",
		DestNodeID:     "root-1",
		ConflictPolicy: aphotos.ConflictSkip,
		BytesTotal:     3000,
		ItemsTotal:     2,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	items := []store.TransferItem{
		{
			ID:         "item-1",
			TransferID: "tr-1",
			LocalPath:  "/tmp/a.jpg",
			Name:       "a.jpg",
			MediaType:  store.MediaTypePhoto,
			Size:       1000,
			MD5:        "11111111111111111111111111111111",
			Status:     store.ItemStatusRunning,
			UpdatedAt:  now,
		},
		{
			ID:         "item-2",
			TransferID: "tr-1",
			LocalPath:  "/tmp/b.mp4",
			Name:       "b.mp4",
			MediaType:  store.MediaTypeVideo,
			Size:       2000,
			MD5:        "22222222222222222222222222222222",
			Status:     store.ItemStatusQueued,
			UpdatedAt:  now,
		},
	}

	if _, err := st.CreateTransfer(ctx, tr, items); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	reconcileList, err := st.ListTransfersNeedingReconciliation(ctx)
	if err != nil {
		t.Fatalf("ListTransfersNeedingReconciliation: %v", err)
	}
	if len(reconcileList) != 1 || reconcileList[0].ID != "tr-1" {
		t.Fatalf("unexpected reconcileList: %+v", reconcileList)
	}

	// Update item-1 to confirmed and item-2 to unconfirmed
	items[0].Status = store.ItemStatusConfirmed
	items[0].RemoteNodeID = "remote-node-1"
	items[0].BytesDone = 1000
	if err := st.UpdateTransferItem(ctx, items[0]); err != nil {
		t.Fatalf("UpdateTransferItem 0: %v", err)
	}

	items[1].Status = store.ItemStatusUnconfirmed
	items[1].ErrorMessage = "connection reset after body write"
	if err := st.UpdateTransferItem(ctx, items[1]); err != nil {
		t.Fatalf("UpdateTransferItem 1: %v", err)
	}

	tr.Stage = store.StageUnconfirmed
	tr.BytesDone = 1000
	tr.ItemsDone = 1
	tr.ItemsUnconfirmed = 1
	if err := st.UpdateTransfer(ctx, tr); err != nil {
		t.Fatalf("UpdateTransfer: %v", err)
	}

	confirmed, err := st.ListConfirmedItems(ctx, 10)
	if err != nil {
		t.Fatalf("ListConfirmedItems: %v", err)
	}
	if len(confirmed) != 1 || confirmed[0].RemoteNodeID != "remote-node-1" {
		t.Fatalf("unexpected confirmed items: %+v", confirmed)
	}
}

func TestStoreKeepFolderStructureAndRemoteDirRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()

	rule, err := st.CreateBackupRule(ctx, store.BackupRule{ID: "r1", LocalPath: "/src", KeepFolderStructure: true})
	if err != nil || !rule.KeepFolderStructure {
		t.Fatalf("CreateBackupRule: %+v, %v", rule, err)
	}
	if err := st.UpsertTrackedFile(ctx, store.TrackedFile{BackupRuleID: "r1", RelPath: "a.jpg", AbsPath: "/src/a.jpg", Status: store.ItemStatusConfirmed, LastSyncedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("track: %v", err)
	}
	rule.KeepFolderStructure = false
	updated, err := st.UpdateBackupRule(ctx, rule)
	if err != nil || updated.KeepFolderStructure {
		t.Fatalf("UpdateBackupRule: %+v, %v", updated, err)
	}
	files, _ := st.ListTrackedFiles(ctx, "r1")
	if len(files) != 0 {
		t.Errorf("changing KeepFolderStructure must reset tracked files, got %d", len(files))
	}

	now := time.Now().UTC()
	if _, err := st.CreateTransfer(ctx, store.Transfer{ID: "t1", Kind: store.TransferKindUpload, Stage: store.StageQueued}, []store.TransferItem{
		{ID: "i1", Name: "a.jpg", RemoteDir: "Trips/sub", Status: store.ItemStatusQueued, UpdatedAt: now},
	}); err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	it, err := st.GetTransferItem(ctx, "i1")
	if err != nil || it.RemoteDir != "Trips/sub" {
		t.Fatalf("GetTransferItem: %+v, %v", it, err)
	}
	it.Status = store.ItemStatusConfirmed
	if err := st.UpdateTransferItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	if again, _ := st.GetTransferItem(ctx, "i1"); again.RemoteDir != "Trips/sub" {
		t.Errorf("RemoteDir lost on update: %+v", again)
	}
}

func TestStoreOpensDatabaseCreatedBeforeFolderColumns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "state.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE backup_rules (id TEXT PRIMARY KEY, local_path TEXT NOT NULL, include_videos INTEGER NOT NULL DEFAULT 1, schedule_mode TEXT NOT NULL DEFAULT 'immediate', interval_minutes INTEGER NOT NULL DEFAULT 15, conflict_policy TEXT NOT NULL DEFAULT 'skip', destination_node_id TEXT NOT NULL DEFAULT '', paused INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'active', last_scan_at TEXT NOT NULL DEFAULT '', last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`INSERT INTO backup_rules (id, local_path, created_at, updated_at) VALUES ('r', '/x', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		`CREATE TABLE transfers (id TEXT PRIMARY KEY, kind TEXT NOT NULL, stage TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE transfer_items (id TEXT PRIMARY KEY, transfer_id TEXT NOT NULL, name TEXT NOT NULL, status TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	_ = raw.Close()

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()
	r, err := st.GetBackupRule(ctx, "r")
	if err != nil || r.KeepFolderStructure {
		t.Errorf("GetBackupRule: %+v, %v", r, err)
	}
}
