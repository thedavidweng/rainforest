package store

import (
	"context"
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
