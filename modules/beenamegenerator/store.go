package beenamegenerator

import (
	"context"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

// getBeeName returns a random bee name from the database
func (s *Store) getBeeName() database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	var beeName string
	err := db.QueryRow(context.Background(), "SELECT name FROM bee_name ORDER BY random() LIMIT 1").Scan(&beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to get bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// uploadBeeName uploads a bee name to the database
func (s *Store) uploadBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to upload bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// deleteBeeName deletes a bee name from the database
func (s *Store) deleteBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "DELETE FROM bee_name WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to delete bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// submitBeeName submits a bee name to the suggestion database
func (s *Store) submitBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name_suggestion (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to submit bee name", err)
	}
	return database.SuccessResponse(beeName)
}

// getBeeNameSuggestions returns a list of bee name suggestions
func (s *Store) getBeeNameSuggestions(amount int64) database.Response[[]string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	var beeNames []string
	rows, err := db.Query(context.Background(), "SELECT name FROM bee_name_suggestion ORDER BY random() LIMIT $1", amount)
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

// acceptBeeNameSuggestion accepts a bee name suggestion
func (s *Store) acceptBeeNameSuggestion(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to accept bee name suggestion", err)
	}

	_, err = db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to accept bee name suggestion", err)
	}
	return database.SuccessResponse(beeName)
}

// rejectBeeNameSuggestion rejects a bee name suggestion
func (s *Store) rejectBeeNameSuggestion(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to reject bee name suggestion", err)
	}
	return database.SuccessResponse(beeName)
}
