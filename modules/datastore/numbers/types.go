package numbersds

// NumberData - Number Data
type NumberData struct {
	StoreID string  `db:"store_id" json:"store_id" xml:"store_id" validate:"required"`
	UserID  string  `db:"user_id" json:"user_id" xml:"user_id" validate:"required"`
	Value   float64 `db:"value" json:"value" xml:"value"`
}

// NewNumberData - Create a new NumberData
func NewNumberData(storeID, userID string, value float64) *NumberData {
	return &NumberData{
		StoreID: storeID,
		UserID:  userID,
		Value:   value,
	}
}
