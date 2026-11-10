package datastore

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

type fakeDSService struct {
	err error
}

var _ DSService = (*fakeDSService)(nil)

func (f *fakeDSService) Create(_ context.Context, s *Store) (*Store, error) { return s, f.err }
func (f *fakeDSService) Read(_ context.Context, s *Store) (*Store, error)   { return s, f.err }
func (f *fakeDSService) Update(_ context.Context, s *Store) (*Store, error) { return s, f.err }
func (f *fakeDSService) Delete(_ context.Context, _ *Store) error           { return f.err }

func dsRequest() *http.Request {
	r := testutil.NewRequest(http.MethodPost, "/", strings.NewReader(`{"store_id":"1","owner_id":"2"}`))
	sess := &session.Session{ID: "s1", UserID: "2", Permissions: []string{"datastore.admin"}}
	return r.WithContext(context.WithValue(r.Context(), reqctx.SessionKey, sess))
}

func runDSHandler(t *testing.T, h http.HandlerFunc, wantStatus int) {
	t.Helper()
	w := httptest.NewRecorder()
	h(w, dsRequest())
	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, wantStatus, w.Body.String())
	}
}

func TestDSH_DataStoreHandlers(t *testing.T) {
	t.Run("DSH-01_ReadNotFound", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: ErrDataStoreNotFound}).ReadDataStore, http.StatusNotFound)
	})
	t.Run("DSH-02_ReadUnexpectedError", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: testutil.ErrBoom}).ReadDataStore, http.StatusInternalServerError)
	})
	t.Run("DSH-03_ReadSuccess", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{}).ReadDataStore, http.StatusOK)
	})
	t.Run("DSH-04_UpdateNotFound", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: ErrDataStoreNotFound}).UpdateDataStore, http.StatusNotFound)
	})
	t.Run("DSH-05_UpdateOwnerNotFound", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: ErrDataStoreOwnerNotFound}).UpdateDataStore, http.StatusNotFound)
	})
	t.Run("DSH-06_UpdateUnexpectedError", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: testutil.ErrBoom}).UpdateDataStore, http.StatusInternalServerError)
	})
	t.Run("DSH-07_DeleteNotFound", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: ErrDataStoreNotFound}).DeleteDataStore, http.StatusNotFound)
	})
	t.Run("DSH-08_DeleteInUse", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: ErrDataStoreInUse}).DeleteDataStore, http.StatusConflict)
	})
	t.Run("DSH-09_DeleteUnexpectedError", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{err: testutil.ErrBoom}).DeleteDataStore, http.StatusInternalServerError)
	})
	t.Run("DSH-10_DeleteSuccess", func(t *testing.T) {
		runDSHandler(t, NewHandler(&fakeDSService{}).DeleteDataStore, http.StatusNoContent)
	})
	t.Run("DSH-11_InvalidIDIs400", func(t *testing.T) {
		svc := &fakeDSService{err: ErrDataStoreInvalidID}
		runDSHandler(t, NewHandler(svc).ReadDataStore, http.StatusBadRequest)
		runDSHandler(t, NewHandler(svc).UpdateDataStore, http.StatusBadRequest)
		runDSHandler(t, NewHandler(svc).DeleteDataStore, http.StatusBadRequest)
	})
}
