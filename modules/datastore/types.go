package datastore

import "github.com/google/uuid"

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

// -------------- Structs --------------

type DataStore interface {
	Create(storeID, userID uuid.UUID, initVal any) error
	Read(storeID, userID uuid.UUID) (any, error)
	Update(storeID, userID uuid.UUID, newVal any) error
	Delete(storeID, userID uuid.UUID) error
}
