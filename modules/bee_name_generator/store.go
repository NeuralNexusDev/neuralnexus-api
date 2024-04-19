package beenamegenerator

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BNGStore - Bee Name Generator Store
type BNGStore interface {
	GetBeeName() (string, error)
	UploadBeeName(beeName string) (string, error)
	DeleteBeeName(beeName string) (string, error)
	SubmitBeeName(beeName string) (string, error)
	GetBeeNameSuggestions(amount int64) ([]string, error)
	AcceptBeeNameSuggestion(beeName string) (string, error)
	RejectBeeNameSuggestion(beeName string) (string, error)
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
func (s *store) GetBeeName() (string, error) {
	var beeName string
	err := s.db.QueryRow(context.Background(), "SELECT name FROM bee_name ORDER BY random() LIMIT 1").Scan(&beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}

// UploadBeeName uploads a bee name to the database
func (s *store) UploadBeeName(beeName string) (string, error) {
	_, err := s.db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}

// DeleteBeeName deletes a bee name from the database
func (s *store) DeleteBeeName(beeName string) (string, error) {
	_, err := s.db.Exec(context.Background(), "DELETE FROM bee_name WHERE name = $1", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}

// SubmitBeeName submits a bee name to the suggestion database
func (s *store) SubmitBeeName(beeName string) (string, error) {
	_, err := s.db.Exec(context.Background(), "INSERT INTO bee_name_suggestion (name) VALUES ($1)", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}

// GetBeeNameSuggestions returns a list of bee name suggestions
func (s *store) GetBeeNameSuggestions(amount int64) ([]string, error) {
	var beeNames []string
	rows, err := s.db.Query(context.Background(), "SELECT name FROM bee_name_suggestion ORDER BY random() LIMIT $1", amount)
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

	if len(beeNames) == 0 {
		return []string{}, err
	}
	return beeNames, nil
}

// AcceptBeeNameSuggestion accepts a bee name suggestion
func (s *store) AcceptBeeNameSuggestion(beeName string) (string, error) {
	_, err := s.db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return "", err
	}

	_, err = s.db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}

// RejectBeeNameSuggestion rejects a bee name suggestion
func (s *store) RejectBeeNameSuggestion(beeName string) (string, error) {
	_, err := s.db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return "", err
	}
	return beeName, nil
}
