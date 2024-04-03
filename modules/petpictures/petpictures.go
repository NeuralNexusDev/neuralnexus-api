package petpictures

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// CREATE TABLE pictures(
//     id text not null primary key,
//     file_ext text not null,
//     subjects integer[] not null,
//     aliases text[],
//     created_at timestamp with time zone default current_timestamp,
//     CONSTRAINT id_check UNIQUE ( id ),
//     CONSTRAINT subjects_check CHECK ( subjects <> '{}' )
// );

// CREATE TABLE pets(
//     id serial not null primary key,
//     name text not null,
//     profile_picture text default null,
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
// PetPicture - Pet picture struct
type PetPicture struct {
	ID       string   `json:"id"`
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
	ID             int    `json:"id"`
	Name           string `json:"name"`
	ProfilePicture string `json:"profile_picture"`
}

// APIResponse - API response struct
type APIResponse[T Pet | PetPicture] struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}

// -------------- DB Functions --------------
// createPet - Create a new pet
func createPet(name string) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"INSERT INTO pets (name) VALUES ($1) RETURNING id, name, profile_picture",
		name,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		log.Println("Unable to create pet:", err)
		return database.Response[Pet]{
			Success: false,
			Message: "Unable to create pet (pet may already exist)",
		}
	}

	return database.Response[Pet]{
		Success: true,
		Data:    pet,
	}
}

// getPet - Get a pet by ID
func getPet(id int) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"SELECT id, name, profile_picture FROM pets WHERE id = $1",
		id,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		log.Println("Unable to get pet:", err)
		return database.Response[Pet]{
			Success: false,
			Message: "Unable to get pet",
		}
	}

	return database.Response[Pet]{
		Success: true,
		Data:    pet,
	}
}

// getPetByName - Get a pet by name
func getPetByName(name string) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"SELECT id, name, profile_picture FROM pets WHERE name = $1",
		name,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		log.Println("Unable to get pet:", err)
		return database.Response[Pet]{
			Success: false,
			Message: "Unable to get pet",
		}
	}

	return database.Response[Pet]{
		Success: true,
		Data:    pet,
	}
}

// updatePet - Update a pet
func updatePet(id int, name string, picture string) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"UPDATE pets SET name = $1, profile_picture = $2 WHERE id = $3 RETURNING id, name, profile_picture",
		name, picture, id,
	).Scan(&pet.ID, &pet.Name)
	if err != nil {
		log.Println("Unable to update pet:", err)
		return database.Response[Pet]{
			Success: false,
			Message: "Unable to update pet",
		}
	}

	return database.Response[Pet]{
		Success: true,
		Data:    pet,
	}
}

// createPetPicture - Create a new pet picture
func createPetPicture(md5 string, fileExt string, subjects []int, aliases []string) database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"INSERT INTO pictures (id, file_ext, subjects, aliases) VALUES ($1, $2, $3, $4) RETURNING id, file_ext, subjects, aliases, created_at",
		md5, fileExt, subjects, aliases,
	).Scan(&petPicture.ID, &petPicture.FileExt, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		log.Println("Unable to create pet picture:", err)
		return database.Response[PetPicture]{
			Success: false,
			Message: "Unable to create pet picture",
		}
	}

	return database.Response[PetPicture]{
		Success: true,
		Data:    petPicture,
	}
}

// getRandPetPicture - Get a random pet picture
func getRandPetPicture() database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"SELECT id, subjects, aliases, created_at FROM pictures ORDER BY random() LIMIT 1",
	).Scan(&petPicture.ID, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		log.Println("Unable to get random pet picture:", err)
		return database.Response[PetPicture]{
			Success: false,
			Message: "Unable to get random pet picture",
		}
	}

	return database.Response[PetPicture]{
		Success: true,
		Data:    petPicture,
	}
}

// getPetPicture - Get a pet picture by ID
func getPetPicture(id string) database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"SELECT id, subjects, aliases, created_at FROM pictures WHERE id = $1",
		id,
	).Scan(&petPicture.ID, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		log.Println("Unable to get pet picture:", err)
		return database.Response[PetPicture]{
			Success: false,
			Message: "Unable to get pet picture",
		}
	}

	return database.Response[PetPicture]{
		Success: true,
		Data:    petPicture,
	}
}

// updatePetPicture - Update a pet picture
func updatePetPicture(id string, fileExt string, subjects []int, aliases []string) database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"UPDATE pictures SET id = $1, file_ext = $2, subjects = $3, aliases = $4 WHERE id = $5 RETURNING id, file_ext, subjects, aliases, created_at",
		id, fileExt, subjects, aliases, id,
	).Scan(&petPicture.ID, &petPicture.FileExt, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		log.Println("Unable to update pet picture:", err)
		return database.Response[PetPicture]{
			Success: false,
			Message: "Unable to update pet picture",
		}
	}

	return database.Response[PetPicture]{
		Success: true,
		Data:    petPicture,
	}
}

// deletePetPicture - Delete a pet picture
func deletePetPicture(id string) database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	err := db.QueryRow(context.Background(),
		"DELETE FROM pictures WHERE id = $1 RETURNING id, file_ext, subjects, aliases, created_at",
		id,
	).Scan(&petPicture.ID, &petPicture.FileExt, &petPicture.Subjects, &petPicture.Aliases, &petPicture.Created)
	if err != nil {
		log.Println("Unable to delete pet picture:", err)
		return database.Response[PetPicture]{
			Success: false,
			Message: "Unable to delete pet picture",
		}
	}

	return database.Response[PetPicture]{
		Success: true,
		Data:    petPicture,
	}
}

// -------------- Functions --------------

// UploadPetPicture - Upload a pet picture
func UploadPetPicture(file *os.File, subjects []int, aliases []string) APIResponse[PetPicture] {
	// Get SHA1 hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		log.Println("Unable to get SHA256 hash:", err)
		return APIResponse[PetPicture]{
			Success: false,
			Message: "Unable to get SHA256 hash",
		}
	}
	sha := hex.EncodeToString(hash.Sum(nil))

	splitName := strings.Split(file.Name(), ".")
	fileExt := splitName[len(splitName)-1]

	petPictureResponse := createPetPicture(sha, fileExt, subjects, aliases)
	if !petPictureResponse.Success {
		return APIResponse[PetPicture]{
			Success: false,
			Message: petPictureResponse.Message,
		}
	}
	petPicture := petPictureResponse.Data

	// Update file name
	newFileName := string(petPicture.ID) + "." + fileExt
	if err := os.Rename(file.Name(), newFileName); err != nil {
		log.Println("Unable to rename file:", err)
		return APIResponse[PetPicture]{
			Success: false,
			Message: "Unable to rename file",
		}
	}

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, CDN_URL+"/upload", nil)
	if err != nil {
		return APIResponse[PetPicture]{
			Success: false,
			Message: "Unable to create request",
		}
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
		return APIResponse[PetPicture]{
			Success: false,
			Message: "Unable to create form file",
		}
	}

	// Copy the file to the form file
	if _, err := io.Copy(part, file); err != nil {
		return APIResponse[PetPicture]{
			Success: false,
			Message: "Unable to copy form file",
		}
	}
	writer.Close()

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Body = io.NopCloser(body)

	// Send the request
	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return APIResponse[PetPicture]{
			Success: false,
			Message: "Unable to send request",
		}
	}

	return APIResponse[PetPicture]{
		Success: true,
		Data:    petPicture,
	}
}

// -------------- Routes --------------

// ApplyRoutes - Apply routes to the router
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	mux.HandleFunc("POST /petpictures/pets/{name}", CreatePetHandler)
	mux.HandleFunc("POST /petpictures/pets", CreatePetHandler)
	mux.HandleFunc("GET /petpictures/pets/{id}", GetPetHandler)
	mux.HandleFunc("GET /petpictures/pets", GetPetHandler)
	mux.HandleFunc("PUT /petpictures/pets", UpdatePetHandler)
	mux.HandleFunc("GET /petpictures/pictures/random", GetRandPetPictureHandler)
	mux.HandleFunc("GET /petpictures/pictures/{id}", GetPetPictureHandler)
	mux.HandleFunc("GET /petpictures/pictures", GetPetPictureHandler)
	mux.HandleFunc("PUT /petpictures/pictures", UpdatePetPictureHandler)
	mux.HandleFunc("DELETE /petpictures/pictures/{id}", DeletePetPictureHandler)
	mux.HandleFunc("DELETE /petpictures/pictures", DeletePetPictureHandler)
	return mux
}

// CreatePetHandler - Create a new pet
func CreatePetHandler(w http.ResponseWriter, r *http.Request) {
	petName := r.PathValue("name")
	if petName == "" {
		var pet Pet
		err := responses.DecodeStruct(r, &pet)
		if err == nil {
			petName = pet.Name
		}
	}
	if petName == "" {
		problem := responses.NewProblemResponse(
			"invalid_input",
			"Invalid input",
			"Pet name is required",
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusBadRequest, problem)
		return
	}

	petResponse := createPet(petName)
	if !petResponse.Success {
		problem := responses.NewProblemResponse(
			"unable_to_create_pet",
			"Unable to create pet",
			petResponse.Message,
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusInternalServerError, problem)
		return
	}

	responses.SendAndEncodeStruct(w, r, http.StatusCreated, petResponse.Data)
}

// GetPetHandler - Get a pet by ID
func GetPetHandler(w http.ResponseWriter, r *http.Request) {
	var petID int
	stringPetID := r.PathValue("id")
	if stringPetID != "" {
		var err error
		petID, err = strconv.Atoi(stringPetID)
		if err != nil {
			petID = 0
		}
	}
	if petID == 0 {
		var pet Pet
		err := responses.DecodeStruct(r, &pet)
		if err == nil {
			petID = pet.ID
		}
	}

	if petID == 0 {
		problem := responses.NewProblemResponse(
			"invalid_input",
			"Invalid input",
			"Pet ID is required",
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusBadRequest, problem)
		return
	}

	petResponse := getPet(petID)
	if !petResponse.Success {
		problem := responses.NewProblemResponse(
			"not_found",
			"Pet not found",
			petResponse.Message,
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusNotFound, problem)
		return
	}

	responses.SendAndEncodeStruct(w, r, http.StatusOK, petResponse.Data)
}

// UpdatePetHandler - Update a pet
func UpdatePetHandler(w http.ResponseWriter, r *http.Request) {
	var pet Pet
	err := responses.DecodeStruct(r, &pet)
	if err != nil {
		problem := responses.NewProblemResponse(
			"invalid_input",
			"Invalid input",
			"Invalid input",
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusBadRequest, problem)
		return
	}

	petResponse := updatePet(pet.ID, pet.Name, pet.ProfilePicture)
	if !petResponse.Success {
		problem := responses.NewProblemResponse(
			"unable_to_update_pet",
			"Unable to update pet",
			petResponse.Message,
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusInternalServerError, problem)
		return
	}

	responses.SendAndEncodeStruct(w, r, http.StatusOK, petResponse.Data)
}

// GetRandPetPictureHandler - Get a random pet picture
func GetRandPetPictureHandler(w http.ResponseWriter, r *http.Request) {
	petPictureResponse := getRandPetPicture()
	if !petPictureResponse.Success {
		problem := responses.NewProblemResponse(
			"not_found",
			"Pet picture not found",
			petPictureResponse.Message,
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusNotFound, problem)
		return
	}

	responses.SendAndEncodeStruct(w, r, http.StatusOK, petPictureResponse.Data)
}

// GetPetPictureHandler - Get a pet picture by ID
func GetPetPictureHandler(w http.ResponseWriter, r *http.Request) {
	petPictureID := r.PathValue("id")
	if petPictureID == "" {
		var petPicture PetPicture
		err := responses.DecodeStruct(r, &petPicture)
		if err == nil {
			petPictureID = string(petPicture.ID)
		}
	}
	if petPictureID == "" {
		problem := responses.NewProblemResponse(
			"invalid_input",
			"Invalid input",
			"Pet picture ID is required",
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusBadRequest, problem)
		return
	}

	petPictureResponse := getPetPicture(petPictureID)
	if !petPictureResponse.Success {
		problem := responses.NewProblemResponse(
			"not_found",
			"Pet picture not found",
			petPictureResponse.Message,
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusNotFound, problem)
		return
	}

	responses.SendAndEncodeStruct(w, r, http.StatusOK, petPictureResponse.Data)
}

// UpdatePetPictureHandler - Update a pet picture
func UpdatePetPictureHandler(w http.ResponseWriter, r *http.Request) {
	var petPicture PetPicture
	err := responses.DecodeStruct(r, &petPicture)
	if err != nil {
		problem := responses.NewProblemResponse(
			"invalid_input",
			"Invalid input",
			"Invalid input",
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusBadRequest, problem)
		return
	}

	petPictureResponse := updatePetPicture(petPicture.ID, petPicture.FileExt, petPicture.Subjects, petPicture.Aliases)
	if !petPictureResponse.Success {
		problem := responses.NewProblemResponse(
			"unable_to_update_pet_picture",
			"Unable to update pet picture",
			petPictureResponse.Message,
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusInternalServerError, problem)
		return
	}

	responses.SendAndEncodeStruct(w, r, http.StatusOK, petPictureResponse.Data)
}

// DeletePetPictureHandler - Delete a pet picture
func DeletePetPictureHandler(w http.ResponseWriter, r *http.Request) {
	petPictureID := r.PathValue("id")
	if petPictureID == "" {
		var petPicture PetPicture
		err := responses.DecodeStruct(r, &petPicture)
		if err == nil {
			petPictureID = string(petPicture.ID)
		}
	}
	if petPictureID == "" {
		problem := responses.NewProblemResponse(
			"invalid_input",
			"Invalid input",
			"Pet picture ID is required",
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusBadRequest, problem)
		return
	}

	petPictureResponse := deletePetPicture(petPictureID)

	if !petPictureResponse.Success {
		problem := responses.NewProblemResponse(
			"unable_to_delete_pet_picture",
			"Unable to delete pet picture",
			petPictureResponse.Message,
			// TODO: Add instance
			"TODO: Add instance",
		)
		responses.SendAndEncodeProblem(w, r, http.StatusInternalServerError, problem)
		return
	}

	responses.SendAndEncodeStruct(w, r, http.StatusOK, petPictureResponse.Data)
}
