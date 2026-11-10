package petpictures

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
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
	msgUnableToCreatePet           = "Unable to create pet"
	msgPetNameAlreadyExists        = "A pet with that name already exists"
	msgInvalidPetName              = "Invalid pet name"
	msgPetIDIsRequired             = "Pet ID is required"
	msgUnableToUpdatePet           = "Unable to update pet"
	msgUnableToGetRandomPetPicture = "Unable to get random pet picture"
	msgPetPictureNotFound          = "Pet picture not found"
	msgUnableToUpdatePetPicture    = "Unable to update pet picture"
	msgUnableToDeletePetPicture    = "Unable to delete pet picture"
	logUnableToGetPetPicture       = "[Error]: Unable to get pet picture:\n\t"
	logUnableToGetPet              = "[Error]: Unable to get pet:\n\t"
)

func validPetName(name string) bool {
	return !strings.Contains(name, "\x00") && utf8.ValidString(name)
}

// Handler serves the pet pictures routes.
type Handler interface {
	CreatePet(w http.ResponseWriter, r *http.Request)
	GetPet(w http.ResponseWriter, r *http.Request)
	UpdatePet(w http.ResponseWriter, r *http.Request)
	GetRandPetPictureByName(w http.ResponseWriter, r *http.Request)
	GetPetPicture(w http.ResponseWriter, r *http.Request)
	UpdatePetPicture(w http.ResponseWriter, r *http.Request)
	DeletePetPicture(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	s PetPicService
}

// NewHandler returns a Handler that serves the pet pictures routes from the given service.
func NewHandler(s PetPicService) Handler {
	return &handler{s}
}

// CreatePet - Create a new pet
func (h *handler) CreatePet(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
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
	if !validPetName(petName) {
		responses.BadRequest(w, r, msgInvalidPetName)
		return
	}

	petResponse, err := h.s.GetStore().CreatePet(r.Context(), petName)
	if err != nil {
		reqctx.Log(r.Context(), "[Error]: Unable to create pet:\n\t", err.Error())
		switch err {
		case ErrPetNameEmpty:
			responses.BadRequest(w, r, msgPetNameMustNotBeEmpty)
		case ErrPetNameExists:
			responses.Conflict(w, r, msgPetNameAlreadyExists)
		default:
			responses.InternalServerError(w, r, msgUnableToCreatePet)
		}
		return
	}
	responses.SendStruct(w, r, http.StatusCreated, petResponse)
}

// GetPet - Get a pet by ID
func (h *handler) GetPet(w http.ResponseWriter, r *http.Request) {
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

	pet, err := h.s.GetStore().GetPet(r.Context(), petID)
	if err != nil {
		reqctx.Log(r.Context(), logUnableToGetPet, err.Error())
		if err == ErrPetNotFound {
			responses.NotFound(w, r, msgPetNotFound)
			return
		}
		responses.InternalServerError(w, r, msgUnableToGetPet)
		return
	}
	responses.StructOK(w, r, pet)
}

// UpdatePet - Update a pet
func (h *handler) UpdatePet(w http.ResponseWriter, r *http.Request) {
	var pet *Pet
	err := responses.DecodeStruct(r, &pet)
	if err != nil {
		responses.BadRequest(w, r, msgUnableToParseBody)
		return
	}

	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermissionValue(perms.ScopePetPictures, pet.Name) {
		responses.Forbidden(w, r, msgNoPermissionToUpdatePet)
		return
	}
	if !validPetName(pet.Name) {
		responses.BadRequest(w, r, msgInvalidPetName)
		return
	}

	_, err = h.s.GetStore().UpdatePet(r.Context(), pet)
	if err != nil {
		reqctx.Log(r.Context(), "[Error]: Unable to update pet:\n\t", err.Error())
		switch err {
		case ErrPetNotFound:
			responses.NotFound(w, r, msgPetNotFound)
		case ErrPetNameEmpty:
			responses.BadRequest(w, r, msgPetNameMustNotBeEmpty)
		case ErrPetNameExists:
			responses.Conflict(w, r, msgPetNameAlreadyExists)
		default:
			responses.InternalServerError(w, r, msgUnableToUpdatePet)
		}
		return
	}
	responses.StructOK(w, r, pet)
}

// GetRandPetPictureByName - Get a random pet picture
func (h *handler) GetRandPetPictureByName(w http.ResponseWriter, r *http.Request) {
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

	petPicture, err := h.s.GetStore().GetRandPetPictureByName(r.Context(), petName)
	if err != nil {
		reqctx.Log(r.Context(), "[Error]: Unable to get random pet picture:\n\t", err.Error())
		switch err {
		case ErrPetNotFound:
			responses.NotFound(w, r, msgPetNotFound)
		case ErrPetPictureNotFound:
			responses.NotFound(w, r, msgPetPictureNotFound)
		default:
			responses.InternalServerError(w, r, msgUnableToGetRandomPetPicture)
		}
		return
	}
	responses.StructOK(w, r, petPicture)
}

// GetPetPicture - Get a pet picture by ID
func (h *handler) GetPetPicture(w http.ResponseWriter, r *http.Request) {
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

	petPicture, err := h.s.GetStore().GetPetPicture(r.Context(), petPictureID)
	if err != nil {
		reqctx.Log(r.Context(), logUnableToGetPetPicture, err.Error())
		if err == ErrPetPictureNotFound {
			responses.NotFound(w, r, msgPetPictureNotFound)
			return
		}
		responses.InternalServerError(w, r, msgUnableToGetPetPicture)
		return
	}
	responses.StructOK(w, r, petPicture)
}

// UpdatePetPicture - Update a pet picture
func (h *handler) UpdatePetPicture(w http.ResponseWriter, r *http.Request) {
	var petPicture PetPicture
	err := responses.DecodeStruct(r, &petPicture)
	if err != nil {
		responses.BadRequest(w, r, msgUnableToParseBody)
		return
	}

	pet, err := h.s.GetStore().GetPet(r.Context(), petPicture.PrimarySubject)
	if err != nil {
		reqctx.Log(r.Context(), logUnableToGetPet, err.Error())
		if err == ErrPetNotFound {
			responses.NotFound(w, r, msgPetNotFound)
			return
		}
		responses.InternalServerError(w, r, msgUnableToGetPet)
		return
	}

	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermissionValue(perms.ScopePetPictures, pet.Name) {
		responses.Forbidden(w, r, msgNoPermissionToUpdatePet)
		return
	}

	_, err = h.s.GetStore().UpdatePetPicture(r.Context(), petPicture)
	if err != nil {
		reqctx.Log(r.Context(), "[Error]: Unable to update pet picture:\n\t", err.Error())
		if err == ErrPetPictureNotFound {
			responses.NotFound(w, r, msgPetPictureNotFound)
			return
		}
		responses.InternalServerError(w, r, msgUnableToUpdatePetPicture)
		return
	}
	responses.StructOK(w, r, petPicture)
}

// DeletePetPicture - Delete a pet picture
func (h *handler) DeletePetPicture(w http.ResponseWriter, r *http.Request) {
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

	petPicture, err := h.s.GetStore().GetPetPicture(r.Context(), petPictureID)
	if err != nil {
		reqctx.Log(r.Context(), logUnableToGetPetPicture, err.Error())
		if err == ErrPetPictureNotFound {
			responses.NotFound(w, r, msgPetPictureNotFound)
			return
		}
		responses.InternalServerError(w, r, msgUnableToGetPetPicture)
		return
	}

	pet, err := h.s.GetStore().GetPet(r.Context(), petPicture.PrimarySubject)
	if err != nil {
		reqctx.Log(r.Context(), logUnableToGetPet, err.Error())
		if err == ErrPetNotFound {
			responses.NotFound(w, r, msgPetNotFound)
			return
		}
		responses.InternalServerError(w, r, msgUnableToGetPet)
		return
	}

	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermissionValue(perms.ScopePetPictures, pet.Name) {
		responses.Forbidden(w, r, msgNoPermissionToUpdatePet)
		return
	}

	_, err = h.s.GetStore().DeletePetPicture(r.Context(), petPictureID)
	if err != nil {
		reqctx.Log(r.Context(), "[Error]: Unable to delete pet picture:\n\t", err.Error())
		responses.InternalServerError(w, r, msgUnableToDeletePetPicture)
		return
	}
	responses.NoContent(w, r)
}
