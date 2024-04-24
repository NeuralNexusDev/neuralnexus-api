package numbersds

// -------------- Structs --------------

// NumberService - Number Service
type NumberService interface {
	Add(*NumberData, float64) error
	Create(*NumberData) (*NumberData, error)
	Read(*NumberData) (*NumberData, error)
	Update(*NumberData) (*NumberData, error)
	Delete(*NumberData) error
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
func (s *numberService) Add(data *NumberData, value float64) error {
	data.Value += value
	return s.store.Add(data.StoreID, data.UserID, value)
}

// Create - Create a new entry in the datastore
func (s *numberService) Create(data *NumberData) (*NumberData, error) {
	val, err := s.store.Create(data.StoreID, data.UserID, data.Value)
	if err != nil {
		return nil, err
	}
	return NewNumberData(data.StoreID, data.UserID, val), nil
}

// Read - Read an entry from the datastore
func (s *numberService) Read(data *NumberData) (*NumberData, error) {
	val, err := s.store.Read(data.StoreID, data.UserID)
	if err != nil {
		return nil, err
	}
	return NewNumberData(data.StoreID, data.UserID, val), nil
}

// Update - Update an entry in the datastore
func (s *numberService) Update(data *NumberData) (*NumberData, error) {
	val, err := s.store.Update(data.StoreID, data.UserID, data.Value)
	if err != nil {
		return nil, err
	}
	return NewNumberData(data.StoreID, data.UserID, val), nil
}

// Delete - Delete an entry from the datastore
func (s *numberService) Delete(data *NumberData) error {
	return s.store.Delete(data.StoreID, data.UserID)
}
