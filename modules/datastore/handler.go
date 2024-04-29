package datastore

import (
	"log"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	sess "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	dsStore := NewStore(database.GetDB("neuralnexus"))
	dsService := NewService(dsStore)
	mux.Handle("POST /api/v1/datastore", mw.Auth(CreateDataStoreHandler(dsService)))
	mux.Handle("GET /api/v1/datastore", ReadDataStoreHandler(dsService))
	mux.Handle("PUT /api/v1/datastore", mw.Auth(UpdateDataStoreHandler(dsService)))
	mux.Handle("DELETE /api/v1/datastore", mw.Auth(DeleteDataStoreHandler(dsService)))
	return mux
}

// CreateDataStoreHandler - Create a new data store
func CreateDataStoreHandler(s DSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
		if !session.HasPermission(perms.ScopeAdminDataStore) {
			responses.Forbidden(w, r, "You do not have permission to create a datastore")
			return
		}

		id, err := database.GenSnowflake()
		if err != nil {
			log.Println("Failed to generate snowflake:\n\t", err)
			responses.InternalServerError(w, r, "Failed to create datastore")
			return
		}
		ds := NewDataStore(id, session.UserID)
		ds, err = s.Create(ds)
		if err != nil {
			log.Println("Failed to create data store:\n\t", err)
			responses.InternalServerError(w, r, "Failed to create datastore")
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
			log.Println("Bad body:\n\t", err)
			responses.BadRequest(w, r, "")
			return
		}
		ds, err = s.Read(ds)
		if err != nil {
			log.Println("Failed to read data store:\n\t", err)
			responses.InternalServerError(w, r, "Failed to read datastore")
			return
		}
		responses.StructOK(w, r, ds)
	}
}

// UpdateDataStoreHandler - Update a data store
func UpdateDataStoreHandler(s DSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
		if !session.HasPermission(perms.ScopeAdminDataStore) {
			responses.Forbidden(w, r, "You do not have permission to update a datastore")
			return
		}

		var ds *Store
		err := responses.DecodeStruct(r, &ds)
		if err != nil {
			log.Println("Bad body:\n\t", err)
			responses.BadRequest(w, r, "")
			return
		}

		ds, err = s.Update(ds)
		if err != nil {
			log.Println("Failed to update data store:\n\t", err)
			responses.InternalServerError(w, r, "Failed to update datastore")
			return
		}
		responses.StructOK(w, r, ds)
	}
}

// DeleteDataStoreHandler - Delete a data store
func DeleteDataStoreHandler(s DSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
		if !session.HasPermission(perms.ScopeAdminDataStore) {
			responses.Forbidden(w, r, "You do not have permission to delete a datastore")
			return
		}

		var ds *Store
		err := responses.DecodeStruct(r, &ds)
		if err != nil {
			log.Println("Bad body:\n\t", err)
			responses.BadRequest(w, r, "")
			return
		}

		err = s.Delete(ds)
		if err != nil {
			log.Println("Failed to delete data store:\n\t", err)
			responses.InternalServerError(w, r, "Failed to delete datastore")
			return
		}
	}
}
