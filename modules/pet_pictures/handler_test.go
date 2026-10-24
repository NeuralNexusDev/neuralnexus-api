package petpictures

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
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

func (m *hdMockStore) CreatePet(_ context.Context, name string) (*Pet, error) {
	m.createPetCalls = append(m.createPetCalls, name)
	return m.createPetResult, m.createPetErr
}
func (m *hdMockStore) GetPet(_ context.Context, id int) (*Pet, error) {
	m.getPetCalls = append(m.getPetCalls, id)
	return m.getPetResult, m.getPetErr
}
func (m *hdMockStore) GetPetByName(_ context.Context, name string) (*Pet, error) {
	return m.getPetByNameResult, m.getPetByNameErr
}
func (m *hdMockStore) UpdatePet(_ context.Context, pet *Pet) (*Pet, error) {
	m.updatePetCalls = append(m.updatePetCalls, pet)
	return m.updatePetResult, m.updatePetErr
}
func (m *hdMockStore) CreatePetPicture(_ context.Context, id string, fileExt string, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error) {
	return m.createPetPictureResult, m.createPetPictureErr
}
func (m *hdMockStore) GetRandPetPictureByName(_ context.Context, name string) (*PetPicture, error) {
	m.getRandPetPictureByNameCalls = append(m.getRandPetPictureByNameCalls, name)
	return m.getRandPetPictureByNameResult, m.getRandPetPictureByNameErr
}
func (m *hdMockStore) GetPetPicture(_ context.Context, id string) (*PetPicture, error) {
	m.getPetPictureCalls = append(m.getPetPictureCalls, id)
	return m.getPetPictureResult, m.getPetPictureErr
}
func (m *hdMockStore) UpdatePetPicture(_ context.Context, picture PetPicture) (*PetPicture, error) {
	m.updatePetPictureCalls = append(m.updatePetPictureCalls, picture)
	return m.updatePetPictureResult, m.updatePetPictureErr
}
func (m *hdMockStore) DeletePetPicture(_ context.Context, id string) (*PetPicture, error) {
	m.deletePetPictureCalls = append(m.deletePetPictureCalls, id)
	return m.deletePetPictureResult, m.deletePetPictureErr
}

func hdCtxWithSession(session session.Session) context.Context {
	return context.WithValue(testutil.LogContext(context.Background()), reqctx.SessionKey, &session)
}

// hdSessionWithPermissions builds a session whose Permissions are exactly the given strings.
func hdSessionWithPermissions(permissions ...string) session.Session {
	return session.Session{Permissions: permissions}
}

func TestHD01to05_CreatePetHandler(t *testing.T) {
	t.Run("PPH-01_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{createPetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(session.Session{}))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		testutil.ExpectDetail(t, w, msgNoPermissionToCreatePet)
		if len(mock.createPetCalls) != 0 {
			t.Errorf("CreatePet called %d times, want 0", len(mock.createPetCalls))
		}
	})

	t.Run("PPH-02_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{createPetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.admin")))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
		}
		if len(mock.createPetCalls) != 1 || mock.createPetCalls[0] != "Rex" {
			t.Errorf("CreatePet calls = %v, want [Rex]", mock.createPetCalls)
		}
		body := testutil.DecodeJSON(t, w.Body.Bytes())
		if body["name"] != "Rex" {
			t.Errorf("response name = %v, want Rex", body["name"])
		}
	})

	t.Run("PPH-03_NameFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{createPetResult: &Pet{ID: 2, Name: "Fido"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPost, "/pets", strings.NewReader(`{"name":"Fido"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.admin")))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
		}
		if len(mock.createPetCalls) != 1 || mock.createPetCalls[0] != "Fido" {
			t.Errorf("CreatePet calls = %v, want [Fido]", mock.createPetCalls)
		}
	})

	t.Run("PPH-04_NoNameBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPost, "/pets", nil)
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.admin")))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetNameIsRequired)
		if len(mock.createPetCalls) != 0 {
			t.Errorf("CreatePet called %d times, want 0", len(mock.createPetCalls))
		}
	})

	t.Run("PPH-05_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{createPetErr: testutil.ErrDBDown}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.admin")))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToCreatePet)
	})
}

func TestHD06to10_GetPetHandler(t *testing.T) {
	t.Run("PPH-06_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{getPetResult: &Pet{ID: 7, Name: "Rex"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/7", nil)
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

	t.Run("PPH-07_IDFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{getPetResult: &Pet{ID: 7, Name: "Rex"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets", strings.NewReader(`{"id":7}`))
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getPetCalls) != 1 || mock.getPetCalls[0] != 7 {
			t.Errorf("GetPet calls = %v, want [7]", mock.getPetCalls)
		}
	})

	t.Run("PPH-08_NoIDBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets", nil)
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetIDIsRequired)
		if len(mock.getPetCalls) != 0 {
			t.Errorf("GetPet called %d times, want 0", len(mock.getPetCalls))
		}
	})

	t.Run("PPH-09_NonNumericPathValueNoBodyFallback", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/abc", nil)
		req.SetPathValue("id", "abc")
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetIDIsRequired)
		if len(mock.getPetCalls) != 0 {
			t.Errorf("GetPet called %d times, want 0", len(mock.getPetCalls))
		}
	})

	t.Run("PPH-10_StoreErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: ErrPetNotFound}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/7", nil)
		req.SetPathValue("id", "7")
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetNotFound)
	})
}

func TestHD11to14and35_UpdatePetHandler(t *testing.T) {
	t.Run("PPH-11_InvalidBodyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pets", strings.NewReader(`not json`))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgUnableToParseBody)
		if len(mock.updatePetCalls) != 0 {
			t.Errorf("UpdatePet called %d times, want 0", len(mock.updatePetCalls))
		}
	})

	t.Run("PPH-12_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:SomeoneElsesPet")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdatePet)
		if len(mock.updatePetCalls) != 0 {
			t.Errorf("UpdatePet called %d times, want 0", len(mock.updatePetCalls))
		}
	})

	t.Run("PPH-13_HappyPath", func(t *testing.T) {
		mock := &hdMockStore{updatePetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.updatePetCalls) != 1 || mock.updatePetCalls[0].Name != "Rex" {
			t.Errorf("UpdatePet calls = %+v, want a single call with Name=Rex", mock.updatePetCalls)
		}
	})

	t.Run("PPH-14_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{updatePetErr: testutil.ErrDBDown}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToUpdatePet)
	})

	t.Run("PPH-35_StoreNoRowsNotFound", func(t *testing.T) {
		mock := &hdMockStore{updatePetErr: ErrPetNotFound}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetNotFound)
	})
}

func TestHD15to18_GetRandPetPictureByNameHandler(t *testing.T) {
	t.Run("PPH-15_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameResult: &PetPicture{ID: "abc"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
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

	t.Run("PPH-16_NameFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameResult: &PetPicture{ID: "abc"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/picture", strings.NewReader(`{"name":"Rex"}`))
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getRandPetPictureByNameCalls) != 1 || mock.getRandPetPictureByNameCalls[0] != "Rex" {
			t.Errorf("GetRandPetPictureByName calls = %v, want [Rex]", mock.getRandPetPictureByNameCalls)
		}
	})

	t.Run("PPH-17_NoNameBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/picture", nil)
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetNameIsRequired)
	})

	t.Run("PPH-18_StoreErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameErr: ErrPetPictureNotFound}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
		req.SetPathValue("name", "Rex")
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetPictureNotFound)
	})
}

func TestHD19to22_GetPetPictureHandler(t *testing.T) {
	t.Run("PPH-19_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureResult: &PetPicture{ID: "abc123"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pictures/abc123", nil)
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

	t.Run("PPH-20_IDFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureResult: &PetPicture{ID: "abc123"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pictures", strings.NewReader(`{"id":"abc123"}`))
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.getPetPictureCalls) != 1 || mock.getPetPictureCalls[0] != "abc123" {
			t.Errorf("GetPetPicture calls = %v, want [abc123]", mock.getPetPictureCalls)
		}
	})

	t.Run("PPH-21_NoIDBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pictures", nil)
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetPictureIDIsRequired)
	})

	t.Run("PPH-22_StoreErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: ErrPetPictureNotFound}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetPictureNotFound)
	})
}

func TestHD23to27and36_UpdatePetPictureHandler(t *testing.T) {
	t.Run("PPH-23_InvalidBodyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`not json`))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgUnableToParseBody)
		if len(mock.getPetCalls) != 0 || len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("expected no store calls, got GetPet=%v UpdatePetPicture=%v", mock.getPetCalls, mock.updatePetPictureCalls)
		}
	})

	t.Run("PPH-24_PetNotFoundNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: ErrPetNotFound}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetNotFound)
		if len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("UpdatePetPicture called %d times, want 0", len(mock.updatePetPictureCalls))
		}
	})

	t.Run("PPH-25_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{getPetResult: &Pet{ID: 1, Name: "Rex"}}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:SomeoneElse")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdatePet)
		if len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("UpdatePetPicture called %d times, want 0", len(mock.updatePetPictureCalls))
		}
	})

	t.Run("PPH-26_HappyPath", func(t *testing.T) {
		mock := &hdMockStore{
			getPetResult:           &Pet{ID: 1, Name: "Rex"},
			updatePetPictureResult: &PetPicture{ID: "abc123"},
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if len(mock.updatePetPictureCalls) != 1 || mock.updatePetPictureCalls[0].ID != "abc123" {
			t.Errorf("UpdatePetPicture calls = %+v, want a single call with ID=abc123", mock.updatePetPictureCalls)
		}
	})

	t.Run("PPH-27_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
			updatePetPictureErr: testutil.ErrDBDown,
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToUpdatePetPicture)
	})

	t.Run("PPH-36_StoreNoRowsNotFound", func(t *testing.T) {
		mock := &hdMockStore{
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
			updatePetPictureErr: ErrPetPictureNotFound,
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetPictureNotFound)
	})
}

func TestHD28to34_DeletePetPictureHandler(t *testing.T) {
	t.Run("PPH-28_HappyPathFromPathValue", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if len(mock.deletePetPictureCalls) != 1 || mock.deletePetPictureCalls[0] != "abc123" {
			t.Errorf("DeletePetPicture calls = %v, want [abc123]", mock.deletePetPictureCalls)
		}
	})

	t.Run("PPH-29_IDFallsBackToBody", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures", strings.NewReader(`{"id":"abc123"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if len(mock.getPetPictureCalls) != 1 || mock.getPetPictureCalls[0] != "abc123" {
			t.Errorf("GetPetPicture calls = %v, want [abc123]", mock.getPetPictureCalls)
		}
	})

	t.Run("PPH-30_NoIDBadRequest", func(t *testing.T) {
		mock := &hdMockStore{}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures", nil)
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetPictureIDIsRequired)
	})

	t.Run("PPH-31_GetPetPictureErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: ErrPetPictureNotFound}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetPictureNotFound)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("PPH-32_GetPetErrorNotFound", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetErr:           ErrPetNotFound,
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetNotFound)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("PPH-33_NoPermissionForbidden", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:SomeoneElse")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdatePet)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("PPH-34_StoreErrorInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetResult:        &Pet{ID: 1, Name: "Rex"},
			deletePetPictureErr: testutil.ErrDBDown,
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToDeletePetPicture)
	})
}

func TestHD37to44_StoreFailureMapping(t *testing.T) {
	t.Run("PPH-37_CreatePetNameEmptyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{createPetErr: ErrPetNameEmpty}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPost, "/pets/Rex", nil)
		req.SetPathValue("name", "Rex")
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.admin")))
		w := httptest.NewRecorder()

		CreatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetNameMustNotBeEmpty)
	})

	t.Run("PPH-38_UpdatePetNameEmptyBadRequest", func(t *testing.T) {
		mock := &hdMockStore{updatePetErr: ErrPetNameEmpty}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pets", strings.NewReader(`{"id":1,"name":"Rex"}`))
		req = req.WithContext(hdCtxWithSession(hdSessionWithPermissions("petpictures.pets:Rex")))
		w := httptest.NewRecorder()

		UpdatePetHandler(svc)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		testutil.ExpectDetail(t, w, msgPetNameMustNotBeEmpty)
	})

	t.Run("PPH-39_GetPetStoreFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: testutil.ErrDBDown}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/7", nil)
		req.SetPathValue("id", "7")
		w := httptest.NewRecorder()

		GetPetHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToGetPet)
	})

	t.Run("PPH-45_GetRandPetPicturePetNotFound", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameErr: ErrPetNotFound}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
		req.SetPathValue("name", "Rex")
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		testutil.ExpectDetail(t, w, msgPetNotFound)
	})

	t.Run("PPH-40_GetRandPetPictureStoreFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getRandPetPictureByNameErr: testutil.ErrDBDown}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pets/Rex/picture", nil)
		req.SetPathValue("name", "Rex")
		w := httptest.NewRecorder()

		GetRandPetPictureByNameHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToGetRandomPetPicture)
	})

	t.Run("PPH-41_GetPetPictureStoreFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: testutil.ErrDBDown}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodGet, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		GetPetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToGetPetPicture)
	})

	t.Run("PPH-42_UpdatePetPictureGetPetFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetErr: testutil.ErrDBDown}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodPut, "/pictures", strings.NewReader(`{"id":"abc123","prime_subj":1}`))
		w := httptest.NewRecorder()

		UpdatePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToGetPet)
		if len(mock.updatePetPictureCalls) != 0 {
			t.Errorf("UpdatePetPicture called %d times, want 0", len(mock.updatePetPictureCalls))
		}
	})

	t.Run("PPH-43_DeletePetPictureGetPetPictureFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{getPetPictureErr: testutil.ErrDBDown}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToGetPetPicture)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})

	t.Run("PPH-44_DeletePetPictureGetPetFailureInternalServerError", func(t *testing.T) {
		mock := &hdMockStore{
			getPetPictureResult: &PetPicture{ID: "abc123", PrimarySubject: 1},
			getPetErr:           testutil.ErrDBDown,
		}
		svc := NewService(mock)
		req := testutil.NewRequest(http.MethodDelete, "/pictures/abc123", nil)
		req.SetPathValue("id", "abc123")
		w := httptest.NewRecorder()

		DeletePetPictureHandler(svc)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		testutil.ExpectDetail(t, w, msgUnableToGetPet)
		if len(mock.deletePetPictureCalls) != 0 {
			t.Errorf("DeletePetPicture called %d times, want 0", len(mock.deletePetPictureCalls))
		}
	})
}
