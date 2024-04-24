package datastore

// DSService - Data Store Service
type DSService interface {
	Create(*Store) (*Store, error)
	Read(*Store) (*Store, error)
	Update(*Store) (*Store, error)
	Delete(*Store) error
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
func (s *dsService) Create(data *Store) (*Store, error) {
	return s.store.CreateNewDataStore(data.StoreID, data.OwnerID)
}

// Read - Read an entry from the datastore
func (s *dsService) Read(data *Store) (*Store, error) {
	return s.store.GetDataStore(data.StoreID)
}

// Update - Update an entry in the datastore
func (s *dsService) Update(data *Store) (*Store, error) {
	return s.store.UpdateDataStore(data.StoreID, data.OwnerID)
}

// Delete - Delete an entry from the datastore
func (s *dsService) Delete(data *Store) error {
	return s.store.DeleteDataStore(data.StoreID)
}
