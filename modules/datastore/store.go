package datastore

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CREATE TRIGGER update_datastores_modtime
// BEFORE UPDATE ON datastores
// FOR EACH ROW
// EXECUTE PROCEDURE update_modified_column();

// CREATE TABLE datastores (
//  store_id UUID PRIMARY KEY NOT NULL,
// 	owner_id UUID NOT NULL,
// 	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
// 	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
//  FOREIGN KEY (owner_id) REFERENCES accounts(user_id)
// );

// DSStore - Data Store Interface
type DSStore interface {
	CreateNewDataStore(storeID, userID uuid.UUID) (*Store, error)
	GetDataStore(storeID uuid.UUID) (*Store, error)
	UpdateDataStore(storeID, userID uuid.UUID) (*Store, error)
	DeleteDataStore(storeID uuid.UUID) error
}

// DataStore - Data Store
type dataStore struct {
	db *pgxpool.Pool
}

// NewStore - Create a new Data store
func NewStore(db *pgxpool.Pool) DSStore {
	return &dataStore{db: db}
}

// RunQueryAndReturn - Run a query and return the result
func RunQueryAndReturn(db *pgxpool.Pool, query string, args ...any) (*Store, error) {
	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}

	var data *Store
	data, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Store])
	if err != nil {
		return nil, err
	}
	return data, nil
}

// CreateNewDataStore - Create a new Data store
func (s *dataStore) CreateNewDataStore(storeID, ownerID uuid.UUID) (*Store, error) {
	return RunQueryAndReturn(s.db, "INSERT INTO datastores (store_id, owner_id) VALUES ($1, $2) RETURNING *", storeID, ownerID)
}

// GetDataStore - Get a Data store
func (s *dataStore) GetDataStore(storeID uuid.UUID) (*Store, error) {
	return RunQueryAndReturn(s.db, "SELECT * FROM datastores WHERE store_id = $1", storeID)
}

// UpdateDataStore - Update a Data store
func (s *dataStore) UpdateDataStore(storeID uuid.UUID, ownerID uuid.UUID) (*Store, error) {
	return RunQueryAndReturn(s.db, "UPDATE datastores SET owner_id = $2 WHERE store_id = $1 RETURNING *", storeID, ownerID)
}

// DeleteDataStore - Delete a Data store
func (s *dataStore) DeleteDataStore(storeID uuid.UUID) error {
	_, err := s.db.Exec(context.Background(), "DELETE FROM datastores WHERE store_id = $1", storeID)
	if err != nil {
		return err
	}
	return nil
}
