package numbersds

import "context"

// -------------- Structs --------------

// NumberService - Number Service
type NumberService interface {
	Add(context.Context, *NumberData, float64) error
	Create(context.Context, *NumberData) (*NumberData, error)
	Read(context.Context, *NumberData) (*NumberData, error)
	Update(context.Context, *NumberData) (*NumberData, error)
	Delete(context.Context, *NumberData) error
}

// numberService - Number Service implementation
type numberService struct {
	store NumberStore
}

// NewService - Create a new Number service
func NewService(store NumberStore) NumberService {
	return &numberService{store: store}
}

// Add - Add a number to an existing entry in the datastore, and update the value
func (s *numberService) Add(ctx context.Context, data *NumberData, value float64) error {
	data.Value += value
	return s.store.Add(ctx, data.StoreID, data.UserID, value)
}

// Create - Create a new entry in the datastore
func (s *numberService) Create(ctx context.Context, data *NumberData) (*NumberData, error) {
	val, err := s.store.Create(ctx, data.StoreID, data.UserID, data.Value)
	if err != nil {
		return nil, err
	}
	return NewNumberData(data.StoreID, data.UserID, val), nil
}

// Read - Read an entry from the datastore
func (s *numberService) Read(ctx context.Context, data *NumberData) (*NumberData, error) {
	val, err := s.store.Read(ctx, data.StoreID, data.UserID)
	if err != nil {
		return nil, err
	}
	return NewNumberData(data.StoreID, data.UserID, val), nil
}

// Update - Update an entry in the datastore
func (s *numberService) Update(ctx context.Context, data *NumberData) (*NumberData, error) {
	val, err := s.store.Update(ctx, data.StoreID, data.UserID, data.Value)
	if err != nil {
		return nil, err
	}
	return NewNumberData(data.StoreID, data.UserID, val), nil
}

// Delete - Delete an entry from the datastore
func (s *numberService) Delete(ctx context.Context, data *NumberData) error {
	return s.store.Delete(ctx, data.StoreID, data.UserID)
}
