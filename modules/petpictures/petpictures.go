package petpictures

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

// CREATE TABLE pictures(
//     id uuid not null primary key default gen_random_uuid(),
//     md5 text not null,
//     file_ext text not null,
//     subjects integer[] not null,
//     aliases text[],
//     created_at timestamp with time zone default current_timestamp,
//     CONSTRAINT md5_check UNIQUE ( md5 ),
//     CONSTRAINT subjects_check CHECK ( subjects <> '{}' )
// );

// CREATE TABLE pets(
//     id serial not null primary key,
//     name text not null,
//     profile_picture uuid,
//     created_at timestamp with time zone default current_timestamp,
//     CONSTRAINT name_check UNIQUE ( name )
// );

// -------------- Globals --------------
var (
	CDN_URL  = "https://cdn.neuralnexus.dev"
	CDN_PATH = "/petpictures/"
	CDN_KEY  = os.Getenv("CDN_KEY")
)

// -------------- Structs --------------
// UUID - UUID type alias
type UUID string

// PetPicture - Pet picture struct
type PetPicture struct {
	ID       UUID     `json:"id"`
	MD5      string   `json:"md5"`
	FileExt  string   `json:"file_ext"`
	Subjects []int    `json:"subjects"`
	Aliases  []string `json:"aliases"`
	Created  string   `json:"created"`
}

// GetPetPictureURL - Get the URL for a pet picture
func (p *PetPicture) GetPetPictureURL() string {
	return CDN_URL + CDN_PATH + string(p.ID) + "." + p.FileExt
}

// Pet - Pet struct
type Pet struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// -------------- DB Functions --------------
// createPet - Create a new pet
func createPet(name string) (Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"INSERT INTO pets (name) VALUES ($1) RETURNING id, name, profile_picture",
		name,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		return Pet{}, err
	}

	return pet, nil
}

// getPet - Get a pet by ID
func getPet(id int) (Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"SELECT id, name, profile_picture FROM pets WHERE id = $1",
		id,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		return Pet{}, err
	}

	return pet, nil
}

// getPetByName - Get a pet by name
func getPetByName(name string) (Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"SELECT id, name, profile_picture FROM pets WHERE name = $1",
		name,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		return Pet{}, err
	}

	return pet, nil
}

// updatePet - Update a pet
func updatePet(id int, name string, picture UUID) (Pet, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"UPDATE pets SET name = $1, profile_picture = $2 WHERE id = $3 RETURNING id, name, profile_picture",
		name, picture, id,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		return Pet{}, err
	}

	return pet, nil
}

// createPetPicture - Create a new pet picture
func createPetPicture(md5 string, fileExt string, subjects []int, aliases []string) (PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"INSERT INTO pictures (md5, file_ext, subjects, aliases) VALUES ($1, $2, $3, $4) RETURNING id, md5, file_ext, subjects, aliases, created_at",
		md5, fileExt, subjects, aliases,
	).Scan(&petPicture.ID, &petPicture.MD5, &petPicture.FileExt, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		return PetPicture{}, err
	}

	return petPicture, nil
}

// getRandPetPicture - Get a random pet picture
func getRandPetPicture() (PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"SELECT id, md5, subjects, aliases, created_at FROM pictures ORDER BY random() LIMIT 1",
	).Scan(&petPicture.ID, &petPicture.MD5, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		return PetPicture{}, err
	}

	return petPicture, nil
}

// getPetPicture - Get a pet picture by ID
func getPetPicture(id UUID) (PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"SELECT id, md5, subjects, aliases, created_at FROM pictures WHERE id = $1",
		id,
	).Scan(&petPicture.ID, &petPicture.MD5, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		return PetPicture{}, err
	}

	return petPicture, nil
}

// getPetPictureByMD5 - Get a pet picture by MD5
func getPetPictureByMD5(md5 string) (PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"SELECT id, md5, subjects, aliases, created_at FROM pictures WHERE md5 = $1",
		md5,
	).Scan(&petPicture.ID, &petPicture.MD5, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		return PetPicture{}, err
	}

	return petPicture, nil
}

// updatePetPicture - Update a pet picture
func updatePetPicture(id UUID, md5 string, fileExt string, subjects []int, aliases []string) (PetPicture, error) {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"UPDATE pictures SET md5 = $1, file_ext = $2, subjects = $3, aliases = $4 WHERE id = $5 RETURNING id, md5, file_ext, subjects, aliases, created_at",
		md5, fileExt, subjects, aliases, id,
	).Scan(&petPicture.ID, &petPicture.MD5, &petPicture.FileExt, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		return PetPicture{}, err
	}

	return petPicture, nil
}

// -------------- Functions --------------

// UploadPetPicture - Upload a pet picture
func UploadPetPicture(file *os.File, subjects []int, aliases []string) (PetPicture, error) {
	// Get MD5 hash
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return PetPicture{}, err
	}
	md5 := hex.EncodeToString(hash.Sum(nil))

	splitName := strings.Split(file.Name(), ".")
	fileExt := splitName[len(splitName)-1]

	petPicture, err := createPetPicture(md5, fileExt, subjects, aliases)
	if err != nil {
		return PetPicture{}, err
	}

	// Update file name
	newFileName := string(petPicture.ID) + "." + fileExt
	if err := os.Rename(file.Name(), newFileName); err != nil {
		return PetPicture{}, err
	}

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, CDN_URL+"/upload", nil)
	if err != nil {
		return PetPicture{}, err
	}

	// Create a new multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Write the form data
	writer.WriteField("upload_key", CDN_KEY)
	writer.WriteField("upload_path", CDN_PATH)

	// Create a new form file
	part, err := writer.CreateFormFile("file", newFileName)
	if err != nil {
		return PetPicture{}, err
	}

	// Copy the file to the form file
	if _, err := io.Copy(part, file); err != nil {
		return PetPicture{}, err
	}
	writer.Close()

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Body = io.NopCloser(body)

	// Send the request
	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return PetPicture{}, err
	}

	return petPicture, nil
}

// -------------- Routes --------------

// ApplyRoutes - Apply routes to the router
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	mux.HandleFunc("POST /petpictures/pets/{name}", CreatePetHandler)
	return mux
}

// CreatePetHandler - Create a new pet
func CreatePetHandler(w http.ResponseWriter, r *http.Request) {
	petName := r.PathValue("name")
	if petName == "" {
		var pet Pet
		err := json.NewDecoder(r.Body).Decode(&pet)
		if err == nil {
			petName = pet.Name
		}
	}
	if petName == "" {
		http.Error(w, "Invalid pet name", http.StatusBadRequest)
		return
	}
	// TODO: Finish this and use database type to return errors
}
