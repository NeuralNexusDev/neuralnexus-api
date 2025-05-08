package petpictures

import (
	"log"
	"net/http"
	"strconv"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply routes to the router
func ApplyRoutes(router *http.ServeMux) *http.ServeMux {
	store := NewStore(database.GetDB("pet_pictures"))
	service := NewService(store)

	db := database.GetDB("neuralnexus")
	rdb := database.GetRedis()
	authStore := auth.NewStore(db, rdb)
	session := auth.NewSessionService(authStore)

	router.HandleFunc("POST /api/v1/pet-pictures/pets/{name}", mw.Auth(session, CreatePetHandler(service)))
	router.HandleFunc("POST /api/v1/pet-pictures/pets", mw.Auth(session, CreatePetHandler(service)))
	router.HandleFunc("GET /api/v1/pet-pictures/pets/{id}", GetPetHandler(service))
	router.HandleFunc("GET /api/v1/pet-pictures/pets", GetPetHandler(service))
	router.HandleFunc("PUT /api/v1/pet-pictures/pets", mw.Auth(session, UpdatePetHandler(service)))
	router.HandleFunc("GET /api/v1/pet-pictures/pictures/random", GetRandPetPictureByNameHandler(service))
	router.HandleFunc("GET /api/v1/pet-pictures/pictures/{id}", GetPetPictureHandler(service))
	router.HandleFunc("GET /api/v1/pet-pictures/pictures", GetPetPictureHandler(service))
	router.HandleFunc("PUT /api/v1/pet-pictures/pictures", mw.Auth(session, UpdatePetPictureHandler(service)))
	router.HandleFunc("DELETE /api/v1/pet-pictures/pictures/{id}", mw.Auth(session, DeletePetPictureHandler(service)))
	router.HandleFunc("DELETE /api/v1/pet-pictures/pictures", mw.Auth(session, DeletePetPictureHandler(service)))
	return router
}

// CreatePetHandler - Create a new pet
func CreatePetHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(auth.Session)
		if !session.HasPermission(perms.ScopeAdminPetPictures) {
			responses.Forbidden(w, r, "You do not have permission to create a pet")
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
			responses.BadRequest(w, r, "Pet name is required")
			return
		}

		petResponse, err := s.GetStore().CreatePet(petName)
		if err != nil {
			log.Println("[Error]: Unable to create pet:\n\t", err)
			responses.InternalServerError(w, r, "Unable to create pet (pet may already exist)")
			return
		}
		responses.SendStruct(w, r, http.StatusCreated, petResponse)
	}
}

// GetPetHandler - Get a pet by ID
func GetPetHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			responses.BadRequest(w, r, "Pet ID is required")
			return
		}

		pet, err := s.GetStore().GetPet(petID)
		if err != nil {
			log.Println("[Error]: Unable to get pet:\n\t", err)
			responses.NotFound(w, r, "Pet not found")
			return
		}
		responses.StructOK(w, r, pet)
	}
}

// UpdatePetHandler - Update a pet
func UpdatePetHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var pet *Pet
		err := responses.DecodeStruct(r, &pet)
		if err != nil {
			responses.BadRequest(w, r, "Invalid input, unable to parse body")
			return
		}

		session := r.Context().Value(mw.SessionKey).(auth.Session)
		if !session.HasPermission(perms.ScopePetPictures(pet.Name)) {
			responses.Forbidden(w, r, "You do not have permission to update this pet")
			return
		}

		_, err = s.GetStore().UpdatePet(pet)
		if err != nil {
			log.Println("[Error]: Unable to update pet:\n\t", err)
			responses.InternalServerError(w, r, "Unable to update pet")
			return
		}
		responses.StructOK(w, r, pet)
	}
}

// GetRandPetPictureByNameHandler - Get a random pet picture
func GetRandPetPictureByNameHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		petName := r.PathValue("name")
		if petName == "" {
			var pet Pet
			err := responses.DecodeStruct(r, &pet)
			if err == nil {
				petName = pet.Name
			}
		}
		if petName == "" {
			responses.BadRequest(w, r, "Pet name is required")
			return
		}

		petPicture, err := s.GetStore().GetRandPetPictureByName(petName)
		if err != nil {
			log.Println("[Error]: Unable to get random pet picture:\n\t", err)
			responses.NotFound(w, r, "Unable to get random pet picture")
			return
		}
		responses.StructOK(w, r, petPicture)
	}
}

// GetPetPictureHandler - Get a pet picture by ID
func GetPetPictureHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		petPictureID := r.PathValue("id")
		if petPictureID == "" {
			var petPicture PetPicture
			err := responses.DecodeStruct(r, &petPicture)
			if err == nil {
				petPictureID = string(petPicture.ID)
			}
		}
		if petPictureID == "" {
			responses.BadRequest(w, r, "Pet picture ID is required")
			return
		}

		petPicture, err := s.GetStore().GetPetPicture(petPictureID)
		if err != nil {
			log.Println("[Error]: Unable to get pet picture:\n\t", err)
			responses.NotFound(w, r, "Unable to get pet picture")
			return
		}
		responses.StructOK(w, r, petPicture)
	}
}

// UpdatePetPictureHandler - Update a pet picture
func UpdatePetPictureHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var petPicture PetPicture
		err := responses.DecodeStruct(r, &petPicture)
		if err != nil {
			responses.BadRequest(w, r, "Invalid input, unable to parse body")
			return
		}

		pet, err := s.GetStore().GetPet(petPicture.PrimarySubject)
		if err != nil {
			log.Println("[Error]: Unable to get pet:\n\t", err)
			responses.NotFound(w, r, "Unable to get pet")
			return
		}

		session := r.Context().Value(mw.SessionKey).(auth.Session)
		if !session.HasPermission(perms.ScopePetPictures(pet.Name)) {
			responses.Forbidden(w, r, "You do not have permission to update this pet")
			return
		}

		_, err = s.GetStore().UpdatePetPicture(petPicture)
		if err != nil {
			log.Println("[Error]: Unable to update pet picture:\n\t", err)
			responses.InternalServerError(w, r, "Unable to update pet picture")
			return
		}
		responses.StructOK(w, r, petPicture)
	}
}

// DeletePetPictureHandler - Delete a pet picture
func DeletePetPictureHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		petPictureID := r.PathValue("id")
		if petPictureID == "" {
			var petPicture PetPicture
			err := responses.DecodeStruct(r, &petPicture)
			if err == nil {
				petPictureID = string(petPicture.ID)
			}
		}
		if petPictureID == "" {
			responses.BadRequest(w, r, "Pet picture ID is required")
			return
		}

		petPicture, err := s.GetStore().GetPetPicture(petPictureID)
		if err != nil {
			log.Println("[Error]: Unable to get pet picture:\n\t", err)
			responses.NotFound(w, r, "Unable to get pet picture")
			return
		}

		pet, err := s.GetStore().GetPet(petPicture.PrimarySubject)
		if err != nil {
			log.Println("[Error]: Unable to get pet:\n\t", err)
			responses.NotFound(w, r, "Unable to get pet")
			return
		}

		session := r.Context().Value(mw.SessionKey).(auth.Session)
		if !session.HasPermission(perms.ScopePetPictures(pet.Name)) {
			responses.Forbidden(w, r, "You do not have permission to update this pet")
			return
		}

		_, err = s.GetStore().DeletePetPicture(petPictureID)
		if err != nil {
			log.Println("[Error]: Unable to delete pet picture:\n\t", err)
			responses.InternalServerError(w, r, "Unable to delete pet picture")
			return
		}
		responses.NoContent(w, r)
	}
}
