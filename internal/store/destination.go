package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) BindDestination(ctx context.Context, key, scope string) error {
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO destination_bindings (id, scope) VALUES (?, ?)`, key, scope); err != nil {
		return err
	}
	var existing string
	if err := s.db.QueryRowContext(ctx, `SELECT scope FROM destination_bindings WHERE id = ?`, key).Scan(&existing); err != nil {
		return err
	}
	if existing != scope {
		return errors.New("account, region or destination changed; restore the original account/target or create a new job/rule")
	}
	return nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Store) SaveAlbumCheckpoint(ctx context.Context, item TransferItem, tracked *TrackedFile) error {
	if tracked == nil {
		return s.UpdateTransferItem(ctx, item)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateTransferItem(ctx, tx, item); err != nil {
		return err
	}
	tracked.MD5 = item.MD5
	tracked.RemoteNodeID = item.RemoteNodeID
	tracked.Status = item.Status
	tracked.LastSyncedAt = item.UpdatedAt
	if err := upsertTrackedFile(ctx, tx, *tracked); err != nil {
		return err
	}
	return tx.Commit()
}
