package datastore

import (
	"log"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgNoPermissionToCreateDatastore = "You do not have permission to create a datastore"
	msgFailedToReadDatastore         = "Failed to read datastore"
	msgNoPermissionToUpdateDatastore = "You do not have permission to update a datastore"
	msgFailedToUpdateDatastore       = "Failed to update datastore"
	msgNoPermissionToDeleteDatastore = "You do not have permission to delete a datastore"
	msgFailedToDeleteDatastore       = "Failed to delete datastore"
	logBadBody                       = "Bad body:\n\t"
	msgFailedToCreateDatastore       = "Failed to create datastore"
)

// CreateDataStoreHandler - Create a new data store
func CreateDataStoreHandler(s DSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminDataStore) {
			responses.Forbidden(w, r, msgNoPermissionToCreateDatastore)
			return
		}

		id, err := database.GenSnowflake()
		if err != nil {
			log.Println("Failed to generate snowflake:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToCreateDatastore)
			return
		}
		ds := NewDataStore(id, session.UserID)
		ds, err = s.Create(ds)
		if err != nil {
			log.Println("Failed to create data store:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToCreateDatastore)
			return
		}
		responses.StructOK(w, r, ds)
	}
}

// ReadDataStoreHandler - Read a data store
func ReadDataStoreHandler(s DSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ds *Store
		err := responses.DecodeStruct(r, &ds)
		if err != nil {
			log.Println(logBadBody, err)
			responses.BadRequest(w, r, "")
			return
		}
		ds, err = s.Read(ds)
		if err != nil {
			log.Println("Failed to read data store:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToReadDatastore)
			return
		}
		responses.StructOK(w, r, ds)
	}
}

// UpdateDataStoreHandler - Update a data store
func UpdateDataStoreHandler(s DSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminDataStore) {
			responses.Forbidden(w, r, msgNoPermissionToUpdateDatastore)
			return
		}

		var ds *Store
		err := responses.DecodeStruct(r, &ds)
		if err != nil {
			log.Println(logBadBody, err)
			responses.BadRequest(w, r, "")
			return
		}

		ds, err = s.Update(ds)
		if err != nil {
			log.Println("Failed to update data store:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToUpdateDatastore)
			return
		}
		responses.StructOK(w, r, ds)
	}
}

// DeleteDataStoreHandler - Delete a data store
func DeleteDataStoreHandler(s DSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminDataStore) {
			responses.Forbidden(w, r, msgNoPermissionToDeleteDatastore)
			return
		}

		var ds *Store
		err := responses.DecodeStruct(r, &ds)
		if err != nil {
			log.Println(logBadBody, err)
			responses.BadRequest(w, r, "")
			return
		}

		err = s.Delete(ds)
		if err != nil {
			log.Println("Failed to delete data store:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToDeleteDatastore)
			return
		}
	}
}
