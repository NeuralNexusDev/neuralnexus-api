package beenamegenerator

import (
	"context"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BNGStore - Bee Name Generator Store
type BNGStore interface {
	GetBeeName() database.Response[string]
	UploadBeeName(beeName string) database.Response[string]
	DeleteBeeName(beeName string) database.Response[string]
	SubmitBeeName(beeName string) database.Response[string]
	GetBeeNameSuggestions(amount int64) database.Response[[]string]
	AcceptBeeNameSuggestion(beeName string) database.Response[string]
	RejectBeeNameSuggestion(beeName string) database.Response[string]
}

// store - Bee Name Generator Store PG implementation
type store struct {
	db *pgxpool.Pool
}

// NewStore - Create a new bee name generator store
func NewStore(db *pgxpool.Pool) *store {
	return &store{db: db}
}

// GetBeeName returns a random bee name from the database
func (s *store) GetBeeName() database.Response[string] {
	var beeName string
	err := s.db.QueryRow(context.Background(), "SELECT name FROM bee_name ORDER BY random() LIMIT 1").Scan(&beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to get bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// UploadBeeName uploads a bee name to the database
func (s *store) UploadBeeName(beeName string) database.Response[string] {
	_, err := s.db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to upload bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// DeleteBeeName deletes a bee name from the database
func (s *store) DeleteBeeName(beeName string) database.Response[string] {
	_, err := s.db.Exec(context.Background(), "DELETE FROM bee_name WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to delete bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// SubmitBeeName submits a bee name to the suggestion database
func (s *store) SubmitBeeName(beeName string) database.Response[string] {
	_, err := s.db.Exec(context.Background(), "INSERT INTO bee_name_suggestion (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to submit bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// GetBeeNameSuggestions returns a list of bee name suggestions
func (s *store) GetBeeNameSuggestions(amount int64) database.Response[[]string] {
	var beeNames []string
	rows, err := s.db.Query(context.Background(), "SELECT name FROM bee_name_suggestion ORDER BY random() LIMIT $1", amount)
	if err != nil {
		return database.ErrorResponse[[]string]("Failed to get bee name suggestions", err)
	}
	defer rows.Close()

	for rows.Next() {
		var beeName string
		err := rows.Scan(&beeName)
		if err != nil {
			return database.ErrorResponse[[]string]("Failed to get bee name suggestions", err)
		}
		beeNames = append(beeNames, beeName)
	}

	if len(beeNames) == 0 {
		return database.ErrorResponse[[]string]("No bee name suggestions found", err)
	}
	return database.SuccessResponse(beeNames)
}

// AcceptBeeNameSuggestion accepts a bee name suggestion
func (s *store) AcceptBeeNameSuggestion(beeName string) database.Response[string] {
	_, err := s.db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to accept bee name suggestion", err)
	}

	_, err = s.db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to accept bee name suggestion", err)
	}
	return database.SuccessResponse(beeName)
}

// RejectBeeNameSuggestion rejects a bee name suggestion
func (s *store) RejectBeeNameSuggestion(beeName string) database.Response[string] {
	_, err := s.db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to reject bee name suggestion", err)
	}
	return database.SuccessResponse(beeName)
}
