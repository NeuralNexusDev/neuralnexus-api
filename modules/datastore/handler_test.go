package datastore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
)

type hdStubService struct {
	deleteErr   error
	deleteCalls []*Store
}

var _ DSService = (*hdStubService)(nil)

func (s *hdStubService) Create(*Store) (*Store, error) { return nil, nil }
func (s *hdStubService) Read(*Store) (*Store, error)   { return nil, nil }
func (s *hdStubService) Update(*Store) (*Store, error) { return nil, nil }
func (s *hdStubService) Delete(ds *Store) error {
	s.deleteCalls = append(s.deleteCalls, ds)
	return s.deleteErr
}

func hdDeleteRequest(body string, session *auth.Session) *http.Request {
	r := httptest.NewRequest(http.MethodDelete, "/datastore", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r.WithContext(context.WithValue(context.Background(), mw.SessionKey, session))
}

func hdAdminSession() *auth.Session {
	return &auth.Session{Permissions: []string{perms.ScopeAdminDataStore.Name + "|" + perms.ScopeAdminDataStore.Value}}
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

func TestHD01to04_DeleteDataStoreHandler(t *testing.T) {
	t.Run("HD-01_NoPermissionForbidden", func(t *testing.T) {
		svc := &hdStubService{}
		w := httptest.NewRecorder()

		DeleteDataStoreHandler(svc)(w, hdDeleteRequest(`{"store_id":"s1"}`, &auth.Session{}))

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		hdRequireDetail(t, w, msgNoPermissionToDeleteDatastore)
		if len(svc.deleteCalls) != 0 {
			t.Errorf("Delete called %d times, want 0", len(svc.deleteCalls))
		}
	})

	t.Run("HD-02_MalformedBodyBadRequest", func(t *testing.T) {
		svc := &hdStubService{}
		w := httptest.NewRecorder()

		DeleteDataStoreHandler(svc)(w, hdDeleteRequest(`not json`, hdAdminSession()))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if len(svc.deleteCalls) != 0 {
			t.Errorf("Delete called %d times, want 0", len(svc.deleteCalls))
		}
	})

	t.Run("HD-03_ServiceErrorInternalServerError", func(t *testing.T) {
		svc := &hdStubService{deleteErr: testerrors.ErrDBDown}
		w := httptest.NewRecorder()

		DeleteDataStoreHandler(svc)(w, hdDeleteRequest(`{"store_id":"s1"}`, hdAdminSession()))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
		hdRequireDetail(t, w, msgFailedToDeleteDatastore)
	})

	t.Run("HD-04_SuccessNoContent", func(t *testing.T) {
		svc := &hdStubService{}
		w := httptest.NewRecorder()

		DeleteDataStoreHandler(svc)(w, hdDeleteRequest(`{"store_id":"s1"}`, hdAdminSession()))

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if len(svc.deleteCalls) != 1 || svc.deleteCalls[0].StoreID != "s1" {
			t.Errorf("Delete calls = %+v, want one call with store_id s1", svc.deleteCalls)
		}
	})
}
