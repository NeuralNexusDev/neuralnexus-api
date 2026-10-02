package petpictures

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

type hdMockStore struct {
	createPetResult *Pet
	createPetErr    error
	createPetCalls  []string

	getPetResult *Pet
	getPetErr    error
	getPetCalls  []int

	getPetByNameResult *Pet
	getPetByNameErr    error

	updatePetResult *Pet
	updatePetErr    error
	updatePetCalls  []*Pet

	createPetPictureResult *PetPicture
	createPetPictureErr    error

	getRandPetPictureByNameResult *PetPicture
	getRandPetPictureByNameErr    error
	getRandPetPictureByNameCalls  []string

	getPetPictureResult *PetPicture
	getPetPictureErr    error
	getPetPictureCalls  []string

	updatePetPictureResult *PetPicture
	updatePetPictureErr    error
	updatePetPictureCalls  []PetPicture

	deletePetPictureResult *PetPicture
	deletePetPictureErr    error
	deletePetPictureCalls  []string
}

func (m *hdMockStore) CreatePet(name string) (*Pet, error) {
	m.createPetCalls = append(m.createPetCalls, name)
	return m.createPetResult, m.createPetErr
}
func (m *hdMockStore) GetPet(id int) (*Pet, error) {
	m.getPetCalls = append(m.getPetCalls, id)
	return m.getPetResult, m.getPetErr
}
func (m *hdMockStore) GetPetByName(name string) (*Pet, error) {
	return m.getPetByNameResult, m.getPetByNameErr
}
func (m *hdMockStore) UpdatePet(pet *Pet) (*Pet, error) {
	m.updatePetCalls = append(m.updatePetCalls, pet)
	return m.updatePetResult, m.updatePetErr
}
func (m *hdMockStore) CreatePetPicture(id string, fileExt string, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error) {
	return m.createPetPictureResult, m.createPetPictureErr
}
func (m *hdMockStore) GetRandPetPictureByName(name string) (*PetPicture, error) {
	m.getRandPetPictureByNameCalls = append(m.getRandPetPictureByNameCalls, name)
	return m.getRandPetPictureByNameResult, m.getRandPetPictureByNameErr
}
func (m *hdMockStore) GetPetPicture(id string) (*PetPicture, error) {
	m.getPetPictureCalls = append(m.getPetPictureCalls, id)
	return m.getPetPictureResult, m.getPetPictureErr
}
func (m *hdMockStore) UpdatePetPicture(picture PetPicture) (*PetPicture, error) {
	m.updatePetPictureCalls = append(m.updatePetPictureCalls, picture)
	return m.updatePetPictureResult, m.updatePetPictureErr
}
func (m *hdMockStore) DeletePetPicture(id string) (*PetPicture, error) {
	m.deletePetPictureCalls = append(m.deletePetPictureCalls, id)
	return m.deletePetPictureResult, m.deletePetPictureErr
}

func hdCtxWithSession(session auth.Session) context.Context {
	return context.WithValue(context.Background(), mw.SessionKey, &session)
}

// hdSessionWithPermissions builds a session whose Permissions are exactly the given strings.
func hdSessionWithPermissions(permissions ...string) auth.Session {
	return auth.Session{Permissions: permissions}
}

func hdDecodeJSON(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("failed to decode JSON body %q: %v", body, err)
	}
	return m
}

func hdRequireDetail(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", w.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func TestHD01to05_CreatePetHandler(t *testing.T) {
	t.Run("HD-01_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{createPetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(auth.Session{}))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		hdRequireDetail(t, w, msgNoPermissionToCreatePet)
		if len(mock.createPetCalls) != 0 {
			t.Errorf("CreatePet called %d times, want 0", len(mock.createPetCalls))
		}
	})

	t.Run("HD-02_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{createPetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopeAdminPetPictures.Node)))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
		}
		if len(mock.createPetCalls) != 1 || mock.createPetCalls[0] != "Rex" {
			t.Errorf("CreatePet calls = %v, want [Rex]", mock.createPetCalls)
		}
		body := hdDecodeJSON(t, w.Body.Bytes())
		if body["name"] != "Rex" {
			t.Errorf("response name = %v, want Rex", body["name"])
		}
	})

	t.Run("HD-03_NameFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{createPetResult: &Pet{ID: 2, Name: "Fido"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPost, "/pets", strings.NewReader(`{"name":"Fido"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopeAdminPetPictures.Node)))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
		}
		if len(mock.createPetCalls) != 1 || mock.createPetCalls[0] != "Fido" {
			t.Errorf("CreatePet calls = %v, want [Fido]", mock.createPetCalls)
		}
	})

	t.Run("HD-04_NoNameBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPost, "/pets", nil)
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopeAdminPetPictures.Node)))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetNameIsRequired)
		if len(mock.createPetCalls) != 0 {
			t.Errorf("CreatePet called %d times, want 0", len(mock.createPetCalls))
		}
	})

	t.Run("HD-05_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{createPetErr: testerrors.ErrDBDown}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopeAdminPetPictures.Node)))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToCreatePet)
	})
}

func TestHD06to10_GetPetHandler(t *testing.T) {
	t.Run("HD-06_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{getPetResult: &Pet{ID: 7, Name: "Rex"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/7", nil)
		req.SetPathValue("id", "7")
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getPetCalls) != 1 || mock.getPetCalls[0] != 7 {
			t.Errorf("GetPet calls = %v, want [7]", mock.getPetCalls)
		}
	})

	t.Run("HD-07_IDFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{getPetResult: &Pet{ID: 7, Name: "Rex"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets", strings.NewReader(`{"id":7}`))
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getPetCalls) != 1 || mock.getPetCalls[0] != 7 {
			t.Errorf("GetPet calls = %v, want [7]", mock.getPetCalls)
		}
	})

	t.Run("HD-08_NoIDBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets", nil)
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetIDIsRequired)
		if len(mock.getPetCalls) != 0 {
			t.Errorf("GetPet called %d times, want 0", len(mock.getPetCalls))
		}
	})

	t.Run("HD-09_NonNumericPathValueNoBodyFallback", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/abc", nil)
		req.SetPathValue("id", "abc")
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetIDIsRequired)
		if len(mock.getPetCalls) != 0 {
			t.Errorf("GetPet called %d times, want 0", len(mock.getPetCalls))
		}
	})

	t.Run("HD-10_StoreErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: ErrPetNotFound}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/7", nil)
		req.SetPathValue("id", "7")
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetNotFound)
	})
}

func TestHD11to14and35_UpdatePetHandler(t *testing.T) {
	t.Run("HD-11_InvalidBodyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pets", strings.NewReader(`not json`))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgUnableToParseBody)
		if len(mock.updatePetCalls) != 0 {
			t.Errorf("UpdatePet called %d times, want 0", len(mock.updatePetCalls))
		}
	})

	t.Run("HD-12_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":SomeoneElsesPet")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		hdRequireDetail(t, w, msgNoPermissionToUpdatePet)
		if len(mock.updatePetCalls) != 0 {
			t.Errorf("UpdatePet called %d times, want 0", len(mock.updatePetCalls))
		}
	})

	t.Run("HD-13_HappyPath", func(t *testing.T) {
		mock := &hdMockStore{updatePetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.updatePetCalls) != 1 || mock.updatePetCalls[0].Name != "Rex" {
			t.Errorf("UpdatePet calls = %+v, want a single call with Name=Rex", mock.updatePetCalls)
		}
	})

	t.Run("HD-14_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{updatePetErr: testerrors.ErrDBDown}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToUpdatePet)
	})

	t.Run("HD-35_StoreNoRowsNotFound", func(t *testing.T) {
		mock := &hdMockStore{updatePetErr: ErrPetNotFound}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetNotFound)
	})
}

func TestHD15to18_GetRandPetPictureByNameHandler(t *testing.T) {
	t.Run("HD-15_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameResult: &PetPicture{ID: "abc"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
		req.SetPathValue("name", "Rex")
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getRandPetPictureByNameCalls) != 1 || mock.getRandPetPictureByNameCalls[0] != "Rex" {
			t.Errorf("GetRandPetPictureByName calls = %v, want [Rex]", mock.getRandPetPictureByNameCalls)
		}
	})

	t.Run("HD-16_NameFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameResult: &PetPicture{ID: "abc"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/picture", strings.NewReader(`{"name":"Rex"}`))
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getRandPetPictureByNameCalls) != 1 || mock.getRandPetPictureByNameCalls[0] != "Rex" {
			t.Errorf("GetRandPetPictureByName calls = %v, want [Rex]", mock.getRandPetPictureByNameCalls)
		}
	})

	t.Run("HD-17_NoNameBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/picture", nil)
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetNameIsRequired)
	})

	t.Run("HD-18_StoreErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameErr: ErrPetPictureNotFound}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
		req.SetPathValue("name", "Rex")
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetPictureNotFound)
	})
}

func TestHD19to22_GetPetPictureHandler(t *testing.T) {
	t.Run("HD-19_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureResult: &PetPicture{ID: "abc123"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getPetPictureCalls) != 1 || mock.getPetPictureCalls[0] != "abc123" {
			t.Errorf("GetPetPicture calls = %v, want [abc123]", mock.getPetPictureCalls)
		}
	})

	t.Run("HD-20_IDFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureResult: &PetPicture{ID: "abc123"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pictures", strings.NewReader(`{"id":"abc123"}`))
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getPetPictureCalls) != 1 || mock.getPetPictureCalls[0] != "abc123" {
			t.Errorf("GetPetPicture calls = %v, want [abc123]", mock.getPetPictureCalls)
		}
	})

	t.Run("HD-21_NoIDBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pictures", nil)
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetPictureIDIsRequired)
	})

	t.Run("HD-22_StoreErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: ErrPetPictureNotFound}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetPictureNotFound)
	})
}

func TestHD23to27and36_UpdatePetPictureHandler(t *testing.T) {
	t.Run("HD-23_InvalidBodyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`not json`))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgUnableToParseBody)
		if len(mock.getPetCalls) != 0 || len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("expected no store calls, got GetPet=%v UpdatePetPicture=%v", mock.getPetCalls, mock.updatePetPictureCalls)
		}
	})

	t.Run("HD-24_PetNotFoundNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: ErrPetNotFound}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetNotFound)
		if len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("UpdatePetPicture called %d times, want 0", len(mock.updatePetPictureCalls))
		}
	})

	t.Run("HD-25_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{getPetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":SomeoneElse")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		hdRequireDetail(t, w, msgNoPermissionToUpdatePet)
		if len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("UpdatePetPicture called %d times, want 0", len(mock.updatePetPictureCalls))
		}
	})

	t.Run("HD-26_HappyPath", func(t *testing.T) {
		mock := &hdMockStore{
			getPetResult:           &Pet{ID: 1, Name: "Rex"},
			updatePetPictureResult: &PetPicture{ID: "abc123"},
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.updatePetPictureCalls) != 1 || mock.updatePetPictureCalls[0].ID != "abc123" {
			t.Errorf("UpdatePetPicture calls = %+v, want a single call with ID=abc123", mock.updatePetPictureCalls)
		}
	})

	t.Run("HD-27_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
			updatePetPictureErr: testerrors.ErrDBDown,
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToUpdatePetPicture)
	})

	t.Run("HD-36_StoreNoRowsNotFound", func(t *testing.T) {
		mock := &hdMockStore{
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
			updatePetPictureErr: ErrPetPictureNotFound,
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetPictureNotFound)
	})
}

func TestHD28to34_DeletePetPictureHandler(t *testing.T) {
	t.Run("HD-28_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if len(mock.deletePetPictureCalls) != 1 || mock.deletePetPictureCalls[0] != "abc123" {
			t.Errorf("DeletePetPicture calls = %v, want [abc123]", mock.deletePetPictureCalls)
		}
	})

	t.Run("HD-29_IDFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures", strings.NewReader(`{"id":"abc123"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if len(mock.getPetPictureCalls) != 1 || mock.getPetPictureCalls[0] != "abc123" {
			t.Errorf("GetPetPicture calls = %v, want [abc123]", mock.getPetPictureCalls)
		}
	})

	t.Run("HD-30_NoIDBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures", nil)
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetPictureIDIsRequired)
	})

	t.Run("HD-31_GetPetPictureErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: ErrPetPictureNotFound}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetPictureNotFound)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("HD-32_GetPetErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetErr:           ErrPetNotFound,
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetNotFound)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("HD-33_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":SomeoneElse")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		hdRequireDetail(t, w, msgNoPermissionToUpdatePet)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("HD-34_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
			deletePetPictureErr: testerrors.ErrDBDown,
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToDeletePetPicture)
	})
}

func TestHD37to44_StoreFailureMapping(t *testing.T) {
	t.Run("HD-37_CreatePetNameEmptyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{createPetErr: ErrPetNameEmpty}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopeAdminPetPictures.Node)))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetNameMustNotBeEmpty)
	})

	t.Run("HD-38_UpdatePetNameEmptyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{updatePetErr: ErrPetNameEmpty}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions(perms.ScopePetPictures.Node + ":Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		hdRequireDetail(t, w, msgPetNameMustNotBeEmpty)
	})

	t.Run("HD-39_GetPetStoreFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: testerrors.ErrDBDown}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/7", nil)
		req.SetPathValue("id", "7")
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToGetPet)
	})

	t.Run("HD-45_GetRandPetPicturePetNotFound", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameErr: ErrPetNotFound}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
		req.SetPathValue("name", "Rex")
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		hdRequireDetail(t, w, msgPetNotFound)
	})

	t.Run("HD-40_GetRandPetPictureStoreFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameErr: testerrors.ErrDBDown}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
		req.SetPathValue("name", "Rex")
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToGetRandomPetPicture)
	})

	t.Run("HD-41_GetPetPictureStoreFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: testerrors.ErrDBDown}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodGet, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToGetPetPicture)
	})

	t.Run("HD-42_UpdatePetPictureGetPetFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: testerrors.ErrDBDown}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToGetPet)
		if len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("UpdatePetPicture called %d times, want 0", len(mock.updatePetPictureCalls))
		}
	})

	t.Run("HD-43_DeletePetPictureGetPetPictureFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: testerrors.ErrDBDown}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToGetPetPicture)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("HD-44_DeletePetPictureGetPetFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetErr:           testerrors.ErrDBDown,
		}
		svc := NewService(mock)
		req := httptest.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgUnableToGetPet)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})
}
