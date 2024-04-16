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

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/jackc/pgx/v5"
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

// -------------- Globals --------------
var (
	CDN_URL  = "https://cdn.neuralnexus.dev"
	CDN_PATH = "/petpictures/"
	CDN_KEY  = os.Getenv("CDN_KEY")
)

// -------------- Structs --------------
// PetPicture - Pet picture struct
type PetPicture struct {
	ID             string   `json:"id" xml:"id" db:"id"`
	FileExt        string   `json:"file_ext" xml:"file_ext" db:"file_ext"`
	PrimarySubject int      `json:"prime_subj" xml:"prime_subj" db:"prime_subj"`
	OthersSubjects []int    `json:"othr_subj" xml:"othr_subj" db:"othr_subj"`
	Aliases        []string `json:"aliases" xml:"aliases" db:"aliases"`
	Created        string   `json:"created" xml:"created" db:"created"`
}

// GetPetPictureURL - Get the URL for a pet picture
func (p *PetPicture) GetPetPictureURL() string {
	return CDN_URL + CDN_PATH + string(p.ID) + "." + p.FileExt
}

// Pet - Pet struct
type Pet struct {
	ID             int    `json:"id" xml:"id" db:"id"`
	Name           string `json:"name" xml:"name" db:"name"`
	ProfilePicture string `json:"profile_picture" xml:"profile_picture" db:"profile_picture"`
}

// APIResponse - API response struct
type APIResponse[T Pet | PetPicture] struct {
	Success bool
	Message string
	Data    T
}

// APISuccessResponse - Create a new API success response
func APISuccessResponse[T Pet | PetPicture](data T) APIResponse[T] {
	return APIResponse[T]{
		Success: true,
		Data:    data,
	}
}

// APIErrorResponse - Create a new API error response
func APIErrorResponse[T Pet | PetPicture](message string) APIResponse[T] {
	log.Println(message + ":")
	return APIResponse[T]{
		Success: false,
		Message: message,
	}
}

// -------------- DB Functions --------------

// createPet - Create a new pet
func createPet(name string) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(),
		"INSERT INTO pets (name) VALUES ($1) RETURNING id, name, profile_picture", name,
	).Scan(&pet.ID, &pet.Name, &pet.ProfilePicture)
	if err != nil {
		return database.ErrorResponse[Pet]("Unable to create pet (pet may already exist)", err)
	}
	return database.SuccessResponse(pet)
}

// getPet - Get a pet by ID
func getPet(id int) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(), "SELECT * FROM pets WHERE id = $1", id).Scan(&pet.ID, &pet.Name, &pet.ProfilePicture)
	if err != nil {
		return database.ErrorResponse[Pet]("Unable to get pet", err)
	}
	return database.SuccessResponse(pet)
}

// getPetByName - Get a pet by name
func getPetByName(name string) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var pet Pet
	err := db.QueryRow(context.Background(), "SELECT id, name, profile_picture FROM pets WHERE name = $1", name).Scan(&pet.ID, &pet.Name, &pet.ProfilePicture)
	if err != nil {
		return database.ErrorResponse[Pet]("Unable to get pet", err)
	}
	return database.SuccessResponse(pet)
}

// updatePet - Update a pet
func updatePet(pet Pet) database.Response[Pet] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	_, err := db.Query(context.Background(), "UPDATE pets SET name = $1, profile_picture = $2 WHERE id = $3", pet.Name, pet.ProfilePicture, pet.ID)
	if err != nil {
		return database.ErrorResponse[Pet]("Unable to update pet", err)
	}
	return database.SuccessResponse(pet)
}

// createPetPicture - Create a new pet picture
func createPetPicture(id string, fileExt string, primarySubject int, othersSubjects []int, aliases []string) database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	_, err := db.Query(context.Background(),
		"INSERT INTO pictures (id, file_ext, prime_subj, othr_subj, aliases) VALUES ($1, $2, $3, $4, $5)",
		id, fileExt, primarySubject, othersSubjects, aliases,
	)
	if err != nil {
		return database.ErrorResponse[PetPicture]("Unable to create pet picture", err)
	}
	return database.SuccessResponse(PetPicture{
		ID:             id,
		FileExt:        fileExt,
		PrimarySubject: primarySubject,
		OthersSubjects: othersSubjects,
		Aliases:        aliases,
	})
}

// getRandPetPictureByName - Get a random pet picture by name
func getRandPetPictureByName(name string) database.Response[PetPicture] {
	pet := getPetByName(name)
	if !pet.Success {
		return database.Response[PetPicture]{
			Success: false,
			Message: pet.Message,
		}
	}

	db := database.GetDB("pet_pictures")
	defer db.Close()

	rows, err := db.Query(context.Background(),
		"SELECT * FROM pictures WHERE prime_subj = $1 OR $2 = ANY(othr_subj) ORDER BY random() LIMIT 1", pet.Data.ID, pet.Data.ID)
	if err != nil {
		return database.ErrorResponse[PetPicture]("Unable to get random pet picture by name", err)
	}

	var picture *PetPicture
	picture, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[PetPicture])
	if err != nil {
		return database.ErrorResponse[PetPicture]("Unable to get random pet picture by name", err)
	}
	return database.SuccessResponse(*picture)
}

// getPetPicture - Get a pet picture by ID
func getPetPicture(id string) database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM pictures WHERE id = $1", id)
	if err != nil {
		return database.ErrorResponse[PetPicture]("Unable to get pet picture", err)
	}

	var picture *PetPicture
	picture, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[PetPicture])
	if err != nil {
		return database.ErrorResponse[PetPicture]("Unable to get pet picture", err)
	}
	return database.SuccessResponse(*picture)
}

// updatePetPicture - Update a pet picture
func updatePetPicture(picture PetPicture) database.Response[PetPicture] {
	db := database.GetDB("pet_pictures")
	defer db.Close()

	var petPicture PetPicture
	_, err := db.Query(context.Background(),
		"UPDATE pictures SET file_ext = $1, prime_subj = $2, othr_subj = $3, aliases = $4 WHERE id = $5",
		picture.FileExt, picture.PrimarySubject, picture.OthersSubjects, picture.Aliases, picture.ID,
	)
	if err != nil {
		return database.ErrorResponse[PetPicture]("Unable to update pet picture", err)
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

	_, err := db.Query(context.Background(), "DELETE FROM pictures WHERE id = $1", id)
	if err != nil {
		return database.ErrorResponse[PetPicture]("Unable to delete pet picture", err)
	}
	return database.SuccessResponse(PetPicture{ID: id})
}

// -------------- Functions --------------

// UploadPetPicture - Upload a pet picture
func UploadPetPicture(file *os.File, primarySubject int, othersSubjects []int, aliases []string) APIResponse[PetPicture] {
	// Get SHA1 hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return APIErrorResponse[PetPicture]("Unable to get SHA256 hash")
	}
	sha := hex.EncodeToString(hash.Sum(nil))

	splitName := strings.Split(file.Name(), ".")
	fileExt := splitName[len(splitName)-1]

	petPictureResponse := createPetPicture(sha, fileExt, primarySubject, othersSubjects, aliases)
	if !petPictureResponse.Success {
		return APIErrorResponse[PetPicture](petPictureResponse.Message)
	}
	petPicture := petPictureResponse.Data

	// Update file name
	newFileName := string(petPicture.ID) + "." + fileExt
	if err := os.Rename(file.Name(), newFileName); err != nil {
		return APIErrorResponse[PetPicture]("Unable to rename file")
	}

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, CDN_URL+"/upload", nil)
	if err != nil {
		return APIErrorResponse[PetPicture]("Unable to create request")
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
		return APIErrorResponse[PetPicture]("Unable to create form file")
	}

	// Copy the file to the form file
	if _, err := io.Copy(part, file); err != nil {
		return APIErrorResponse[PetPicture]("Unable to copy file")
	}
	writer.Close()

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Body = io.NopCloser(body)

	// Send the request
	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return APIErrorResponse[PetPicture]("Unable to send request")
	}
	return APISuccessResponse(petPicture)
}

// -------------- Routes --------------

// ApplyRoutes - Apply routes to the router
func ApplyRoutes(router *http.ServeMux) *http.ServeMux {
	router.HandleFunc("POST /api/v1/pet-pictures/pets/{name}", mw.Auth(CreatePetHandler))
	router.HandleFunc("POST /api/v1/pet-pictures/pets", mw.Auth(CreatePetHandler))
	router.HandleFunc("GET /api/v1/pet-pictures/pets/{id}", GetPetHandler)
	router.HandleFunc("GET /api/v1/pet-pictures/pets", GetPetHandler)
	router.HandleFunc("PUT /api/v1/pet-pictures/pets", mw.Auth(UpdatePetHandler))
	router.HandleFunc("GET /api/v1/pet-pictures/pictures/random", GetRandPetPictureByNameHandler)
	router.HandleFunc("GET /api/v1/pet-pictures/pictures/{id}", GetPetPictureHandler)
	router.HandleFunc("GET /api/v1/pet-pictures/pictures", GetPetPictureHandler)
	router.HandleFunc("PUT /api/v1/pet-pictures/pictures", mw.Auth(UpdatePetPictureHandler))
	router.HandleFunc("DELETE /api/v1/pet-pictures/pictures/{id}", mw.Auth(DeletePetPictureHandler))
	router.HandleFunc("DELETE /api/v1/pet-pictures/pictures", mw.Auth(DeletePetPictureHandler))
	return router
}

// CreatePetHandler - Create a new pet
func CreatePetHandler(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(mw.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopeAdminPetPictures) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to create a pet")
		return
	}

	petName := r.PathValue("name")
	if petName == "" {
		var pet Pet
		err := responses.DecodeStruct(r, &pet)
		if err == nil {
			petName = pet.Name
		}
	}
	if petName == "" {
		responses.SendAndEncodeBadRequest(w, r, "Pet name is required")
		return
	}

	petResponse := createPet(petName)
	if !petResponse.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Unable to create pet")
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
		responses.SendAndEncodeBadRequest(w, r, "Pet ID is required")
		return
	}

	petResponse := getPet(petID)
	if !petResponse.Success {
		responses.SendAndEncodeNotFound(w, r, petResponse.Message)
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, petResponse.Data)
}

// UpdatePetHandler - Update a pet
func UpdatePetHandler(w http.ResponseWriter, r *http.Request) {
	var pet Pet
	err := responses.DecodeStruct(r, &pet)
	if err != nil {
		responses.SendAndEncodeBadRequest(w, r, "Invalid input, unable to parse body")
		return
	}

	session := r.Context().Value(mw.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopePetPictures(pet.Name)) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to update this pet")
		return
	}

	petResponse := updatePet(pet)
	if !petResponse.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Unable to update pet")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, petResponse.Data)
}

// GetRandPetPictureByNameHandler - Get a random pet picture
func GetRandPetPictureByNameHandler(w http.ResponseWriter, r *http.Request) {
	petName := r.PathValue("name")
	if petName == "" {
		var pet Pet
		err := responses.DecodeStruct(r, &pet)
		if err == nil {
			petName = pet.Name
		}
	}
	if petName == "" {
		responses.SendAndEncodeBadRequest(w, r, "Pet name is required")
		return
	}

	petPictureResponse := getRandPetPictureByName(petName)
	if !petPictureResponse.Success {
		responses.SendAndEncodeNotFound(w, r, petPictureResponse.Message)
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
		responses.SendAndEncodeBadRequest(w, r, "Pet picture ID is required")
		return
	}

	petPictureResponse := getPetPicture(petPictureID)
	if !petPictureResponse.Success {
		responses.SendAndEncodeNotFound(w, r, petPictureResponse.Message)
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, petPictureResponse.Data)
}

// UpdatePetPictureHandler - Update a pet picture
func UpdatePetPictureHandler(w http.ResponseWriter, r *http.Request) {
	var petPicture PetPicture
	err := responses.DecodeStruct(r, &petPicture)
	if err != nil {
		responses.SendAndEncodeBadRequest(w, r, "Invalid input, unable to parse body")
		return
	}

	pet := getPet(petPicture.PrimarySubject)
	if !pet.Success {
		responses.SendAndEncodeNotFound(w, r, pet.Message)
		return
	}

	session := r.Context().Value(mw.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopePetPictures(pet.Data.Name)) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to update this pet")
		return
	}

	petPictureResponse := updatePetPicture(petPicture)
	if !petPictureResponse.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Unable to update pet picture")
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
		responses.SendAndEncodeBadRequest(w, r, "Pet picture ID is required")
		return
	}

	petPicture := getPetPicture(petPictureID)
	if !petPicture.Success {
		responses.SendAndEncodeNotFound(w, r, petPicture.Message)
		return
	}

	pet := getPet(petPicture.Data.PrimarySubject)
	if !pet.Success {
		responses.SendAndEncodeNotFound(w, r, pet.Message)
		return
	}

	session := r.Context().Value(mw.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopePetPictures(pet.Data.Name)) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to update this pet")
		return
	}

	petPictureResponse := deletePetPicture(petPictureID)
	if !petPictureResponse.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Unable to delete pet picture")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, petPictureResponse.Data)
}
