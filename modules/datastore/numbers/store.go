package numbersds

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CREATE TRIGGER update_datastore_numbers_modtime
// BEFORE UPDATE ON datastore_numbers
// FOR EACH ROW
// EXECUTE PROCEDURE update_modified_column();

// CREATE TABLE datastore_numbers (
//  store_id UUID PRIMARY KEY NOT NULL,
// 	user_id UUID NOT NULL,
// 	value FLOAT NOT NULL,
// 	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
// 	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
//  FOREIGN KEY (store_id) REFERENCES datastores(store_id),
//  FOREIGN KEY (user_id) REFERENCES accounts(user_id)
// );

// NumberStore - Number Store
type NumberStore interface {
	Create(storeID, userID uuid.UUID, initVal float64) (float64, error)
	Read(storeID, userID uuid.UUID) (float64, error)
	Update(storeID, userID uuid.UUID, newVal float64) (float64, error)
	Delete(storeID, userID uuid.UUID) error
	Add(storeID, userID uuid.UUID, value float64) error
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
func (s *numberStore) Add(storeID, userID uuid.UUID, value float64) error {
	_, err := s.db.Exec(context.Background(), "UPDATE datastore_numbers SET value = value + $1 WHERE store_id = $2 AND user_id = $3", value, storeID, userID)
	if err != nil {
		return err
	}
	return nil
}

// Create - Create a new entry in the datastore
func (s *numberStore) Create(storeID, userID uuid.UUID, initVal float64) (float64, error) {
	_, err := s.db.Exec(context.Background(), "INSERT INTO datastore_numbers (store_id, user_id, value) VALUES ($1, $2, $3)", storeID, userID, initVal)
	if err != nil {
		return 0, err
	}
	return initVal, nil
}

// Read - Read an entry from the datastore
func (s *numberStore) Read(storeID, userID uuid.UUID) (float64, error) {
	var value float64
	err := s.db.QueryRow(context.Background(), "SELECT value FROM datastore_numbers WHERE store_id = $1 AND user_id = $2", storeID, userID).Scan(&value)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// Update - Update an entry in the datastore
func (s *numberStore) Update(storeID, userID uuid.UUID, value float64) (float64, error) {
	_, err := s.db.Exec(context.Background(), "UPDATE datastore_numbers SET value = $1 WHERE store_id = $2 AND user_id = $3", value, storeID, userID)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// Delete - Delete an entry from the datastore
func (s *numberStore) Delete(storeID, userID uuid.UUID) error {
	_, err := s.db.Exec(context.Background(), "DELETE FROM datastore_numbers WHERE store_id = $1 AND user_id = $2", storeID, userID)
	if err != nil {
		return err
	}
	return nil
}
