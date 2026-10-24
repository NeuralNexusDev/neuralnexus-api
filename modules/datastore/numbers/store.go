package numbersds

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNumberNotFound = errors.New("number not found")

// NumberStore - Number Store
type NumberStore interface {
	Create(ctx context.Context, storeID, userID string, initVal float64) (float64, error)
	Read(ctx context.Context, storeID, userID string) (float64, error)
	Update(ctx context.Context, storeID, userID string, newVal float64) (float64, error)
	Delete(ctx context.Context, storeID, userID string) error
	Add(ctx context.Context, storeID, userID string, value float64) error
}

// numberStore - Number Store
type numberStore struct {
	db *pgxpool.Pool
}

// NewStore - Create a new Number store
func NewStore(db *pgxpool.Pool) NumberStore {
	return &numberStore{db: db}
}

// Add - Add a value to an existing entry in the datastore
func (s *numberStore) Add(ctx context.Context, storeID, userID string, value float64) error {
	_, err := s.db.Exec(ctx, "UPDATE datastore_numbers SET value = value + $1 WHERE store_id = $2 AND user_id = $3", value, storeID, userID)
	if err != nil {
		return err
	}
	return nil
}

// Create - Create a new entry in the datastore
func (s *numberStore) Create(ctx context.Context, storeID, userID string, initVal float64) (float64, error) {
	_, err := s.db.Exec(ctx, "INSERT INTO datastore_numbers (store_id, user_id, value) VALUES ($1, $2, $3)", storeID, userID, initVal)
	if err != nil {
		return 0, err
	}
	return initVal, nil
}

// Read - Read an entry from the datastore
func (s *numberStore) Read(ctx context.Context, storeID, userID string) (float64, error) {
	var value float64
	err := s.db.QueryRow(ctx, "SELECT value FROM datastore_numbers WHERE store_id = $1 AND user_id = $2", storeID, userID).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNumberNotFound
		}
		return 0, err
	}
	return value, nil
}

// Update - Update an entry in the datastore
func (s *numberStore) Update(ctx context.Context, storeID, userID string, value float64) (float64, error) {
	_, err := s.db.Exec(ctx, "UPDATE datastore_numbers SET value = $1 WHERE store_id = $2 AND user_id = $3", value, storeID, userID)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// Delete - Delete an entry from the datastore
func (s *numberStore) Delete(ctx context.Context, storeID, userID string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM datastore_numbers WHERE store_id = $1 AND user_id = $2", storeID, userID)
	if err != nil {
		return err
	}
	return nil
}
