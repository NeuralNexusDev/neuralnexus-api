package numbersds

import "github.com/google/uuid"

// -------------- Structs --------------

// NumberData - Number Data
type NumberData struct {
	StoreID uuid.UUID `db:"store_id" json:"store_id" xml:"store_id" validate:"required"`
	UserID  uuid.UUID `db:"user_id" json:"user_id" xml:"user_id" validate:"required"`
	Value   float64   `db:"value" json:"value" xml:"value"`
}

// NewNumberData - Create a new NumberData
func NewNumberData(storeID, userID uuid.UUID, value float64) *NumberData {
	return &NumberData{
		StoreID: storeID,
		UserID:  userID,
		Value:   value,
	}
}
