package rbac

import (
	"errors"
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
	msgInvalidRoleName    = "Role names start with a lower-case letter and use only lower-case letters, digits and underscores, up to 63 characters"
	msgInvalidDescription = "Descriptions are at most 256 characters"
	msgInvalidScope       = "Scope names and values are required, must not contain a pipe, and are at most 64 and 128 characters"
	msgRoleNotFound       = "Role not found"
	msgPermissionNotFound = "Permission not found"
	msgRoleNameTaken      = "A role with that name already exists"
	msgPermissionExists   = "That permission already exists"
	msgRoleInUse          = "The role is assigned to an account"
	msgPermissionInUse    = "The permission is granted by a role"
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
	ScopeName  string `json:"scope_name" xml:"scope_name"`
	ScopeValue string `json:"scope_value" xml:"scope_value"`
}

type failureMapping struct {
	err     error
	respond func(http.ResponseWriter, *http.Request, string)
	msg     string
}

var failures = []failureMapping{
	{ErrInvalidID, responses.BadRequest, msgInvalidID},
	{ErrInvalidRoleName, responses.BadRequest, msgInvalidRoleName},
	{ErrInvalidDescription, responses.BadRequest, msgInvalidDescription},
	{ErrInvalidScope, responses.BadRequest, msgInvalidScope},
	{ErrRoleNotFound, responses.NotFound, msgRoleNotFound},
	{ErrPermissionNotFound, responses.NotFound, msgPermissionNotFound},
	{ErrRoleNameTaken, responses.Conflict, msgRoleNameTaken},
	{ErrPermissionExists, responses.Conflict, msgPermissionExists},
	{ErrRoleInUse, responses.Conflict, msgRoleInUse},
	{ErrPermissionInUse, responses.Conflict, msgPermissionInUse},
}

func respondFailure(w http.ResponseWriter, r *http.Request, err error) {
	for _, m := range failures {
		if errors.Is(err, m.err) {
			m.respond(w, r, m.msg)
			return
		}
	}
	log.Println(logFailedToHandleRbac, err)
	responses.InternalServerError(w, r, msgFailedToHandleRbac)
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
			respondFailure(w, r, err)
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
		if err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.StructOK(w, r, role)
	}
}

// GetRoleByNameHandler gets a role by name
func GetRoleByNameHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		role, err := s.GetRoleByName(r.PathValue("name"))
		if err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.StructOK(w, r, role)
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
		if err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.SendStruct(w, r, http.StatusCreated, role)
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
		if err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.StructOK(w, r, role)
	}
}

// DeleteRoleHandler deletes a role no account holds
func DeleteRoleHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if err := s.DeleteRole(r.PathValue("id")); err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.NoContent(w, r)
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
			respondFailure(w, r, err)
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
		if err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.StructOK(w, r, permission)
	}
}

// GetPermissionByScopeHandler gets a permission by its scope name and value
func GetPermissionByScopeHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		permission, err := s.GetPermissionByScope(r.PathValue("scope_name"), r.PathValue("scope_value"))
		if err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.StructOK(w, r, permission)
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
		permission, err := s.CreatePermission(body.ScopeName, body.ScopeValue)
		if err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.SendStruct(w, r, http.StatusCreated, permission)
	}
}

// DeletePermissionHandler deletes a permission no role grants
func DeletePermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if err := s.DeletePermission(r.PathValue("id")); err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.NoContent(w, r)
	}
}

// AttachPermissionHandler grants a permission to a role
func AttachPermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if err := s.AttachPermission(r.PathValue("id"), r.PathValue("permission_id")); err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.NoContent(w, r)
	}
}

// DetachPermissionHandler removes a permission from a role
func DetachPermissionHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if err := s.DetachPermission(r.PathValue("id"), r.PathValue("permission_id")); err != nil {
			respondFailure(w, r, err)
			return
		}
		responses.NoContent(w, r)
	}
}
