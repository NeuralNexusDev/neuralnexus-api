package datastore

import "context"

// DSService - Data Store Service
type DSService interface {
	Create(context.Context, *Store) (*Store, error)
	Read(context.Context, *Store) (*Store, error)
	Update(context.Context, *Store) (*Store, error)
	Delete(context.Context, *Store) error
}

// dsService - Data Store Service implementation
type dsService struct {
	store DSStore
}

// NewService - Create a new Data Store service
func NewService(store DSStore) DSService {
	return &dsService{store: store}
}

// Create - Create a new entry in the datastore
func (s *dsService) Create(ctx context.Context, data *Store) (*Store, error) {
	return s.store.CreateNewDataStore(ctx, data.StoreID, data.OwnerID)
}

// Read - Read an entry from the datastore
func (s *dsService) Read(ctx context.Context, data *Store) (*Store, error) {
	return s.store.GetDataStore(ctx, data.StoreID)
}

// Update - Update an entry in the datastore
func (s *dsService) Update(ctx context.Context, data *Store) (*Store, error) {
	return s.store.UpdateDataStore(ctx, data.StoreID, data.OwnerID)
}

// Delete - Delete an entry from the datastore
func (s *dsService) Delete(ctx context.Context, data *Store) error {
	return s.store.DeleteDataStore(ctx, data.StoreID)
}
