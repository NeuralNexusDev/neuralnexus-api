package petpictures

import (
	"context"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CREATE TABLE pictures (
//     id text not null primary key,
//     file_ext text not null,
//     prime_subj integer not null,
//     othr_subj integer[],
//     aliases text[],
//     created_at timestamp with time zone default current_timestamp,
//     CONSTRAINT id_check UNIQUE ( id )
// );

// CREATE TABLE pets (
//     id serial not null primary key,
//     name text not null,
//     profile_picture text default null,
//     created_at timestamp with time zone default current_timestamp,
//     CONSTRAINT name_check UNIQUE ( name )
// );

// PetPicStore - Pet Picture Store
type PetPicStore interface {
	CreatePet(name string) (*Pet, error)
	GetPet(id int) (*Pet, error)
	GetPetByName(name string) (*Pet, error)
	UpdatePet(pet Pet) (*Pet, error)
	CreatePetPicture(id string, fileExt string, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error)
	GetRandPetPictureByName(name string) (*PetPicture, error)
	GetPetPicture(id string) (*PetPicture, error)
	UpdatePetPicture(picture PetPicture) (*PetPicture, error)
	DeletePetPicture(id string) (*PetPicture, error)
}

// store - Pet Picture Store PG implementation
type store struct {
	db *pgxpool.Pool
}

// NewStore - Create a new Pet Picture store
func NewStore(db *pgxpool.Pool) PetPicStore {
	return &store{db: db}
}

// CreatePet - Create a new pet
func (s *store) CreatePet(name string) (*Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"INSERT INTO pets (name) VALUES ($1) RETURNING id, name, profile_picture", name,
	).Scan(&pet.ID, &pet.Name, &pet.ProfilePicture)
	if err != nil {
		return nil, err
	}
	return &pet, nil
}

// GetPet - Get a pet by ID
func (s *store) GetPet(id int) (*Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(), "SELECT * FROM pets WHERE id = $1", id).Scan(&pet.ID, &pet.Name, &pet.ProfilePicture)
	if err != nil {
		return nil, err
	}
	return &pet, nil
}

// GetPetByName - Get a pet by name
func (s *store) GetPetByName(name string) (*Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(), "SELECT id, name, profile_picture FROM pets WHERE name = $1", name).Scan(&pet.ID, &pet.Name, &pet.ProfilePicture)
	if err != nil {
		return nil, err
	}
	return &pet, nil
}

// UpdatePet - Update a pet
func (s *store) UpdatePet(pet Pet) (*Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	_, err := db.Query(context.Background(), "UPDATE pets SET name = $1, profile_picture = $2 WHERE id = $3", pet.Name, pet.ProfilePicture, pet.ID)
	if err != nil {
		return nil, err
	}
	return &pet, nil
}

// CreatePetPicture - Create a new pet picture
func (s *store) CreatePetPicture(id string, fileExt string, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	_, err := db.Query(context.Background(),
		"INSERT INTO pictures (id, file_ext, prime_subj, othr_subj, aliases) VALUES ($1, $2, $3, $4, $5)",
		id, fileExt, primarySubject, othersSubjects, aliases,
	)
	if err != nil {
		return nil, err
	}
	return &PetPicture{
		ID:             id,
		FileExt:        fileExt,
		PrimarySubject: primarySubject,
		OthersSubjects: othersSubjects,
		Aliases:        aliases,
	}, nil
}

// GetRandPetPictureByName - Get a random pet picture by name
func (s *store) GetRandPetPictureByName(name string) (*PetPicture, error) {
	pet, err := s.GetPetByName(name)
	if err != nil {
		return nil, err
	}

	db := database.GetDB("pet_pictures")
	defer db.Close()

	rows, err := db.Query(context.Background(),
		"SELECT * FROM pictures WHERE prime_subj = $1 OR $2 = ANY(othr_subj) ORDER BY random() LIMIT 1", pet.ID, pet.ID)
	if err != nil {
		return nil, err
	}

	var picture *PetPicture
	picture, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[PetPicture])
	if err != nil {
		return nil, err
	}
	return picture, nil
}

// GetPetPicture - Get a pet picture by ID
func (s *store) GetPetPicture(id string) (*PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM pictures WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	var picture *PetPicture
	picture, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[PetPicture])
	if err != nil {
		return nil, err
	}
	return picture, nil
}

// UpdatePetPicture - Update a pet picture
func (s *store) UpdatePetPicture(picture PetPicture) (*PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	_, err := db.Query(context.Background(),
		"UPDATE pictures SET file_ext = $1, prime_subj = $2, othr_subj = $3, aliases = $4 WHERE id = $5",
		picture.FileExt, picture.PrimarySubject, picture.OthersSubjects, picture.Aliases, picture.ID,
	)
	if err != nil {
		return nil, err
	}
	return &petPicture, nil
}

// DeletePetPicture - Delete a pet picture
func (s *store) DeletePetPicture(id string) (*PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	_, err := db.Query(context.Background(), "DELETE FROM pictures WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &PetPicture{ID: id}, nil
}
