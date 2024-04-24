package datastore

import (
	"time"

	"github.com/google/uuid"
)

// Store - Data Store
type Store struct {
	StoreID   uuid.UUID `db:"store_id" json:"store_id" xml:"store_id"`
	OwnerID   uuid.UUID `db:"owner_id" json:"owner_id" xml:"owner_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at" xml:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at" xml:"updated_at"`
}

// NewStore - Create a new Data store
func NewDataStore(storeID, ownerID uuid.UUID) *Store {
	return &Store{
		StoreID: storeID,
		OwnerID: ownerID,
	}
}
