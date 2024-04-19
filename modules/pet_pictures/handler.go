package petpictures

import (
	"log"
	"net/http"
	"strconv"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply routes to the router
func ApplyRoutes(router *http.ServeMux) *http.ServeMux {
	store := NewStore(database.GetDB("pet_pictures"))
	service := NewService(store)
	router.HandleFunc("POST /api/v1/pet-pictures/pets/{name}", mw.Auth(CreatePetHandler(service)))
	router.HandleFunc("POST /api/v1/pet-pictures/pets", mw.Auth(CreatePetHandler(service)))
	router.HandleFunc("GET /api/v1/pet-pictures/pets/{id}", GetPetHandler(service))
	router.HandleFunc("GET /api/v1/pet-pictures/pets", GetPetHandler(service))
	router.HandleFunc("PUT /api/v1/pet-pictures/pets", mw.Auth(UpdatePetHandler(service)))
	router.HandleFunc("GET /api/v1/pet-pictures/pictures/random", GetRandPetPictureByNameHandler(service))
	router.HandleFunc("GET /api/v1/pet-pictures/pictures/{id}", GetPetPictureHandler(service))
	router.HandleFunc("GET /api/v1/pet-pictures/pictures", GetPetPictureHandler(service))
	router.HandleFunc("PUT /api/v1/pet-pictures/pictures", mw.Auth(UpdatePetPictureHandler(service)))
	router.HandleFunc("DELETE /api/v1/pet-pictures/pictures/{id}", mw.Auth(DeletePetPictureHandler(service)))
	router.HandleFunc("DELETE /api/v1/pet-pictures/pictures", mw.Auth(DeletePetPictureHandler(service)))
	return router
}

// CreatePetHandler - Create a new pet
func CreatePetHandler(s *service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		petResponse, err := s.db.CreatePet(petName)
		if err != nil {
			log.Println("[Error]: Unable to create pet:\n\t", err)
			responses.SendAndEncodeInternalServerError(w, r, "Unable to create pet (pet may already exist)")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusCreated, petResponse)
	}
}

// GetPetHandler - Get a pet by ID
func GetPetHandler(s *service) http.HandlerFunc {
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
			responses.SendAndEncodeBadRequest(w, r, "Pet ID is required")
			return
		}

		pet, err := s.db.GetPet(petID)
		if err != nil {
			log.Println("[Error]: Unable to get pet:\n\t", err)
			responses.SendAndEncodeNotFound(w, r, "Pet not found")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, pet)
	}
}

// UpdatePetHandler - Update a pet
func UpdatePetHandler(s *service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		_, err = s.db.UpdatePet(pet)
		if err != nil {
			log.Println("[Error]: Unable to update pet:\n\t", err)
			responses.SendAndEncodeInternalServerError(w, r, "Unable to update pet")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, pet)
	}
}

// GetRandPetPictureByNameHandler - Get a random pet picture
func GetRandPetPictureByNameHandler(s *service) http.HandlerFunc {
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
			responses.SendAndEncodeBadRequest(w, r, "Pet name is required")
			return
		}

		petPicture, err := s.db.GetRandPetPictureByName(petName)
		if err != nil {
			log.Println("[Error]: Unable to get random pet picture:\n\t", err)
			responses.SendAndEncodeNotFound(w, r, "Unable to get random pet picture")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, petPicture)
	}
}

// GetPetPictureHandler - Get a pet picture by ID
func GetPetPictureHandler(s *service) http.HandlerFunc {
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
			responses.SendAndEncodeBadRequest(w, r, "Pet picture ID is required")
			return
		}

		petPicture, err := s.db.GetPetPicture(petPictureID)
		if err != nil {
			log.Println("[Error]: Unable to get pet picture:\n\t", err)
			responses.SendAndEncodeNotFound(w, r, "Unable to get pet picture")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, petPicture)
	}
}

// UpdatePetPictureHandler - Update a pet picture
func UpdatePetPictureHandler(s *service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var petPicture PetPicture
		err := responses.DecodeStruct(r, &petPicture)
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Invalid input, unable to parse body")
			return
		}

		pet, err := s.db.GetPet(petPicture.PrimarySubject)
		if err != nil {
			log.Println("[Error]: Unable to get pet:\n\t", err)
			responses.SendAndEncodeNotFound(w, r, "Unable to get pet")
			return
		}

		session := r.Context().Value(mw.SessionKey).(auth.Session)
		if !session.HasPermission(auth.ScopePetPictures(pet.Name)) {
			responses.SendAndEncodeForbidden(w, r, "You do not have permission to update this pet")
			return
		}

		_, err = s.db.UpdatePetPicture(petPicture)
		if err != nil {
			log.Println("[Error]: Unable to update pet picture:\n\t", err)
			responses.SendAndEncodeInternalServerError(w, r, "Unable to update pet picture")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, petPicture)
	}
}

// DeletePetPictureHandler - Delete a pet picture
func DeletePetPictureHandler(s *service) http.HandlerFunc {
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
			responses.SendAndEncodeBadRequest(w, r, "Pet picture ID is required")
			return
		}

		petPicture, err := s.db.GetPetPicture(petPictureID)
		if err != nil {
			log.Println("[Error]: Unable to get pet picture:\n\t", err)
			responses.SendAndEncodeNotFound(w, r, "Unable to get pet picture")
			return
		}

		pet, err := s.db.GetPet(petPicture.PrimarySubject)
		if err != nil {
			log.Println("[Error]: Unable to get pet:\n\t", err)
			responses.SendAndEncodeNotFound(w, r, "Unable to get pet")
			return
		}

		session := r.Context().Value(mw.SessionKey).(auth.Session)
		if !session.HasPermission(auth.ScopePetPictures(pet.Name)) {
			responses.SendAndEncodeForbidden(w, r, "You do not have permission to update this pet")
			return
		}

		_, err = s.db.DeletePetPicture(petPictureID)
		if err != nil {
			log.Println("[Error]: Unable to delete pet picture:\n\t", err)
			responses.SendAndEncodeInternalServerError(w, r, "Unable to delete pet picture")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, petPicture)
	}
}
