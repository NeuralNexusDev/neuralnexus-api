package datastore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDataStoreNotFound      = errors.New("data store not found")
	ErrDataStoreInUse         = errors.New("data store still holds data")
	ErrDataStoreOwnerNotFound = errors.New("data store owner not found")
	ErrDataStoreInvalidID     = errors.New("data store or owner id is not a number")
)

func translateDataStoreErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "22P02":
		return ErrDataStoreInvalidID
	case pgErr.Code == "23503" && pgErr.ConstraintName == "datastores_owner_id_fkey":
		return ErrDataStoreOwnerNotFound
	case pgErr.Code == "23503":
		return ErrDataStoreInUse
	}
	return err
}

// DSStore - Data Store Interface
type DSStore interface {
	CreateNewDataStore(ctx context.Context, storeID, userID string) (*Store, error)
	GetDataStore(ctx context.Context, storeID string) (*Store, error)
	UpdateDataStore(ctx context.Context, storeID, userID string) (*Store, error)
	DeleteDataStore(ctx context.Context, storeID string) error
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
func RunQueryAndReturn(ctx context.Context, db *pgxpool.Pool, query string, args ...any) (*Store, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, translateDataStoreErr(err)
	}

	var data *Store
	data, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Store])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDataStoreNotFound
		}
		return nil, translateDataStoreErr(err)
	}
	return data, nil
}

// CreateNewDataStore - Create a new Data store
func (s *dataStore) CreateNewDataStore(ctx context.Context, storeID, ownerID string) (*Store, error) {
	return RunQueryAndReturn(ctx, s.db, "INSERT INTO datastores (store_id, owner_id) VALUES ($1, $2) RETURNING *", storeID, ownerID)
}

// GetDataStore - Get a Data store
func (s *dataStore) GetDataStore(ctx context.Context, storeID string) (*Store, error) {
	return RunQueryAndReturn(ctx, s.db, "SELECT * FROM datastores WHERE store_id = $1", storeID)
}

// UpdateDataStore - Update a Data store
func (s *dataStore) UpdateDataStore(ctx context.Context, storeID string, ownerID string) (*Store, error) {
	return RunQueryAndReturn(ctx, s.db, "UPDATE datastores SET owner_id = $2 WHERE store_id = $1 RETURNING *", storeID, ownerID)
}

// DeleteDataStore - Delete a Data store
func (s *dataStore) DeleteDataStore(ctx context.Context, storeID string) error {
	tag, err := s.db.Exec(ctx, "DELETE FROM datastores WHERE store_id = $1", storeID)
	if err != nil {
		return translateDataStoreErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDataStoreNotFound
	}
	return nil
}
