package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) SaveDeviceSession(ctx context.Context, tokenHash []byte, expires time.Time) error {
	_, err := s.execContext(ctx, "INSERT INTO device_sessions (token_hash, expires_at) VALUES (?, ?)", tokenHash, expires.Unix())
	return err
}

func (s *Store) ValidDeviceSession(ctx context.Context, tokenHash []byte, now time.Time) (bool, error) {
	var expires int64
	err := s.queryRowContext(ctx, "SELECT expires_at FROM device_sessions WHERE token_hash = ?", tokenHash).Scan(&expires)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return expires > now.Unix(), err
}
