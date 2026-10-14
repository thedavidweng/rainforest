package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("record not found")

type ScheduleMode string

const (
	ScheduleImmediate ScheduleMode = "immediate"
	ScheduleInterval  ScheduleMode = "interval"
)

type BackupStatus string

const (
	BackupStatusActive            BackupStatus = "active"
	BackupStatusPaused            BackupStatus = "paused"
	BackupStatusScanning          BackupStatus = "scanning"
	BackupStatusSourceUnavailable BackupStatus = "source_unavailable"
	BackupStatusError             BackupStatus = "error"
)

type TransferKind string

const (
	TransferKindUpload   TransferKind = "upload"
	TransferKindBackup   TransferKind = "backup"
	TransferKindDownload TransferKind = "download"
)

type TransferStage string

const (
	StageQueued      TransferStage = "queued"
	StageRunning     TransferStage = "running"
	StagePaused      TransferStage = "paused"
	StageCompleted   TransferStage = "completed"
	StagePartial     TransferStage = "partial"
	StageFailed      TransferStage = "failed"
	StageCancelled   TransferStage = "cancelled"
	StageUnconfirmed TransferStage = "unconfirmed"
)

func IsTerminalStage(stage TransferStage) bool {
	switch stage {
	case StageCompleted, StagePartial, StageFailed, StageCancelled, StageUnconfirmed:
		return true
	default:
		return false
	}
}

type ItemStatus string

const (
	ItemStatusQueued      ItemStatus = "queued"
	ItemStatusRunning     ItemStatus = "running"
	ItemStatusConfirmed   ItemStatus = "confirmed"
	ItemStatusUnconfirmed ItemStatus = "unconfirmed"
	ItemStatusSkipped     ItemStatus = "skipped"
	ItemStatusFailed      ItemStatus = "failed"
	ItemStatusCancelled   ItemStatus = "cancelled"
)

type MediaType string

const (
	MediaTypePhoto MediaType = "photo"
	MediaTypeVideo MediaType = "video"
)

type Settings struct {
	Region                 string                 `json:"region"`
	MaxMetadataConcurrency int                    `json:"maxMetadataConcurrency"`
	MaxUploadConcurrency   int                    `json:"maxUploadConcurrency"`
	DefaultConflictPolicy  aphotos.ConflictPolicy `json:"defaultConflictPolicy"`
	LaunchAtLogin          bool                   `json:"launchAtLogin"`
	CloseToTray            bool                   `json:"closeToTray"`
	NotificationsEnabled   bool                   `json:"notificationsEnabled"`
	BackupsStopped         bool                   `json:"backupsStopped"`
}

func DefaultSettings() Settings {
	return Settings{
		Region:                 "",
		MaxMetadataConcurrency: 4,
		MaxUploadConcurrency:   2,
		DefaultConflictPolicy:  aphotos.ConflictSkip,
		LaunchAtLogin:          false,
		CloseToTray:            true,
		NotificationsEnabled:   true,
		BackupsStopped:         false,
	}
}

type BackupRule struct {
	ID                string                 `json:"id"`
	LocalPath         string                 `json:"localPath"`
	IncludeVideos     bool                   `json:"includeVideos"`
	ScheduleMode      ScheduleMode           `json:"scheduleMode"`
	IntervalMinutes   int                    `json:"intervalMinutes"`
	ConflictPolicy    aphotos.ConflictPolicy `json:"conflictPolicy"`
	DestinationNodeID string                 `json:"destinationNodeId"`
	Paused            bool                   `json:"paused"`
	Status            BackupStatus           `json:"status"`
	LastScanAt        *time.Time             `json:"lastScanAt,omitempty"`
	LastError         string                 `json:"lastError,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
}

type TrackedFile struct {
	BackupRuleID string     `json:"backupRuleId"`
	RelPath      string     `json:"relPath"`
	AbsPath      string     `json:"absPath"`
	Size         int64      `json:"size"`
	MtimeNs      int64      `json:"mtimeNs"`
	MD5          string     `json:"md5"`
	RemoteNodeID string     `json:"remoteNodeId"`
	Status       ItemStatus `json:"status"`
	LastSyncedAt time.Time  `json:"lastSyncedAt"`
}

type Transfer struct {
	ID               string                 `json:"id"`
	Kind             TransferKind           `json:"kind"`
	Stage            TransferStage          `json:"stage"`
	SourceSummary    string                 `json:"sourceSummary"`
	DestNodeID       string                 `json:"destNodeId"`
	DestLocalDir     string                 `json:"destLocalDir"`
	ConflictPolicy   aphotos.ConflictPolicy `json:"conflictPolicy"`
	BytesDone        int64                  `json:"bytesDone"`
	BytesTotal       int64                  `json:"bytesTotal"`
	ItemsDone        int                    `json:"itemsDone"`
	ItemsSkipped     int                    `json:"itemsSkipped"`
	ItemsFailed      int                    `json:"itemsFailed"`
	ItemsUnconfirmed int                    `json:"itemsUnconfirmed"`
	ItemsTotal       int                    `json:"itemsTotal"`
	ErrorCode        string                 `json:"errorCode,omitempty"`
	ErrorMessage     string                 `json:"errorMessage,omitempty"`
	CreatedAt        time.Time              `json:"createdAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
	FinishedAt       *time.Time             `json:"finishedAt,omitempty"`
}

type TransferItem struct {
	ID           string     `json:"id"`
	TransferID   string     `json:"transferId"`
	LocalPath    string     `json:"localPath"`
	Name         string     `json:"name"`
	MediaType    MediaType  `json:"mediaType"`
	Size         int64      `json:"size"`
	MD5          string     `json:"md5"`
	RemoteNodeID string     `json:"remoteNodeId"`
	Status       ItemStatus `json:"status"`
	BytesDone    int64      `json:"bytesDone"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Store struct {
	db   *sql.DB
	path string
}

func DefaultDBPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		home, hErr := os.UserHomeDir()
		if hErr == nil && strings.TrimSpace(home) != "" {
			dir = filepath.Join(home, ".config")
		} else {
			dir = "."
		}
	}
	return filepath.Join(dir, "rainforest", "state.db")
}

func Open(ctx context.Context, dbPath string) (*Store, error) {
	cleaned := strings.TrimSpace(dbPath)
	if cleaned == "" {
		cleaned = DefaultDBPath()
	}
	if cleaned != ":memory:" && !strings.HasPrefix(cleaned, "file:") {
		cleaned = filepath.Clean(cleaned)
		if err := os.MkdirAll(filepath.Dir(cleaned), 0o700); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", cleaned)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	st := &Store{db: db, path: cleaned}
	if err := st.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA synchronous = NORMAL;`,
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA busy_timeout = 5000;`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			region TEXT NOT NULL DEFAULT '',
			max_metadata_concurrency INTEGER NOT NULL DEFAULT 4,
			max_upload_concurrency INTEGER NOT NULL DEFAULT 2,
			default_conflict_policy TEXT NOT NULL DEFAULT 'skip',
			launch_at_login INTEGER NOT NULL DEFAULT 0,
			close_to_tray INTEGER NOT NULL DEFAULT 1,
			notifications_enabled INTEGER NOT NULL DEFAULT 1,
			backups_stopped INTEGER NOT NULL DEFAULT 0
		);`,
		`INSERT OR IGNORE INTO settings (
			id, region, max_metadata_concurrency, max_upload_concurrency,
			default_conflict_policy, launch_at_login, close_to_tray,
			notifications_enabled, backups_stopped
		) VALUES (1, '', 4, 2, 'skip', 0, 1, 1, 0);`,
		`CREATE TABLE IF NOT EXISTS backup_rules (
			id TEXT PRIMARY KEY,
			local_path TEXT NOT NULL,
			include_videos INTEGER NOT NULL DEFAULT 1,
			schedule_mode TEXT NOT NULL DEFAULT 'immediate',
			interval_minutes INTEGER NOT NULL DEFAULT 15,
			conflict_policy TEXT NOT NULL DEFAULT 'skip',
			destination_node_id TEXT NOT NULL DEFAULT '',
			paused INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			last_scan_at TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS tracked_files (
			backup_rule_id TEXT NOT NULL,
			rel_path TEXT NOT NULL,
			abs_path TEXT NOT NULL,
			size INTEGER NOT NULL,
			mtime_ns INTEGER NOT NULL,
			md5 TEXT NOT NULL DEFAULT '',
			remote_node_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			last_synced_at TEXT NOT NULL,
			PRIMARY KEY (backup_rule_id, rel_path),
			FOREIGN KEY (backup_rule_id) REFERENCES backup_rules(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS transfers (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			stage TEXT NOT NULL,
			source_summary TEXT NOT NULL DEFAULT '',
			dest_node_id TEXT NOT NULL DEFAULT '',
			dest_local_dir TEXT NOT NULL DEFAULT '',
			conflict_policy TEXT NOT NULL DEFAULT 'skip',
			bytes_done INTEGER NOT NULL DEFAULT 0,
			bytes_total INTEGER NOT NULL DEFAULT 0,
			items_done INTEGER NOT NULL DEFAULT 0,
			items_skipped INTEGER NOT NULL DEFAULT 0,
			items_failed INTEGER NOT NULL DEFAULT 0,
			items_unconfirmed INTEGER NOT NULL DEFAULT 0,
			items_total INTEGER NOT NULL DEFAULT 0,
			error_code TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS transfer_items (
			id TEXT PRIMARY KEY,
			transfer_id TEXT NOT NULL,
			local_path TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			media_type TEXT NOT NULL DEFAULT 'photo',
			size INTEGER NOT NULL DEFAULT 0,
			md5 TEXT NOT NULL DEFAULT '',
			remote_node_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			bytes_done INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL,
			FOREIGN KEY (transfer_id) REFERENCES transfers(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_transfer_items_transfer_id ON transfer_items(transfer_id);`,
		`CREATE INDEX IF NOT EXISTS idx_transfer_items_status ON transfer_items(status);`,
		`CREATE INDEX IF NOT EXISTS idx_tracked_files_rule ON tracked_files(backup_rule_id);`,
	}

	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrate sqlite schema: %w", err)
		}
	}
	return nil
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var out Settings
	var conflict string
	var launch, tray, notify, stopped int
	err := s.db.QueryRowContext(ctx, `
		SELECT region, max_metadata_concurrency, max_upload_concurrency,
		       default_conflict_policy, launch_at_login, close_to_tray,
		       notifications_enabled, backups_stopped
		FROM settings WHERE id = 1
	`).Scan(
		&out.Region,
		&out.MaxMetadataConcurrency,
		&out.MaxUploadConcurrency,
		&conflict,
		&launch,
		&tray,
		&notify,
		&stopped,
	)
	if err != nil {
		return Settings{}, fmt.Errorf("query settings: %w", err)
	}
	out.DefaultConflictPolicy = normalizeConflict(aphotos.ConflictPolicy(conflict))
	out.LaunchAtLogin = launch != 0
	out.CloseToTray = tray != 0
	out.NotificationsEnabled = notify != 0
	out.BackupsStopped = stopped != 0
	if out.MaxMetadataConcurrency <= 0 {
		out.MaxMetadataConcurrency = 4
	}
	if out.MaxUploadConcurrency <= 0 {
		out.MaxUploadConcurrency = 2
	}
	return out, nil
}

func (s *Store) UpdateSettings(ctx context.Context, cfg Settings) error {
	metaConc := cfg.MaxMetadataConcurrency
	if metaConc <= 0 {
		metaConc = 4
	}
	upConc := cfg.MaxUploadConcurrency
	if upConc <= 0 {
		upConc = 2
	}
	conflict := normalizeConflict(cfg.DefaultConflictPolicy)
	_, err := s.db.ExecContext(ctx, `
		UPDATE settings SET
			region = ?,
			max_metadata_concurrency = ?,
			max_upload_concurrency = ?,
			default_conflict_policy = ?,
			launch_at_login = ?,
			close_to_tray = ?,
			notifications_enabled = ?,
			backups_stopped = ?
		WHERE id = 1
	`,
		strings.ToLower(strings.TrimSpace(cfg.Region)),
		metaConc,
		upConc,
		string(conflict),
		boolToInt(cfg.LaunchAtLogin),
		boolToInt(cfg.CloseToTray),
		boolToInt(cfg.NotificationsEnabled),
		boolToInt(cfg.BackupsStopped),
	)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	return nil
}

func (s *Store) CreateBackupRule(ctx context.Context, rule BackupRule) (BackupRule, error) {
	now := time.Now().UTC()
	if strings.TrimSpace(rule.ID) == "" {
		rule.ID = uuid.NewString()
	}
	rule.LocalPath = filepath.Clean(strings.TrimSpace(rule.LocalPath))
	if rule.ScheduleMode != ScheduleInterval {
		rule.ScheduleMode = ScheduleImmediate
	}
	if rule.IntervalMinutes <= 0 {
		rule.IntervalMinutes = 15
	}
	rule.ConflictPolicy = normalizeConflict(rule.ConflictPolicy)
	if rule.Status == "" {
		if rule.Paused {
			rule.Status = BackupStatusPaused
		} else {
			rule.Status = BackupStatusActive
		}
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = rule.CreatedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO backup_rules (
			id, local_path, include_videos, schedule_mode, interval_minutes,
			conflict_policy, destination_node_id, paused, status,
			last_scan_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		rule.ID,
		rule.LocalPath,
		boolToInt(rule.IncludeVideos),
		string(rule.ScheduleMode),
		rule.IntervalMinutes,
		string(rule.ConflictPolicy),
		strings.TrimSpace(rule.DestinationNodeID),
		boolToInt(rule.Paused),
		string(rule.Status),
		formatTimePtr(rule.LastScanAt),
		aphotos.Redact(rule.LastError),
		formatTime(rule.CreatedAt),
		formatTime(rule.UpdatedAt),
	)
	if err != nil {
		return BackupRule{}, fmt.Errorf("insert backup rule: %w", err)
	}
	return rule, nil
}

func (s *Store) UpdateBackupRule(ctx context.Context, rule BackupRule) (BackupRule, error) {
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = time.Now().UTC()
	}
	rule.LocalPath = filepath.Clean(strings.TrimSpace(rule.LocalPath))
	if rule.ScheduleMode != ScheduleInterval {
		rule.ScheduleMode = ScheduleImmediate
	}
	if rule.IntervalMinutes <= 0 {
		rule.IntervalMinutes = 15
	}
	rule.ConflictPolicy = normalizeConflict(rule.ConflictPolicy)
	if rule.Status == "" {
		if rule.Paused {
			rule.Status = BackupStatusPaused
		} else {
			rule.Status = BackupStatusActive
		}
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE backup_rules SET
			local_path = ?,
			include_videos = ?,
			schedule_mode = ?,
			interval_minutes = ?,
			conflict_policy = ?,
			destination_node_id = ?,
			paused = ?,
			status = ?,
			last_scan_at = ?,
			last_error = ?,
			updated_at = ?
		WHERE id = ?
	`,
		rule.LocalPath,
		boolToInt(rule.IncludeVideos),
		string(rule.ScheduleMode),
		rule.IntervalMinutes,
		string(rule.ConflictPolicy),
		strings.TrimSpace(rule.DestinationNodeID),
		boolToInt(rule.Paused),
		string(rule.Status),
		formatTimePtr(rule.LastScanAt),
		aphotos.Redact(rule.LastError),
		formatTime(rule.UpdatedAt),
		rule.ID,
	)
	if err != nil {
		return BackupRule{}, fmt.Errorf("update backup rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return BackupRule{}, ErrNotFound
	}
	return s.GetBackupRule(ctx, rule.ID)
}

func (s *Store) SetBackupRuleStatus(ctx context.Context, id string, status BackupStatus, lastErr string, lastScanAt *time.Time) error {
	now := formatTime(time.Now().UTC())
	if lastScanAt != nil {
		_, err := s.db.ExecContext(ctx, `
			UPDATE backup_rules SET status = ?, last_error = ?, last_scan_at = ?, updated_at = ? WHERE id = ?
		`, string(status), aphotos.Redact(lastErr), formatTimePtr(lastScanAt), now, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE backup_rules SET status = ?, last_error = ?, updated_at = ? WHERE id = ?
	`, string(status), aphotos.Redact(lastErr), now, id)
	return err
}

func (s *Store) GetBackupRule(ctx context.Context, id string) (BackupRule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, local_path, include_videos, schedule_mode, interval_minutes,
		       conflict_policy, destination_node_id, paused, status,
		       last_scan_at, last_error, created_at, updated_at
		FROM backup_rules WHERE id = ?
	`, id)
	rule, err := scanBackupRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return BackupRule{}, ErrNotFound
	}
	return rule, err
}

func (s *Store) ListBackupRules(ctx context.Context) ([]BackupRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, local_path, include_videos, schedule_mode, interval_minutes,
		       conflict_policy, destination_node_id, paused, status,
		       last_scan_at, last_error, created_at, updated_at
		FROM backup_rules ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list backup rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []BackupRule
	for rows.Next() {
		r, sErr := scanBackupRule(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBackupRule(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM backup_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete backup rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpsertTrackedFile(ctx context.Context, tf TrackedFile) error {
	if tf.LastSyncedAt.IsZero() {
		tf.LastSyncedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tracked_files (
			backup_rule_id, rel_path, abs_path, size, mtime_ns,
			md5, remote_node_id, status, last_synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(backup_rule_id, rel_path) DO UPDATE SET
			abs_path = excluded.abs_path,
			size = excluded.size,
			mtime_ns = excluded.mtime_ns,
			md5 = excluded.md5,
			remote_node_id = excluded.remote_node_id,
			status = excluded.status,
			last_synced_at = excluded.last_synced_at
	`,
		tf.BackupRuleID,
		tf.RelPath,
		tf.AbsPath,
		tf.Size,
		tf.MtimeNs,
		strings.ToLower(strings.TrimSpace(tf.MD5)),
		strings.TrimSpace(tf.RemoteNodeID),
		string(tf.Status),
		formatTime(tf.LastSyncedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert tracked file: %w", err)
	}
	return nil
}

func (s *Store) GetTrackedFile(ctx context.Context, ruleID, relPath string) (TrackedFile, error) {
	var tf TrackedFile
	var statusStr, syncedStr string
	err := s.db.QueryRowContext(ctx, `
		SELECT backup_rule_id, rel_path, abs_path, size, mtime_ns,
		       md5, remote_node_id, status, last_synced_at
		FROM tracked_files WHERE backup_rule_id = ? AND rel_path = ?
	`, ruleID, relPath).Scan(
		&tf.BackupRuleID,
		&tf.RelPath,
		&tf.AbsPath,
		&tf.Size,
		&tf.MtimeNs,
		&tf.MD5,
		&tf.RemoteNodeID,
		&statusStr,
		&syncedStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TrackedFile{}, ErrNotFound
	}
	if err != nil {
		return TrackedFile{}, fmt.Errorf("get tracked file: %w", err)
	}
	tf.Status = ItemStatus(statusStr)
	tf.LastSyncedAt = parseTime(syncedStr)
	return tf, nil
}

func (s *Store) ListTrackedFiles(ctx context.Context, ruleID string) ([]TrackedFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT backup_rule_id, rel_path, abs_path, size, mtime_ns,
		       md5, remote_node_id, status, last_synced_at
		FROM tracked_files WHERE backup_rule_id = ? ORDER BY rel_path ASC
	`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TrackedFile
	for rows.Next() {
		var tf TrackedFile
		var statusStr, syncedStr string
		if sErr := rows.Scan(
			&tf.BackupRuleID,
			&tf.RelPath,
			&tf.AbsPath,
			&tf.Size,
			&tf.MtimeNs,
			&tf.MD5,
			&tf.RemoteNodeID,
			&statusStr,
			&syncedStr,
		); sErr != nil {
			return nil, sErr
		}
		tf.Status = ItemStatus(statusStr)
		tf.LastSyncedAt = parseTime(syncedStr)
		out = append(out, tf)
	}
	return out, rows.Err()
}

func (s *Store) CreateTransfer(ctx context.Context, tr Transfer, items []TransferItem) (Transfer, error) {
	now := time.Now().UTC()
	if strings.TrimSpace(tr.ID) == "" {
		tr.ID = uuid.NewString()
	}
	if tr.CreatedAt.IsZero() {
		tr.CreatedAt = now
	}
	if tr.UpdatedAt.IsZero() {
		tr.UpdatedAt = tr.CreatedAt
	}
	tr.ConflictPolicy = normalizeConflict(tr.ConflictPolicy)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Transfer{}, fmt.Errorf("begin transfer tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO transfers (
			id, kind, stage, source_summary, dest_node_id, dest_local_dir,
			conflict_policy, bytes_done, bytes_total, items_done, items_skipped,
			items_failed, items_unconfirmed, items_total, error_code,
			error_message, created_at, updated_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		tr.ID,
		string(tr.Kind),
		string(tr.Stage),
		tr.SourceSummary,
		tr.DestNodeID,
		tr.DestLocalDir,
		string(tr.ConflictPolicy),
		tr.BytesDone,
		tr.BytesTotal,
		tr.ItemsDone,
		tr.ItemsSkipped,
		tr.ItemsFailed,
		tr.ItemsUnconfirmed,
		tr.ItemsTotal,
		tr.ErrorCode,
		aphotos.Redact(tr.ErrorMessage),
		formatTime(tr.CreatedAt),
		formatTime(tr.UpdatedAt),
		formatTimePtr(tr.FinishedAt),
	)
	if err != nil {
		return Transfer{}, fmt.Errorf("insert transfer: %w", err)
	}

	for i := range items {
		it := items[i]
		if strings.TrimSpace(it.ID) == "" {
			it.ID = uuid.NewString()
		}
		it.TransferID = tr.ID
		if it.UpdatedAt.IsZero() {
			it.UpdatedAt = now
		}
		if it.MediaType == "" {
			it.MediaType = MediaTypePhoto
		}
		if it.Status == "" {
			it.Status = ItemStatusQueued
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO transfer_items (
				id, transfer_id, local_path, name, media_type, size,
				md5, remote_node_id, status, bytes_done, error_message, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			it.ID,
			it.TransferID,
			it.LocalPath,
			it.Name,
			string(it.MediaType),
			it.Size,
			strings.ToLower(strings.TrimSpace(it.MD5)),
			strings.TrimSpace(it.RemoteNodeID),
			string(it.Status),
			it.BytesDone,
			aphotos.Redact(it.ErrorMessage),
			formatTime(it.UpdatedAt),
		)
		if err != nil {
			return Transfer{}, fmt.Errorf("insert transfer item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Transfer{}, fmt.Errorf("commit transfer tx: %w", err)
	}
	return tr, nil
}

func (s *Store) UpdateTransfer(ctx context.Context, tr Transfer) error {
	if tr.UpdatedAt.IsZero() {
		tr.UpdatedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE transfers SET
			stage = ?,
			source_summary = ?,
			dest_node_id = ?,
			dest_local_dir = ?,
			conflict_policy = ?,
			bytes_done = ?,
			bytes_total = ?,
			items_done = ?,
			items_skipped = ?,
			items_failed = ?,
			items_unconfirmed = ?,
			items_total = ?,
			error_code = ?,
			error_message = ?,
			updated_at = ?,
			finished_at = ?
		WHERE id = ?
	`,
		string(tr.Stage),
		tr.SourceSummary,
		tr.DestNodeID,
		tr.DestLocalDir,
		string(normalizeConflict(tr.ConflictPolicy)),
		tr.BytesDone,
		tr.BytesTotal,
		tr.ItemsDone,
		tr.ItemsSkipped,
		tr.ItemsFailed,
		tr.ItemsUnconfirmed,
		tr.ItemsTotal,
		tr.ErrorCode,
		aphotos.Redact(tr.ErrorMessage),
		formatTime(tr.UpdatedAt),
		formatTimePtr(tr.FinishedAt),
		tr.ID,
	)
	if err != nil {
		return fmt.Errorf("update transfer: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetTransfer(ctx context.Context, id string) (Transfer, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, kind, stage, source_summary, dest_node_id, dest_local_dir,
		       conflict_policy, bytes_done, bytes_total, items_done, items_skipped,
		       items_failed, items_unconfirmed, items_total, error_code,
		       error_message, created_at, updated_at, finished_at
		FROM transfers WHERE id = ?
	`, id)
	tr, err := scanTransfer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Transfer{}, ErrNotFound
	}
	return tr, err
}

func (s *Store) ListTransfers(ctx context.Context, limit int) ([]Transfer, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, stage, source_summary, dest_node_id, dest_local_dir,
		       conflict_policy, bytes_done, bytes_total, items_done, items_skipped,
		       items_failed, items_unconfirmed, items_total, error_code,
		       error_message, created_at, updated_at, finished_at
		FROM transfers ORDER BY created_at DESC, id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list transfers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Transfer
	for rows.Next() {
		tr, sErr := scanTransfer(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

func (s *Store) ListTransfersNeedingReconciliation(ctx context.Context) ([]Transfer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT t.id, t.kind, t.stage, t.source_summary, t.dest_node_id, t.dest_local_dir,
		       t.conflict_policy, t.bytes_done, t.bytes_total, t.items_done, t.items_skipped,
		       t.items_failed, t.items_unconfirmed, t.items_total, t.error_code,
		       t.error_message, t.created_at, t.updated_at, t.finished_at
		FROM transfers t
		LEFT JOIN transfer_items i ON i.transfer_id = t.id
		WHERE t.stage IN ('running', 'queued', 'unconfirmed')
		   OR i.status IN ('running', 'unconfirmed')
		ORDER BY t.created_at ASC, t.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list transfers needing reconciliation: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Transfer
	for rows.Next() {
		tr, sErr := scanTransfer(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

func (s *Store) GetTransferItem(ctx context.Context, itemID string) (TransferItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, transfer_id, local_path, name, media_type, size,
		       md5, remote_node_id, status, bytes_done, error_message, updated_at
		FROM transfer_items WHERE id = ?
	`, itemID)
	it, err := scanTransferItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TransferItem{}, ErrNotFound
	}
	return it, err
}

func (s *Store) ListTransferItems(ctx context.Context, transferID string) ([]TransferItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, transfer_id, local_path, name, media_type, size,
		       md5, remote_node_id, status, bytes_done, error_message, updated_at
		FROM transfer_items WHERE transfer_id = ? ORDER BY id ASC
	`, transferID)
	if err != nil {
		return nil, fmt.Errorf("list transfer items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TransferItem
	for rows.Next() {
		it, sErr := scanTransferItem(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTransferItem(ctx context.Context, item TransferItem) error {
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE transfer_items SET
			local_path = ?,
			name = ?,
			media_type = ?,
			size = ?,
			md5 = ?,
			remote_node_id = ?,
			status = ?,
			bytes_done = ?,
			error_message = ?,
			updated_at = ?
		WHERE id = ?
	`,
		item.LocalPath,
		item.Name,
		string(item.MediaType),
		item.Size,
		strings.ToLower(strings.TrimSpace(item.MD5)),
		strings.TrimSpace(item.RemoteNodeID),
		string(item.Status),
		item.BytesDone,
		aphotos.Redact(item.ErrorMessage),
		formatTime(item.UpdatedAt),
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("update transfer item: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListConfirmedItems(ctx context.Context, limit int) ([]TransferItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.transfer_id, i.local_path, i.name, i.media_type, i.size,
		       i.md5, i.remote_node_id, i.status, i.bytes_done, i.error_message, i.updated_at
		FROM transfer_items i
		JOIN transfers t ON t.id = i.transfer_id
		WHERE i.status = 'confirmed' AND i.remote_node_id != '' AND t.kind IN ('upload', 'backup')
		ORDER BY i.updated_at DESC, i.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list confirmed items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TransferItem
	seen := make(map[string]struct{})
	for rows.Next() {
		it, sErr := scanTransferItem(rows)
		if sErr != nil {
			return nil, sErr
		}
		if _, dup := seen[it.RemoteNodeID]; dup {
			continue
		}
		seen[it.RemoteNodeID] = struct{}{}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) ClearFinishedTransfers(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM transfers WHERE stage IN ('completed', 'partial', 'failed', 'cancelled')
	`)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBackupRule(r rowScanner) (BackupRule, error) {
	var rule BackupRule
	var incVideos, paused int
	var sched, conflict, statusStr, lastScanStr, createdStr, updatedStr string
	err := r.Scan(
		&rule.ID,
		&rule.LocalPath,
		&incVideos,
		&sched,
		&rule.IntervalMinutes,
		&conflict,
		&rule.DestinationNodeID,
		&paused,
		&statusStr,
		&lastScanStr,
		&rule.LastError,
		&createdStr,
		&updatedStr,
	)
	if err != nil {
		return BackupRule{}, err
	}
	rule.IncludeVideos = incVideos != 0
	rule.ScheduleMode = ScheduleMode(sched)
	rule.ConflictPolicy = normalizeConflict(aphotos.ConflictPolicy(conflict))
	rule.Paused = paused != 0
	rule.Status = BackupStatus(statusStr)
	rule.LastScanAt = parseTimePtr(lastScanStr)
	rule.CreatedAt = parseTime(createdStr)
	rule.UpdatedAt = parseTime(updatedStr)
	return rule, nil
}

func scanTransfer(r rowScanner) (Transfer, error) {
	var tr Transfer
	var kindStr, stageStr, conflictStr, createdStr, updatedStr, finishedStr string
	err := r.Scan(
		&tr.ID,
		&kindStr,
		&stageStr,
		&tr.SourceSummary,
		&tr.DestNodeID,
		&tr.DestLocalDir,
		&conflictStr,
		&tr.BytesDone,
		&tr.BytesTotal,
		&tr.ItemsDone,
		&tr.ItemsSkipped,
		&tr.ItemsFailed,
		&tr.ItemsUnconfirmed,
		&tr.ItemsTotal,
		&tr.ErrorCode,
		&tr.ErrorMessage,
		&createdStr,
		&updatedStr,
		&finishedStr,
	)
	if err != nil {
		return Transfer{}, err
	}
	tr.Kind = TransferKind(kindStr)
	tr.Stage = TransferStage(stageStr)
	tr.ConflictPolicy = normalizeConflict(aphotos.ConflictPolicy(conflictStr))
	tr.CreatedAt = parseTime(createdStr)
	tr.UpdatedAt = parseTime(updatedStr)
	tr.FinishedAt = parseTimePtr(finishedStr)
	return tr, nil
}

func scanTransferItem(r rowScanner) (TransferItem, error) {
	var it TransferItem
	var mediaStr, statusStr, updatedStr string
	err := r.Scan(
		&it.ID,
		&it.TransferID,
		&it.LocalPath,
		&it.Name,
		&mediaStr,
		&it.Size,
		&it.MD5,
		&it.RemoteNodeID,
		&statusStr,
		&it.BytesDone,
		&it.ErrorMessage,
		&updatedStr,
	)
	if err != nil {
		return TransferItem{}, err
	}
	it.MediaType = MediaType(mediaStr)
	it.Status = ItemStatus(statusStr)
	it.UpdatedAt = parseTime(updatedStr)
	return it, nil
}

func normalizeConflict(p aphotos.ConflictPolicy) aphotos.ConflictPolicy {
	switch p {
	case aphotos.ConflictFail, aphotos.ConflictSkip, aphotos.ConflictRename:
		return p
	default:
		return aphotos.ConflictSkip
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) time.Time {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func parseTimePtr(raw string) *time.Time {
	t := parseTime(raw)
	if t.IsZero() {
		return nil
	}
	return &t
}
