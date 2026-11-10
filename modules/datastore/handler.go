package datastore

import (
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
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
	msgDataStoreNotFound             = "Data store not found"
	msgOwnerNotFound                 = "Owner account not found"
	msgDataStoreInUse                = "Data store is still in use and can't be deleted"
	msgInvalidDataStoreID            = "Data store and owner ids must be numbers"
)

// Handler serves the datastore routes.
type Handler interface {
	CreateDataStore(w http.ResponseWriter, r *http.Request)
	ReadDataStore(w http.ResponseWriter, r *http.Request)
	UpdateDataStore(w http.ResponseWriter, r *http.Request)
	DeleteDataStore(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	s DSService
}

// NewHandler returns a Handler that serves the datastore routes from the given service.
func NewHandler(s DSService) Handler {
	return &handler{s}
}

// CreateDataStore - Create a new data store
func (h *handler) CreateDataStore(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminDataStore) {
		responses.Forbidden(w, r, msgNoPermissionToCreateDatastore)
		return
	}

	id, err := database.GenSnowflake()
	if err != nil {
		panic("snowflake generation failed although config.FromEnv limits SNOWFLAKE_NODE_ID and SNOWFLAKE_WORKER_ID to 31: " + err.Error())
	}
	ds := NewDataStore(id, session.UserID)
	ds, err = h.s.Create(r.Context(), ds)
	if err != nil {
		reqctx.Log(r.Context(), "Failed to create data store:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToCreateDatastore)
		return
	}
	responses.StructOK(w, r, ds)
}

// ReadDataStore - Read a data store
func (h *handler) ReadDataStore(w http.ResponseWriter, r *http.Request) {
	var ds *Store
	err := responses.DecodeStruct(r, &ds)
	if err != nil {
		reqctx.Log(r.Context(), logBadBody, err.Error())
		responses.BadRequest(w, r, "")
		return
	}
	ds, err = h.s.Read(r.Context(), ds)
	switch err {
	case nil:
		responses.StructOK(w, r, ds)
	case ErrDataStoreNotFound:
		responses.NotFound(w, r, msgDataStoreNotFound)
	case ErrDataStoreInvalidID:
		responses.BadRequest(w, r, msgInvalidDataStoreID)
	default:
		reqctx.Log(r.Context(), "Failed to read data store:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToReadDatastore)
	}
}

// UpdateDataStore - Update a data store
func (h *handler) UpdateDataStore(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminDataStore) {
		responses.Forbidden(w, r, msgNoPermissionToUpdateDatastore)
		return
	}

	var ds *Store
	err := responses.DecodeStruct(r, &ds)
	if err != nil {
		reqctx.Log(r.Context(), logBadBody, err.Error())
		responses.BadRequest(w, r, "")
		return
	}

	ds, err = h.s.Update(r.Context(), ds)
	switch err {
	case nil:
		responses.StructOK(w, r, ds)
	case ErrDataStoreNotFound:
		responses.NotFound(w, r, msgDataStoreNotFound)
	case ErrDataStoreOwnerNotFound:
		responses.NotFound(w, r, msgOwnerNotFound)
	case ErrDataStoreInvalidID:
		responses.BadRequest(w, r, msgInvalidDataStoreID)
	default:
		reqctx.Log(r.Context(), "Failed to update data store:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToUpdateDatastore)
	}
}

// DeleteDataStore - Delete a data store
func (h *handler) DeleteDataStore(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminDataStore) {
		responses.Forbidden(w, r, msgNoPermissionToDeleteDatastore)
		return
	}

	var ds *Store
	err := responses.DecodeStruct(r, &ds)
	if err != nil {
		reqctx.Log(r.Context(), logBadBody, err.Error())
		responses.BadRequest(w, r, "")
		return
	}

	err = h.s.Delete(r.Context(), ds)
	switch err {
	case nil:
		responses.NoContent(w, r)
	case ErrDataStoreNotFound:
		responses.NotFound(w, r, msgDataStoreNotFound)
	case ErrDataStoreInvalidID:
		responses.BadRequest(w, r, msgInvalidDataStoreID)
	case ErrDataStoreInUse:
		responses.Conflict(w, r, msgDataStoreInUse)
	default:
		reqctx.Log(r.Context(), "Failed to delete data store:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToDeleteDatastore)
	}
}
