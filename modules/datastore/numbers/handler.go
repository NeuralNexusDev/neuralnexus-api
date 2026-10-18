package numbersds

import (
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
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
		session := r.Context().Value(reqctx.SessionKey).(*session.Session)
		if !session.HasPermission(perms.ScopeAdminNumberStore) {
			responses.Forbidden(w, r, msgNoPermissionToCreateNumberstore)
			return
		}

		var n *NumberData
		err := responses.DecodeStruct(r, &n)
		if err != nil {
			reqctx.Log(r.Context(), logBadBody, err.Error())
			responses.BadRequest(w, r, "")
			return
		}

		n, err = s.Create(r.Context(), n)
		if err != nil {
			reqctx.Log(r.Context(), "Failed to create numberstore:\n\t", err.Error())
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
			reqctx.Log(r.Context(), logBadBody, err.Error())
			responses.BadRequest(w, r, "")
			return
		}

		n, err = s.Read(r.Context(), n)
		if err != nil {
			reqctx.Log(r.Context(), "Failed to read numberstore:\n\t", err.Error())
			responses.InternalServerError(w, r, msgFailedToReadNumberstore)
			return
		}
		responses.StructOK(w, r, n)
	}
}

// UpdateNumberHandler - Update a number
func UpdateNumberHandler(s NumberService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(reqctx.SessionKey).(*session.Session)
		if !session.HasPermission(perms.ScopeAdminNumberStore) {
			responses.Forbidden(w, r, msgNoPermissionToUpdateNumberstore)
			return
		}

		var n *NumberData
		err := responses.DecodeStruct(r, &n)
		if err != nil {
			reqctx.Log(r.Context(), logBadBody, err.Error())
			responses.BadRequest(w, r, "")
			return
		}

		n, err = s.Update(r.Context(), n)
		if err != nil {
			reqctx.Log(r.Context(), "Failed to update numberstore:\n\t", err.Error())
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
			reqctx.Log(r.Context(), logBadBody, err.Error())
			responses.BadRequest(w, r, "")
			return
		}

		err = s.Delete(r.Context(), n)
		if err != nil {
			reqctx.Log(r.Context(), "Failed to delete numberstore:\n\t", err.Error())
			responses.InternalServerError(w, r, msgFailedToDeleteNumberstore)
			return
		}
		responses.StructOK(w, r, n)
	}
}
