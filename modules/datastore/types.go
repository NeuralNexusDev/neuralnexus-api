package datastore

import (
	"time"
)

// Store - Data Store
type Store struct {
	StoreID   string    `db:"store_id" json:"store_id" xml:"store_id"`
	OwnerID   string    `db:"owner_id" json:"owner_id" xml:"owner_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at" xml:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at" xml:"updated_at"`
}

// NewDataStore - Create a new Data store
func NewDataStore(storeID, ownerID string) *Store {
	return &Store{
		StoreID: storeID,
		OwnerID: ownerID,
	}
}
