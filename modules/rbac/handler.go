package rbac

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
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
	session := r.Context().Value(mw.SessionKey).(*auth.Session)
	if !session.HasPermission(perms.ScopeAdminRoles) {
		responses.Forbidden(w, r, msgNoPermission)
		return false
	}
	return true
}

// ListRolesHandler lists every role with its permissions
func ListRolesHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		roles, err := s.ListRoles()
		if err != nil {
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
			return
		}
		responses.StructOK(w, r, roles)
	}
}

// GetRoleHandler gets a role by ID
func GetRoleHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		role, err := s.GetRole(r.PathValue("id"))
		switch {
		case errors.Is(err, ErrInvalidID):
			responses.BadRequest(w, r, msgInvalidID)
		case errors.Is(err, ErrRoleNotFound):
			responses.NotFound(w, r, msgRoleNotFound)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.StructOK(w, r, role)
		}
	}
}

// GetRoleByNameHandler gets a role by name
func GetRoleByNameHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		role, err := s.GetRoleByName(r.PathValue("name"))
		switch {
		case errors.Is(err, ErrRoleNotFound):
			responses.NotFound(w, r, msgRoleNotFound)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.StructOK(w, r, role)
		}
	}
}

// CreateRoleHandler creates a role
func CreateRoleHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		var body createRoleRequest
		if err := responses.DecodeStruct(r, &body); err != nil {
			responses.BadRequest(w, r, msgUnableToParseBody)
			return
		}
		role, err := s.CreateRole(body.Name, body.Description)
		switch {
		case errors.Is(err, ErrInvalidRoleName):
			responses.BadRequest(w, r, msgInvalidRoleName)
		case errors.Is(err, ErrInvalidDescription):
			responses.BadRequest(w, r, msgInvalidDescription)
		case errors.Is(err, ErrRoleNameTaken):
			responses.Conflict(w, r, msgRoleNameTaken)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.SendStruct(w, r, http.StatusCreated, role)
		}
	}
}

// UpdateRoleHandler renames a role or changes its description
func UpdateRoleHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		var body updateRoleRequest
		if err := responses.DecodeStruct(r, &body); err != nil {
			responses.BadRequest(w, r, msgUnableToParseBody)
			return
		}
		role, err := s.UpdateRole(r.PathValue("id"), body.Name, body.Description)
		switch {
		case errors.Is(err, ErrInvalidID):
			responses.BadRequest(w, r, msgInvalidID)
		case errors.Is(err, ErrInvalidRoleName):
			responses.BadRequest(w, r, msgInvalidRoleName)
		case errors.Is(err, ErrInvalidDescription):
			responses.BadRequest(w, r, msgInvalidDescription)
		case errors.Is(err, ErrRoleNotFound):
			responses.NotFound(w, r, msgRoleNotFound)
		case errors.Is(err, ErrRoleNameTaken):
			responses.Conflict(w, r, msgRoleNameTaken)
		case errors.Is(err, ErrBuiltinRole):
			responses.Conflict(w, r, msgBuiltinRole)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.StructOK(w, r, role)
		}
	}
}

// DeleteRoleHandler deletes a role no account holds
func DeleteRoleHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		err := s.DeleteRole(r.PathValue("id"))
		switch {
		case errors.Is(err, ErrInvalidID):
			responses.BadRequest(w, r, msgInvalidID)
		case errors.Is(err, ErrRoleNotFound):
			responses.NotFound(w, r, msgRoleNotFound)
		case errors.Is(err, ErrRoleInUse):
			responses.Conflict(w, r, msgRoleInUse)
		case errors.Is(err, ErrBuiltinRole):
			responses.Conflict(w, r, msgBuiltinRole)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.NoContent(w, r)
		}
	}
}

// ListPermissionsHandler lists every permission
func ListPermissionsHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		permissions, err := s.ListPermissions()
		if err != nil {
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
			return
		}
		responses.StructOK(w, r, permissions)
	}
}

// GetPermissionHandler gets a permission by ID
func GetPermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		permission, err := s.GetPermission(r.PathValue("id"))
		switch {
		case errors.Is(err, ErrInvalidID):
			responses.BadRequest(w, r, msgInvalidID)
		case errors.Is(err, ErrPermissionNotFound):
			responses.NotFound(w, r, msgPermissionNotFound)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.StructOK(w, r, permission)
		}
	}
}

// GetPermissionByNodeHandler gets a permission by its node
func GetPermissionByNodeHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		permission, err := s.GetPermissionByNode(r.PathValue("node"))
		switch {
		case errors.Is(err, ErrPermissionNotFound):
			responses.NotFound(w, r, msgPermissionNotFound)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.StructOK(w, r, permission)
		}
	}
}

// CreatePermissionHandler creates a permission
func CreatePermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		var body createPermissionRequest
		if err := responses.DecodeStruct(r, &body); err != nil {
			responses.BadRequest(w, r, msgUnableToParseBody)
			return
		}
		permission, err := s.CreatePermission(body.Node, body.Description, body.ValueType, body.Merge)
		switch {
		case errors.Is(err, ErrInvalidNode):
			responses.BadRequest(w, r, msgInvalidNode)
		case errors.Is(err, ErrInvalidDescription):
			responses.BadRequest(w, r, msgInvalidDescription)
		case errors.Is(err, ErrInvalidValueType):
			responses.BadRequest(w, r, msgInvalidValueType)
		case errors.Is(err, ErrPermissionExists):
			responses.Conflict(w, r, msgPermissionExists)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.SendStruct(w, r, http.StatusCreated, permission)
		}
	}
}

// DeletePermissionHandler deletes a permission no role grants
func DeletePermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		err := s.DeletePermission(r.PathValue("id"))
		switch {
		case errors.Is(err, ErrInvalidID):
			responses.BadRequest(w, r, msgInvalidID)
		case errors.Is(err, ErrPermissionNotFound):
			responses.NotFound(w, r, msgPermissionNotFound)
		case errors.Is(err, ErrPermissionInUse):
			responses.Conflict(w, r, msgPermissionInUse)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.NoContent(w, r)
		}
	}
}

// AttachPermissionHandler grants a permission to a role, with a value if the permission takes one
func AttachPermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		err := s.AttachPermission(r.PathValue("id"), r.PathValue("permission_id"), body.Value)
		switch {
		case errors.Is(err, ErrInvalidID):
			responses.BadRequest(w, r, msgInvalidID)
		case errors.Is(err, ErrInvalidValue):
			responses.BadRequest(w, r, msgInvalidValue)
		case errors.Is(err, ErrRoleNotFound):
			responses.NotFound(w, r, msgRoleNotFound)
		case errors.Is(err, ErrPermissionNotFound):
			responses.NotFound(w, r, msgPermissionNotFound)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.NoContent(w, r)
		}
	}
}

// DetachPermissionHandler removes a permission from a role
func DetachPermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		err := s.DetachPermission(r.PathValue("id"), r.PathValue("permission_id"))
		switch {
		case errors.Is(err, ErrInvalidID):
			responses.BadRequest(w, r, msgInvalidID)
		case errors.Is(err, ErrRoleNotFound):
			responses.NotFound(w, r, msgRoleNotFound)
		case errors.Is(err, ErrPermissionNotFound):
			responses.NotFound(w, r, msgPermissionNotFound)
		case errors.Is(err, ErrBuiltinRole):
			responses.Conflict(w, r, msgBuiltinRole)
		case err != nil:
			log.Println(logFailedToHandleRbac, err)
			responses.InternalServerError(w, r, msgFailedToHandleRbac)
		default:
			responses.NoContent(w, r)
		}
	}
}
