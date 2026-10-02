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
	msgInvalidRoleName    = "Role names start with a lower-case letter and use only lower-case letters, digits and underscores, up to 63 characters"
	msgInvalidDescription = "Descriptions are valid text of at most 256 characters"
	msgInvalidNode        = "Nodes are lower-case words of letters, digits and underscores, starting with a letter and joined by dots, up to 128 characters"
	msgInvalidValueType   = "Value types are int with merge max or min, string with merge first, and string_list with merge union"
	msgInvalidValue       = "The value must match the permission's type and be valid text without control or format characters or surrounding spaces, and permissions without a type take no value"
	msgRoleNotFound       = "Role not found"
	msgPermissionNotFound = "Permission not found"
	msgRoleNameTaken      = "A role with that name already exists"
	msgPermissionExists   = "That permission already exists"
	msgRoleInUse          = "The role is assigned to an account"
	msgPermissionInUse    = "The permission is granted by a role"
	msgBuiltinRole        = "Built-in roles cannot be deleted or renamed, and system and owner keep the roles permission"
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

type failureMapping struct {
	err     error
	respond func(http.ResponseWriter, *http.Request, string)
	msg     string
}

var failures = []failureMapping{
	{ErrInvalidID, responses.BadRequest, msgInvalidID},
	{ErrInvalidRoleName, responses.BadRequest, msgInvalidRoleName},
	{ErrInvalidDescription, responses.BadRequest, msgInvalidDescription},
	{ErrInvalidNode, responses.BadRequest, msgInvalidNode},
	{ErrInvalidValueType, responses.BadRequest, msgInvalidValueType},
	{ErrInvalidValue, responses.BadRequest, msgInvalidValue},
	{ErrRoleNotFound, responses.NotFound, msgRoleNotFound},
	{ErrPermissionNotFound, responses.NotFound, msgPermissionNotFound},
	{ErrRoleNameTaken, responses.Conflict, msgRoleNameTaken},
	{ErrPermissionExists, responses.Conflict, msgPermissionExists},
	{ErrRoleInUse, responses.Conflict, msgRoleInUse},
	{ErrPermissionInUse, responses.Conflict, msgPermissionInUse},
	{ErrBuiltinRole, responses.Conflict, msgBuiltinRole},
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

// GetPermissionByNodeHandler gets a permission by its node
func GetPermissionByNodeHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		permission, err := s.GetPermissionByNode(r.PathValue("node"))
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
		permission, err := s.CreatePermission(body.Node, body.Description, body.ValueType, body.Merge)
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
		if err := s.AttachPermission(r.PathValue("id"), r.PathValue("permission_id"), body.Value); err != nil {
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
