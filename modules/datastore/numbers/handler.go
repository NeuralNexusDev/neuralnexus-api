package numbersds

import (
	"log"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgFailedToCreateNumberstore       = "Failed to create numberstore"
	msgFailedToReadNumberstore         = "Failed to read numberstore"
	msgFailedToUpdateNumberstore       = "Failed to update numberstore"
	msgFailedToDeleteNumberstore       = "Failed to delete numberstore"
	logBadBody                         = "Bad body:\n\t"
	msgNoPermissionToCreateNumberstore = "You do not have permission to create a numberstore"
	msgNoPermissionToUpdateNumberstore = "You do not have permission to update a numberstore"
)

// CreateNumberHandler - Create a new number
func CreateNumberHandler(s NumberService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminNumberStore) {
			responses.Forbidden(w, r, msgNoPermissionToCreateNumberstore)
			return
		}

		var n *NumberData
		err := responses.DecodeStruct(r, &n)
		if err != nil {
			log.Println(logBadBody, err)
			responses.BadRequest(w, r, "")
			return
		}

		n, err = s.Create(n)
		if err != nil {
			log.Println("Failed to create numberstore:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToCreateNumberstore)
			return
		}
		responses.StructOK(w, r, n)
	}
}

// ReadNumberHandler - Read a number
func ReadNumberHandler(s NumberService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var n *NumberData
		err := responses.DecodeStruct(r, &n)
		if err != nil {
			log.Println(logBadBody, err)
			responses.BadRequest(w, r, "")
			return
		}

		n, err = s.Read(n)
		if err != nil {
			log.Println("Failed to read numberstore:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToReadNumberstore)
			return
		}
		responses.StructOK(w, r, n)
	}
}

// UpdateNumberHandler - Update a number
func UpdateNumberHandler(s NumberService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminNumberStore) {
			responses.Forbidden(w, r, msgNoPermissionToUpdateNumberstore)
			return
		}

		var n *NumberData
		err := responses.DecodeStruct(r, &n)
		if err != nil {
			log.Println(logBadBody, err)
			responses.BadRequest(w, r, "")
			return
		}

		n, err = s.Update(n)
		if err != nil {
			log.Println("Failed to update numberstore:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToUpdateNumberstore)
			return
		}
		responses.StructOK(w, r, n)
	}
}

// DeleteNumberHandler - Delete a number
func DeleteNumberHandler(s NumberService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var n *NumberData
		err := responses.DecodeStruct(r, &n)
		if err != nil {
			log.Println(logBadBody, err)
			responses.BadRequest(w, r, "")
			return
		}

		err = s.Delete(n)
		if err != nil {
			log.Println("Failed to delete numberstore:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToDeleteNumberstore)
			return
		}
		responses.StructOK(w, r, n)
	}
}
