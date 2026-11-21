package store

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"slices"
)

type GuidedProgress struct {
	Placed, Total, Revision      int
	Candidate, Opponent          string
	CanUndo, Complete, Restarted bool
	NewFavorite                  string
}

type guidedStep struct {
	Order             []string
	Placed, Low, High int
}

type guidedSession struct {
	Step     guidedStep
	History  []guidedStep
	Revision int
	First    string
}

func (session guidedSession) progress() GuidedProgress {
	step := session.Step
	progress := GuidedProgress{Placed: step.Placed, Total: len(step.Order), Revision: session.Revision, CanUndo: len(session.History) > 0, Complete: step.Placed == len(step.Order)}
	if !progress.Complete {
		progress.Candidate = step.Order[step.Placed]
		progress.Opponent = step.Order[(step.Low+step.High)/2]
	} else if len(step.Order) > 0 && step.Order[0] != session.First {
		progress.NewFavorite = step.Order[0]
	}
	return progress
}

func (s *Store) guidedOrder(ctx context.Context, tx *sql.Tx, listID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, s.bind("SELECT id FROM wishes WHERE wishlist_id = ? ORDER BY position"), listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) loadGuided(ctx context.Context, tx *sql.Tx, listID string, order []string) (guidedSession, bool, error) {
	var encoded string
	err := tx.QueryRowContext(ctx, s.bind("SELECT state FROM guided_rankings WHERE wishlist_id = ?"), listID).Scan(&encoded)
	var session guidedSession
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return session, false, err
	}
	if exists {
		if err := json.Unmarshal([]byte(encoded), &session); err != nil {
			return session, false, err
		}
	}
	if !exists || !slices.Equal(session.Step.Order, order) {
		revision := session.Revision + 1
		session = guidedSession{Step: guidedStep{Order: slices.Clone(order), Placed: min(1, len(order)), High: min(1, len(order))}, Revision: revision}
		if len(order) > 0 {
			session.First = order[0]
		}
		return session, exists, nil
	}
	return session, false, nil
}

func (s *Store) saveGuided(ctx context.Context, tx *sql.Tx, listID string, session guidedSession) error {
	encoded, err := json.Marshal(&session)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.bind("INSERT INTO guided_rankings (wishlist_id,state) VALUES (?,?) ON CONFLICT (wishlist_id) DO UPDATE SET state = excluded.state"), listID, string(encoded))
	return err
}

func (s *Store) GuidedRanking(ctx context.Context, listID string) (GuidedProgress, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GuidedProgress{}, err
	}
	defer tx.Rollback()
	if err := s.lockWishlist(ctx, tx, listID); err != nil {
		return GuidedProgress{}, err
	}
	order, err := s.guidedOrder(ctx, tx, listID)
	if err != nil {
		return GuidedProgress{}, err
	}
	session, restarted, err := s.loadGuided(ctx, tx, listID, order)
	if err != nil {
		return GuidedProgress{}, err
	}
	if err := s.saveGuided(ctx, tx, listID, session); err != nil {
		return GuidedProgress{}, err
	}
	if err := tx.Commit(); err != nil {
		return GuidedProgress{}, err
	}
	progress := session.progress()
	progress.Restarted = restarted
	return progress, nil
}

func (s *Store) PickGuided(ctx context.Context, listID string, revision int, winner, loser string) (GuidedProgress, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GuidedProgress{}, err
	}
	defer tx.Rollback()
	if err := s.lockWishlist(ctx, tx, listID); err != nil {
		return GuidedProgress{}, err
	}
	order, err := s.guidedOrder(ctx, tx, listID)
	if err != nil {
		return GuidedProgress{}, err
	}
	session, restarted, err := s.loadGuided(ctx, tx, listID, order)
	if err != nil {
		return GuidedProgress{}, err
	}
	progress := session.progress()
	if restarted || progress.Revision != revision || progress.Complete || !((winner == progress.Candidate && loser == progress.Opponent) || (loser == progress.Candidate && winner == progress.Opponent)) {
		return GuidedProgress{}, ErrInvalid
	}
	session.History = append(session.History, session.Step)
	step := &session.Step
	middle := (step.Low + step.High) / 2
	if winner == progress.Candidate {
		step.High = middle
	} else {
		step.Low = middle + 1
	}
	if step.Low == step.High {
		next := slices.Clone(step.Order)
		candidate := next[step.Placed]
		copy(next[step.Low+1:step.Placed+1], next[step.Low:step.Placed])
		next[step.Low] = candidate
		step.Order = next
		step.Placed++
		step.Low, step.High = 0, step.Placed
		if err := s.writeGuidedOrder(ctx, tx, listID, next); err != nil {
			return GuidedProgress{}, err
		}
	}
	session.Revision++
	if err := s.saveGuided(ctx, tx, listID, session); err != nil {
		return GuidedProgress{}, err
	}
	if err := tx.Commit(); err != nil {
		return GuidedProgress{}, err
	}
	return session.progress(), nil
}

func (s *Store) writeGuidedOrder(ctx context.Context, tx *sql.Tx, listID string, order []string) error {
	for position, id := range order {
		if _, err := tx.ExecContext(ctx, s.bind("UPDATE wishes SET position = ? WHERE wishlist_id = ? AND id = ?"), position, listID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UndoGuided(ctx context.Context, listID string, revision int) (GuidedProgress, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GuidedProgress{}, err
	}
	defer tx.Rollback()
	if err := s.lockWishlist(ctx, tx, listID); err != nil {
		return GuidedProgress{}, err
	}
	order, err := s.guidedOrder(ctx, tx, listID)
	if err != nil {
		return GuidedProgress{}, err
	}
	session, restarted, err := s.loadGuided(ctx, tx, listID, order)
	if err != nil {
		return GuidedProgress{}, err
	}
	if restarted || session.Revision != revision || len(session.History) == 0 {
		return GuidedProgress{}, ErrInvalid
	}
	session.Step = session.History[len(session.History)-1]
	session.History = session.History[:len(session.History)-1]
	session.Revision++
	if err := s.writeGuidedOrder(ctx, tx, listID, session.Step.Order); err != nil {
		return GuidedProgress{}, err
	}
	if err := s.saveGuided(ctx, tx, listID, session); err != nil {
		return GuidedProgress{}, err
	}
	if err := tx.Commit(); err != nil {
		return GuidedProgress{}, err
	}
	return session.progress(), nil
}

func (s *Store) RestartGuided(ctx context.Context, listID string, revision int) (GuidedProgress, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GuidedProgress{}, err
	}
	defer tx.Rollback()
	if err := s.lockWishlist(ctx, tx, listID); err != nil {
		return GuidedProgress{}, err
	}
	order, err := s.guidedOrder(ctx, tx, listID)
	if err != nil {
		return GuidedProgress{}, err
	}
	session, restarted, err := s.loadGuided(ctx, tx, listID, order)
	if err != nil {
		return GuidedProgress{}, err
	}
	if restarted || session.Revision != revision {
		return GuidedProgress{}, ErrInvalid
	}
	session.Step = guidedStep{Order: slices.Clone(order), Placed: min(1, len(order)), High: min(1, len(order))}
	session.History = nil
	session.First = ""
	if len(order) > 0 {
		session.First = order[0]
	}
	session.Revision++
	if err := s.saveGuided(ctx, tx, listID, session); err != nil {
		return GuidedProgress{}, err
	}
	if err := tx.Commit(); err != nil {
		return GuidedProgress{}, err
	}
	return session.progress(), nil
}
