package rbac

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgNoPermission       = "You do not have permission to manage roles and permissions"
	msgUnableToParseBody  = "Invalid input, unable to parse body"
	msgInvalidID          = "The ID is not a valid ID"
	msgInvalidRoleName    = "Role names start with a lower-case letter and use only lower-case letters, digits and underscores"
	msgInvalidDescription = "Descriptions must be valid text"
	msgInvalidNode        = "Nodes are lower-case words of letters, digits and underscores, starting with a letter and joined by dots"
	msgInvalidValueType   = "Value types are int with merge max or min, string with merge first, and string_list with merge union"
	msgInvalidValue       = "The value must match the permission's type and be valid text without control or format characters or surrounding spaces, and permissions without a type take no value"
	msgRoleNotFound       = "Role not found"
	msgPermissionNotFound = "Permission not found"
	msgRoleNameTaken      = "A role with that name already exists"
	msgPermissionExists   = "That permission already exists"
	msgRoleInUse          = "The role is assigned to an account"
	msgPermissionInUse    = "The permission is granted by a role"
	msgBuiltinRole        = "Built-in roles cannot be deleted or renamed, and system and owner keep roles.admin"
	msgFailedToHandleRbac = "Failed to process the request"
	logFailedToHandleRbac = "[Error]: Unable to process roles and permissions request:\n\t"
)

type createRoleRequest struct {
	Name        string `json:"name" xml:"name"`
	Description string `json:"description" xml:"description"`
}

type updateRoleRequest struct {
	Name        *string `json:"name" xml:"name"`
	Description *string `json:"description" xml:"description"`
}

type createPermissionRequest struct {
	Node        string `json:"node" xml:"node"`
	Description string `json:"description" xml:"description"`
	ValueType   string `json:"value_type" xml:"value_type"`
	Merge       string `json:"merge" xml:"merge"`
}

type attachPermissionRequest struct {
	Value any `json:"value"`
}

func authorized(w http.ResponseWriter, r *http.Request) bool {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminRoles) {
		responses.Forbidden(w, r, msgNoPermission)
		return false
	}
	return true
}

// Handler serves the roles and permissions routes.
type Handler interface {
	ListRoles(w http.ResponseWriter, r *http.Request)
	GetRole(w http.ResponseWriter, r *http.Request)
	GetRoleByName(w http.ResponseWriter, r *http.Request)
	CreateRole(w http.ResponseWriter, r *http.Request)
	UpdateRole(w http.ResponseWriter, r *http.Request)
	DeleteRole(w http.ResponseWriter, r *http.Request)
	ListPermissions(w http.ResponseWriter, r *http.Request)
	GetPermission(w http.ResponseWriter, r *http.Request)
	GetPermissionByNode(w http.ResponseWriter, r *http.Request)
	CreatePermission(w http.ResponseWriter, r *http.Request)
	DeletePermission(w http.ResponseWriter, r *http.Request)
	AttachPermission(w http.ResponseWriter, r *http.Request)
	DetachPermission(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	s Service
}

// NewHandler returns a Handler that serves the roles and permissions routes from the given service.
func NewHandler(s Service) Handler {
	return &handler{s}
}

// ListRoles lists every role with its permissions
func (h *handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	roles, err := h.s.ListRoles(r.Context())
	if err != nil {
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
		return
	}
	responses.StructOK(w, r, roles)
}

// GetRole gets a role by ID
func (h *handler) GetRole(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	role, err := h.s.GetRole(r.Context(), r.PathValue("id"))
	switch err {
	case ErrInvalidID:
		responses.BadRequest(w, r, msgInvalidID)
	case ErrRoleNotFound:
		responses.NotFound(w, r, msgRoleNotFound)
	case nil:
		responses.StructOK(w, r, role)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// GetRoleByName gets a role by name
func (h *handler) GetRoleByName(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	role, err := h.s.GetRoleByName(r.Context(), r.PathValue("name"))
	switch err {
	case ErrRoleNotFound:
		responses.NotFound(w, r, msgRoleNotFound)
	case nil:
		responses.StructOK(w, r, role)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// CreateRole creates a role
func (h *handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	var body createRoleRequest
	if err := responses.DecodeStruct(r, &body); err != nil {
		responses.BadRequest(w, r, msgUnableToParseBody)
		return
	}
	role, err := h.s.CreateRole(r.Context(), body.Name, body.Description)
	switch err {
	case ErrInvalidRoleName:
		responses.BadRequest(w, r, msgInvalidRoleName)
	case ErrInvalidDescription:
		responses.BadRequest(w, r, msgInvalidDescription)
	case ErrRoleNameTaken:
		responses.Conflict(w, r, msgRoleNameTaken)
	case nil:
		responses.SendStruct(w, r, http.StatusCreated, role)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// UpdateRole renames a role or changes its description
func (h *handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	var body updateRoleRequest
	if err := responses.DecodeStruct(r, &body); err != nil {
		responses.BadRequest(w, r, msgUnableToParseBody)
		return
	}
	role, err := h.s.UpdateRole(r.Context(), r.PathValue("id"), body.Name, body.Description)
	switch err {
	case ErrInvalidID:
		responses.BadRequest(w, r, msgInvalidID)
	case ErrInvalidRoleName:
		responses.BadRequest(w, r, msgInvalidRoleName)
	case ErrInvalidDescription:
		responses.BadRequest(w, r, msgInvalidDescription)
	case ErrRoleNotFound:
		responses.NotFound(w, r, msgRoleNotFound)
	case ErrRoleNameTaken:
		responses.Conflict(w, r, msgRoleNameTaken)
	case ErrBuiltinRole:
		responses.Conflict(w, r, msgBuiltinRole)
	case nil:
		responses.StructOK(w, r, role)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// DeleteRole deletes a role no account holds
func (h *handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	err := h.s.DeleteRole(r.Context(), r.PathValue("id"))
	switch err {
	case ErrInvalidID:
		responses.BadRequest(w, r, msgInvalidID)
	case ErrRoleNotFound:
		responses.NotFound(w, r, msgRoleNotFound)
	case ErrRoleInUse:
		responses.Conflict(w, r, msgRoleInUse)
	case ErrBuiltinRole:
		responses.Conflict(w, r, msgBuiltinRole)
	case nil:
		responses.NoContent(w, r)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// ListPermissions lists every permission
func (h *handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	permissions, err := h.s.ListPermissions(r.Context())
	if err != nil {
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
		return
	}
	responses.StructOK(w, r, permissions)
}

// GetPermission gets a permission by ID
func (h *handler) GetPermission(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	permission, err := h.s.GetPermission(r.Context(), r.PathValue("id"))
	switch err {
	case ErrInvalidID:
		responses.BadRequest(w, r, msgInvalidID)
	case ErrPermissionNotFound:
		responses.NotFound(w, r, msgPermissionNotFound)
	case nil:
		responses.StructOK(w, r, permission)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// GetPermissionByNode gets a permission by its node
func (h *handler) GetPermissionByNode(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	permission, err := h.s.GetPermissionByNode(r.Context(), r.PathValue("node"))
	switch err {
	case ErrPermissionNotFound:
		responses.NotFound(w, r, msgPermissionNotFound)
	case nil:
		responses.StructOK(w, r, permission)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// CreatePermission creates a permission
func (h *handler) CreatePermission(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	var body createPermissionRequest
	if err := responses.DecodeStruct(r, &body); err != nil {
		responses.BadRequest(w, r, msgUnableToParseBody)
		return
	}
	permission, err := h.s.CreatePermission(r.Context(), body.Node, body.Description, body.ValueType, body.Merge)
	switch err {
	case ErrInvalidNode:
		responses.BadRequest(w, r, msgInvalidNode)
	case ErrInvalidDescription:
		responses.BadRequest(w, r, msgInvalidDescription)
	case ErrInvalidValueType:
		responses.BadRequest(w, r, msgInvalidValueType)
	case ErrPermissionExists:
		responses.Conflict(w, r, msgPermissionExists)
	case nil:
		responses.SendStruct(w, r, http.StatusCreated, permission)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// DeletePermission deletes a permission no role grants
func (h *handler) DeletePermission(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	err := h.s.DeletePermission(r.Context(), r.PathValue("id"))
	switch err {
	case ErrInvalidID:
		responses.BadRequest(w, r, msgInvalidID)
	case ErrPermissionNotFound:
		responses.NotFound(w, r, msgPermissionNotFound)
	case ErrPermissionInUse:
		responses.Conflict(w, r, msgPermissionInUse)
	case nil:
		responses.NoContent(w, r)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// AttachPermission grants a permission to a role, with a value if the permission takes one
func (h *handler) AttachPermission(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	var body attachPermissionRequest
	if r.ContentLength != 0 {
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			responses.BadRequest(w, r, msgUnableToParseBody)
			return
		}
	}
	err := h.s.AttachPermission(r.Context(), r.PathValue("id"), r.PathValue("permission_id"), body.Value)
	switch err {
	case ErrInvalidID:
		responses.BadRequest(w, r, msgInvalidID)
	case ErrInvalidValue:
		responses.BadRequest(w, r, msgInvalidValue)
	case ErrRoleNotFound:
		responses.NotFound(w, r, msgRoleNotFound)
	case ErrPermissionNotFound:
		responses.NotFound(w, r, msgPermissionNotFound)
	case nil:
		responses.NoContent(w, r)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}

// DetachPermission removes a permission from a role
func (h *handler) DetachPermission(w http.ResponseWriter, r *http.Request) {
	if !authorized(w, r) {
		return
	}
	err := h.s.DetachPermission(r.Context(), r.PathValue("id"), r.PathValue("permission_id"))
	switch err {
	case ErrInvalidID:
		responses.BadRequest(w, r, msgInvalidID)
	case ErrRoleNotFound:
		responses.NotFound(w, r, msgRoleNotFound)
	case ErrPermissionNotFound:
		responses.NotFound(w, r, msgPermissionNotFound)
	case ErrBuiltinRole:
		responses.Conflict(w, r, msgBuiltinRole)
	case nil:
		responses.NoContent(w, r)
	default:
		reqctx.Log(r.Context(), logFailedToHandleRbac, err.Error())
		responses.InternalServerError(w, r, msgFailedToHandleRbac)
	}
}
