package datastore

import (
	"context"
	"errors"
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
	errBoom := errors.New("boom")

	t.Run("DSH-01_ReadNotFound", func(t *testing.T) {
		runDSHandler(t, ReadDataStoreHandler(&fakeDSService{err: ErrDataStoreNotFound}), http.StatusNotFound)
	})
	t.Run("DSH-02_ReadUnexpectedError", func(t *testing.T) {
		runDSHandler(t, ReadDataStoreHandler(&fakeDSService{err: errBoom}), http.StatusInternalServerError)
	})
	t.Run("DSH-03_ReadSuccess", func(t *testing.T) {
		runDSHandler(t, ReadDataStoreHandler(&fakeDSService{}), http.StatusOK)
	})
	t.Run("DSH-04_UpdateNotFound", func(t *testing.T) {
		runDSHandler(t, UpdateDataStoreHandler(&fakeDSService{err: ErrDataStoreNotFound}), http.StatusNotFound)
	})
	t.Run("DSH-05_UpdateOwnerNotFound", func(t *testing.T) {
		runDSHandler(t, UpdateDataStoreHandler(&fakeDSService{err: ErrDataStoreOwnerNotFound}), http.StatusNotFound)
	})
	t.Run("DSH-06_UpdateUnexpectedError", func(t *testing.T) {
		runDSHandler(t, UpdateDataStoreHandler(&fakeDSService{err: errBoom}), http.StatusInternalServerError)
	})
	t.Run("DSH-07_DeleteNotFound", func(t *testing.T) {
		runDSHandler(t, DeleteDataStoreHandler(&fakeDSService{err: ErrDataStoreNotFound}), http.StatusNotFound)
	})
	t.Run("DSH-08_DeleteInUse", func(t *testing.T) {
		runDSHandler(t, DeleteDataStoreHandler(&fakeDSService{err: ErrDataStoreInUse}), http.StatusConflict)
	})
	t.Run("DSH-09_DeleteUnexpectedError", func(t *testing.T) {
		runDSHandler(t, DeleteDataStoreHandler(&fakeDSService{err: errBoom}), http.StatusInternalServerError)
	})
	t.Run("DSH-10_DeleteSuccess", func(t *testing.T) {
		runDSHandler(t, DeleteDataStoreHandler(&fakeDSService{}), http.StatusNoContent)
	})
	t.Run("DSH-11_InvalidIDIs400", func(t *testing.T) {
		svc := &fakeDSService{err: ErrDataStoreInvalidID}
		runDSHandler(t, ReadDataStoreHandler(svc), http.StatusBadRequest)
		runDSHandler(t, UpdateDataStoreHandler(svc), http.StatusBadRequest)
		runDSHandler(t, DeleteDataStoreHandler(svc), http.StatusBadRequest)
	})
}
