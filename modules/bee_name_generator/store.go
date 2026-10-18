package beenamegenerator

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrBeeNameNotFound = errors.New("no bee names found")
var ErrBeeNameExists = errors.New("bee name already exists")
var ErrBeeNameSuggestionExists = errors.New("bee name suggestion already exists")

// BNGStore - Bee Name Generator Store
type BNGStore interface {
	GetBeeName(ctx context.Context) (string, error)
	UploadBeeName(ctx context.Context, beeName string) (string, error)
	DeleteBeeName(ctx context.Context, beeName string) (string, error)
	SubmitBeeName(ctx context.Context, beeName string) (string, error)
	GetBeeNameSuggestions(ctx context.Context, amount int64) ([]string, error)
	AcceptBeeNameSuggestion(ctx context.Context, beeName string) (string, error)
	RejectBeeNameSuggestion(ctx context.Context, beeName string) (string, error)
}

// store - Bee Name Generator Store PG implementation
type store struct {
	db *pgxpool.Pool
}

// NewStore - Create a new Bee Name Generator store
func NewStore(db *pgxpool.Pool) BNGStore {
	return &store{db: db}
}

// GetBeeName returns a random bee name from the database
func (s *store) GetBeeName(ctx context.Context) (string, error) {
	var beeName string
	err := s.db.QueryRow(ctx, "SELECT name FROM bee_name ORDER BY random() LIMIT 1").Scan(&beeName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrBeeNameNotFound
		}
		return "", err
	}
	return beeName, nil
}

// UploadBeeName uploads a bee name to the database
func (s *store) UploadBeeName(ctx context.Context, beeName string) (string, error) {
	_, err := s.db.Exec(ctx, "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrBeeNameExists
		}
		return "", err
	}
	return beeName, nil
}

// DeleteBeeName deletes a bee name from the database
func (s *store) DeleteBeeName(ctx context.Context, beeName string) (string, error) {
	_, err := s.db.Exec(ctx, "DELETE FROM bee_name WHERE name = $1", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}

// SubmitBeeName submits a bee name to the suggestion database
func (s *store) SubmitBeeName(ctx context.Context, beeName string) (string, error) {
	_, err := s.db.Exec(ctx, "INSERT INTO bee_name_suggestion (name) VALUES ($1)", beeName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrBeeNameSuggestionExists
		}
		return "", err
	}
	return beeName, nil
}

// GetBeeNameSuggestions returns a list of bee name suggestions
func (s *store) GetBeeNameSuggestions(ctx context.Context, amount int64) ([]string, error) {
	var beeNames []string
	rows, err := s.db.Query(ctx, "SELECT name FROM bee_name_suggestion ORDER BY random() LIMIT $1", amount)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var beeName string
		err := rows.Scan(&beeName)
		if err != nil {
			return []string{}, err
		}
		beeNames = append(beeNames, beeName)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(beeNames) == 0 {
		return []string{}, nil
	}
	return beeNames, nil
}

// AcceptBeeNameSuggestion accepts a bee name suggestion
func (s *store) AcceptBeeNameSuggestion(ctx context.Context, beeName string) (string, error) {
	_, err := s.db.Exec(ctx, "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrBeeNameExists
		}
		return "", err
	}

	_, err = s.db.Exec(ctx, "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}

// RejectBeeNameSuggestion rejects a bee name suggestion
func (s *store) RejectBeeNameSuggestion(ctx context.Context, beeName string) (string, error) {
	_, err := s.db.Exec(ctx, "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}
