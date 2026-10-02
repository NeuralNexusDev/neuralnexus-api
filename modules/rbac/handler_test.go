package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

type stubService struct {
	err  error
	args []string
}

func (s *stubService) CreateRole(name, description string) (*Role, error) {
	s.args = []string{name, description}
	return &Role{ID: "1", Name: name}, s.err
}
func (s *stubService) GetRole(id string) (*Role, error) {
	s.args = []string{id}
	return &Role{ID: id}, s.err
}
func (s *stubService) GetRoleByName(name string) (*Role, error) {
	s.args = []string{name}
	return &Role{Name: name}, s.err
}
func (s *stubService) ListRoles() ([]*Role, error) { return []*Role{{ID: "1"}}, s.err }
func (s *stubService) UpdateRole(id string, name, description *string) (*Role, error) {
	s.args = []string{id}
	if name != nil {
		s.args = append(s.args, "name="+*name)
	}
	if description != nil {
		s.args = append(s.args, "description="+*description)
	}
	return &Role{ID: id}, s.err
}
func (s *stubService) DeleteRole(id string) error { s.args = []string{id}; return s.err }
func (s *stubService) CreatePermission(node, description, valueType, merge string) (*Permission, error) {
	s.args = []string{node, description, valueType, merge}
	return &Permission{ID: "1", Node: node, Description: description, ValueType: valueType, Merge: merge}, s.err
}
func (s *stubService) GetPermission(id string) (*Permission, error) {
	s.args = []string{id}
	return &Permission{ID: id}, s.err
}
func (s *stubService) GetPermissionByNode(node string) (*Permission, error) {
	s.args = []string{node}
	return &Permission{Node: node}, s.err
}
func (s *stubService) ListPermissions() ([]*Permission, error) {
	return []*Permission{{ID: "1"}}, s.err
}
func (s *stubService) DeletePermission(id string) error { s.args = []string{id}; return s.err }
func (s *stubService) AttachPermission(roleID, permissionID string, value any) error {
	s.args = []string{roleID, permissionID, fmt.Sprint(value)}
	return s.err
}
func (s *stubService) DetachPermission(roleID, permissionID string) error {
	s.args = []string{roleID, permissionID}
	return s.err
}
func (s *stubService) GetPermissionsForRoles([]string) ([]string, error) { return nil, s.err }

type handlerCase struct {
	name    string
	handler func(Service) http.HandlerFunc
	body    string
	paths   map[string]string
	status  int
	args    []string
}

var handlerCases = []handlerCase{
	{"ListRoles", ListRolesHandler, "", nil, http.StatusOK, nil},
	{"GetRole", GetRoleHandler, "", map[string]string{"id": "7"}, http.StatusOK, []string{"7"}},
	{"GetRoleByName", GetRoleByNameHandler, "", map[string]string{"name": "mod"}, http.StatusOK, []string{"mod"}},
	{"CreateRole", CreateRoleHandler, `{"name":"mod","description":"d"}`, nil, http.StatusCreated, []string{"mod", "d"}},
	{"UpdateRole", UpdateRoleHandler, `{"name":"n","description":"d"}`, map[string]string{"id": "7"}, http.StatusOK, []string{"7", "name=n", "description=d"}},
	{"DeleteRole", DeleteRoleHandler, "", map[string]string{"id": "7"}, http.StatusNoContent, []string{"7"}},
	{"ListPermissions", ListPermissionsHandler, "", nil, http.StatusOK, nil},
	{"GetPermission", GetPermissionHandler, "", map[string]string{"id": "8"}, http.StatusOK, []string{"8"}},
	{"GetPermissionByNode", GetPermissionByNodeHandler, "", map[string]string{"node": "a.b"}, http.StatusOK, []string{"a.b"}},
	{"CreatePermission", CreatePermissionHandler, `{"node":"a.b","description":"d","value_type":"int","merge":"max"}`, nil, http.StatusCreated, []string{"a.b", "d", "int", "max"}},
	{"DeletePermission", DeletePermissionHandler, "", map[string]string{"id": "8"}, http.StatusNoContent, []string{"8"}},
	{"AttachPermission", AttachPermissionHandler, "", map[string]string{"id": "7", "permission_id": "8"}, http.StatusNoContent, []string{"7", "8", "<nil>"}},
	{"AttachPermissionWithValue", AttachPermissionHandler, `{"value":["a","b"]}`, map[string]string{"id": "7", "permission_id": "8"}, http.StatusNoContent, []string{"7", "8", "[a b]"}},
	{"AttachPermissionWithNumber", AttachPermissionHandler, `{"value":1000}`, map[string]string{"id": "7", "permission_id": "8"}, http.StatusNoContent, []string{"7", "8", "1000"}},
	{"DetachPermission", DetachPermissionHandler, "", map[string]string{"id": "7", "permission_id": "8"}, http.StatusNoContent, []string{"7", "8"}},
}

var handlerBodies = map[string]string{
	"ListRoles": `[{"id":"1"`, "GetRole": `"id":"7"`, "GetRoleByName": `"name":"mod"`, "CreateRole": `"name":"mod"`,
	"UpdateRole": `"id":"7"`, "ListPermissions": `[{"id":"1"`, "GetPermission": `"id":"8"`,
	"GetPermissionByNode": `"node":"a.b"`, "CreatePermission": `"node":"a.b","description":"d","value_type":"int","merge":"max"`,
}

func rbSession(permissions ...string) *auth.Session {
	return &auth.Session{UserID: "u1", Permissions: permissions}
}

func rbRequest(c handlerCase, session *auth.Session, body string) *http.Request {
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	for k, v := range c.paths {
		r.SetPathValue(k, v)
	}
	return r
}

func rbAdmin() *auth.Session {
	return rbSession(perms.ScopeAdminRoles.Node)
}

func rbDetail(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("response is not a problem document: %v: %s", err, w.Body.String())
	}
	return p.Detail
}

func TestRH01to03Handlers(t *testing.T) {
	t.Run("RH-01_EveryHandlerRejectsASessionWithoutTheRolesAdminNode", func(t *testing.T) {
		for _, c := range handlerCases {
			for _, session := range []*auth.Session{rbSession(), rbSession("roles.other"), rbSession("roles.admins"), rbSession("other.admin"), rbSession("roles")} {
				svc := &stubService{}
				w := httptest.NewRecorder()
				c.handler(svc)(w, rbRequest(c, session, c.body))
				if w.Code != http.StatusForbidden || rbDetail(t, w) != msgNoPermission {
					t.Fatalf("%s with %v: got %d %s", c.name, session.Permissions, w.Code, w.Body.String())
				}
				if svc.args != nil {
					t.Fatalf("%s reached the service without permission", c.name)
				}
			}
		}
	})
	t.Run("RH-02_EveryHandlerSucceedsForTheRolesAdminNodeAndPassesItsInputs", func(t *testing.T) {
		for _, c := range handlerCases {
			svc := &stubService{}
			w := httptest.NewRecorder()
			c.handler(svc)(w, rbRequest(c, rbAdmin(), c.body))
			if w.Code != c.status {
				t.Fatalf("%s: got %d %s, want %d", c.name, w.Code, w.Body.String(), c.status)
			}
			if fmt.Sprint(svc.args) != fmt.Sprint(c.args) {
				t.Fatalf("%s: service got %v, want %v", c.name, svc.args, c.args)
			}
			if want := handlerBodies[c.name]; !strings.Contains(w.Body.String(), want) || (want == "" && w.Body.Len() != 0) {
				t.Fatalf("%s: body %q, want it to contain %q", c.name, w.Body.String(), want)
			}
		}
	})
	t.Run("RH-03_BodyHandlersRejectUnparseableBodies", func(t *testing.T) {
		for _, c := range handlerCases {
			if c.body == "" {
				continue
			}
			svc := &stubService{}
			w := httptest.NewRecorder()
			c.handler(svc)(w, rbRequest(c, rbAdmin(), "{not json"))
			if w.Code != http.StatusBadRequest || rbDetail(t, w) != msgUnableToParseBody {
				t.Fatalf("%s: got %d %s", c.name, w.Code, w.Body.String())
			}
			if svc.args != nil {
				t.Fatalf("%s reached the service with a bad body", c.name)
			}
		}
	})
}

func TestRH04ServiceFailureMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		msg    string
	}{
		{ErrInvalidID, http.StatusBadRequest, msgInvalidID},
		{ErrInvalidRoleName, http.StatusBadRequest, msgInvalidRoleName},
		{ErrInvalidDescription, http.StatusBadRequest, msgInvalidDescription},
		{ErrInvalidNode, http.StatusBadRequest, msgInvalidNode},
		{ErrInvalidValueType, http.StatusBadRequest, msgInvalidValueType},
		{ErrInvalidValue, http.StatusBadRequest, msgInvalidValue},
		{ErrRoleNotFound, http.StatusNotFound, msgRoleNotFound},
		{ErrPermissionNotFound, http.StatusNotFound, msgPermissionNotFound},
		{ErrRoleNameTaken, http.StatusConflict, msgRoleNameTaken},
		{ErrPermissionExists, http.StatusConflict, msgPermissionExists},
		{ErrRoleInUse, http.StatusConflict, msgRoleInUse},
		{ErrPermissionInUse, http.StatusConflict, msgPermissionInUse},
		{ErrBuiltinRole, http.StatusConflict, msgBuiltinRole},
		{fmt.Errorf("wrapped: %w", ErrRoleNotFound), http.StatusNotFound, msgRoleNotFound},
		{errors.New("boom"), http.StatusInternalServerError, msgFailedToHandleRbac},
	}
	t.Run("RH-04_EveryHandlerMapsServiceFailuresToTheirProblemResponse", func(t *testing.T) {
		for _, m := range cases {
			for _, c := range handlerCases {
				w := httptest.NewRecorder()
				c.handler(&stubService{err: m.err})(w, rbRequest(c, rbAdmin(), c.body))
				if w.Code != m.status || rbDetail(t, w) != m.msg {
					t.Fatalf("%s with %v: got %d %s, want %d %s", c.name, m.err, w.Code, w.Body.String(), m.status, m.msg)
				}
			}
		}
	})
}

func TestRH05AttachValueNumbers(t *testing.T) {
	attach := func(body string) (*httptest.ResponseRecorder, *fakeStore) {
		f := &fakeStore{role: &Role{Name: "mod"}, permission: &Permission{ID: "8", Node: "a.b", ValueType: ValueTypeInt, Merge: MergeMax}}
		c := handlerCase{paths: map[string]string{"id": "7", "permission_id": "8"}}
		w := httptest.NewRecorder()
		AttachPermissionHandler(NewService(f))(w, rbRequest(c, rbAdmin(), body))
		return w, f
	}

	t.Run("RH-05_IntValuesAreReadAsExactNumbers", func(t *testing.T) {
		w, f := attach(`{"value":1000}`)
		if w.Code != http.StatusNoContent || string(f.attachedValue) != "1000" {
			t.Fatalf("got %d, stored %q", w.Code, f.attachedValue)
		}
		for _, body := range []string{`{"value":9007199254740992}`, `{"value":-9007199254740992}`} {
			w, f := attach(body)
			if w.Code != http.StatusNoContent || len(f.attachedValue) == 0 {
				t.Fatalf("body %s: got %d %s, want the boundary accepted", body, w.Code, w.Body.String())
			}
		}
		for _, body := range []string{`{"value":9007199254740993}`, `{"value":-9007199254740993}`, `{"value":1e2}`, `{"value":5.0}`, `{"value":1.5}`, `{"value":"5"}`, `{}`} {
			w, f := attach(body)
			if w.Code != http.StatusBadRequest || rbDetail(t, w) != msgInvalidValue || slices.Contains(f.calls, "AttachPermission") {
				t.Fatalf("body %s: got %d %s with calls %v, want 400 and no attach", body, w.Code, w.Body.String(), f.calls)
			}
		}
	})
}
