package petpictures

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgPetNameIsRequired           = "Pet name is required"
	msgPetNameMustNotBeEmpty       = "Pet name must not be empty"
	msgPetNotFound                 = "Pet not found"
	msgUnableToParseBody           = "Invalid input, unable to parse body"
	msgNoPermissionToUpdatePet     = "You do not have permission to update this pet"
	msgPetPictureIDIsRequired      = "Pet picture ID is required"
	msgUnableToGetPetPicture       = "Unable to get pet picture"
	msgUnableToGetPet              = "Unable to get pet"
	msgNoPermissionToCreatePet     = "You do not have permission to create a pet"
	msgUnableToCreatePet           = "Unable to create pet (pet may already exist)"
	msgPetIDIsRequired             = "Pet ID is required"
	msgUnableToUpdatePet           = "Unable to update pet"
	msgUnableToGetRandomPetPicture = "Unable to get random pet picture"
	msgPetPictureNotFound          = "Pet picture not found"
	msgUnableToUpdatePetPicture    = "Unable to update pet picture"
	msgUnableToDeletePetPicture    = "Unable to delete pet picture"
	logUnableToGetPetPicture       = "[Error]: Unable to get pet picture:\n\t"
	logUnableToGetPet              = "[Error]: Unable to get pet:\n\t"
)

// CreatePetHandler - Create a new pet
func CreatePetHandler(s PetPicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminPetPictures) {
			responses.Forbidden(w, r, msgNoPermissionToCreatePet)
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
			responses.BadRequest(w, r, msgPetNameIsRequired)
			return
		}

		petResponse, err := s.GetStore().CreatePet(petName)
		if err != nil {
			log.Println("[Error]: Unable to create pet:\n\t", err)
			if errors.Is(err, ErrPetNameEmpty) {
				responses.BadRequest(w, r, msgPetNameMustNotBeEmpty)
				return
			}
			responses.InternalServerError(w, r, msgUnableToCreatePet)
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
			responses.BadRequest(w, r, msgPetIDIsRequired)
			return
		}

		pet, err := s.GetStore().GetPet(petID)
		if err != nil {
			log.Println(logUnableToGetPet, err)
			if errors.Is(err, ErrPetNotFound) {
				responses.NotFound(w, r, msgPetNotFound)
				return
			}
			responses.InternalServerError(w, r, msgUnableToGetPet)
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
			responses.BadRequest(w, r, msgUnableToParseBody)
			return
		}

		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermissionValue(perms.ScopePetPictures, pet.Name) {
			responses.Forbidden(w, r, msgNoPermissionToUpdatePet)
			return
		}

		_, err = s.GetStore().UpdatePet(pet)
		if err != nil {
			log.Println("[Error]: Unable to update pet:\n\t", err)
			if errors.Is(err, ErrPetNotFound) {
				responses.NotFound(w, r, msgPetNotFound)
				return
			}
			if errors.Is(err, ErrPetNameEmpty) {
				responses.BadRequest(w, r, msgPetNameMustNotBeEmpty)
				return
			}
			responses.InternalServerError(w, r, msgUnableToUpdatePet)
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
			responses.BadRequest(w, r, msgPetNameIsRequired)
			return
		}

		petPicture, err := s.GetStore().GetRandPetPictureByName(petName)
		if err != nil {
			log.Println("[Error]: Unable to get random pet picture:\n\t", err)
			if errors.Is(err, ErrPetNotFound) {
				responses.NotFound(w, r, msgPetNotFound)
				return
			}
			if errors.Is(err, ErrPetPictureNotFound) {
				responses.NotFound(w, r, msgPetPictureNotFound)
				return
			}
			responses.InternalServerError(w, r, msgUnableToGetRandomPetPicture)
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
			responses.BadRequest(w, r, msgPetPictureIDIsRequired)
			return
		}

		petPicture, err := s.GetStore().GetPetPicture(petPictureID)
		if err != nil {
			log.Println(logUnableToGetPetPicture, err)
			if errors.Is(err, ErrPetPictureNotFound) {
				responses.NotFound(w, r, msgPetPictureNotFound)
				return
			}
			responses.InternalServerError(w, r, msgUnableToGetPetPicture)
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
			responses.BadRequest(w, r, msgUnableToParseBody)
			return
		}

		pet, err := s.GetStore().GetPet(petPicture.PrimarySubject)
		if err != nil {
			log.Println(logUnableToGetPet, err)
			if errors.Is(err, ErrPetNotFound) {
				responses.NotFound(w, r, msgPetNotFound)
				return
			}
			responses.InternalServerError(w, r, msgUnableToGetPet)
			return
		}

		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermissionValue(perms.ScopePetPictures, pet.Name) {
			responses.Forbidden(w, r, msgNoPermissionToUpdatePet)
			return
		}

		_, err = s.GetStore().UpdatePetPicture(petPicture)
		if err != nil {
			log.Println("[Error]: Unable to update pet picture:\n\t", err)
			if errors.Is(err, ErrPetPictureNotFound) {
				responses.NotFound(w, r, msgPetPictureNotFound)
				return
			}
			responses.InternalServerError(w, r, msgUnableToUpdatePetPicture)
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
			responses.BadRequest(w, r, msgPetPictureIDIsRequired)
			return
		}

		petPicture, err := s.GetStore().GetPetPicture(petPictureID)
		if err != nil {
			log.Println(logUnableToGetPetPicture, err)
			if errors.Is(err, ErrPetPictureNotFound) {
				responses.NotFound(w, r, msgPetPictureNotFound)
				return
			}
			responses.InternalServerError(w, r, msgUnableToGetPetPicture)
			return
		}

		pet, err := s.GetStore().GetPet(petPicture.PrimarySubject)
		if err != nil {
			log.Println(logUnableToGetPet, err)
			if errors.Is(err, ErrPetNotFound) {
				responses.NotFound(w, r, msgPetNotFound)
				return
			}
			responses.InternalServerError(w, r, msgUnableToGetPet)
			return
		}

		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermissionValue(perms.ScopePetPictures, pet.Name) {
			responses.Forbidden(w, r, msgNoPermissionToUpdatePet)
			return
		}

		_, err = s.GetStore().DeletePetPicture(petPictureID)
		if err != nil {
			log.Println("[Error]: Unable to delete pet picture:\n\t", err)
			responses.InternalServerError(w, r, msgUnableToDeletePetPicture)
			return
		}
		responses.NoContent(w, r)
	}
}
