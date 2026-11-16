package datastore

import (
	"context"
	"encoding/json"
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

const dsValidBody = "{\"store_id\":\"1\",\"owner_id\":\"2\"}"

func dsRequestAs(permissions []string, body string) *http.Request {
	r := testutil.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	sess := &session.Session{ID: "s1", UserID: "2", Permissions: permissions}
	return r.WithContext(context.WithValue(r.Context(), reqctx.SessionKey, sess))
}

func dsRequest() *http.Request {
	return dsRequestAs([]string{"datastore.admin"}, dsValidBody)
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

func TestDSH12to20_DataStoreHandlerGaps(t *testing.T) {
	admin := []string{"datastore.admin"}
	serve := func(h http.HandlerFunc, permissions []string, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h(w, dsRequestAs(permissions, body))
		return w
	}

	t.Run("DSH-12_CreateOwnedBySessionUser", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).CreateDataStore, admin, "")
		testutil.ExpectStatus(t, w, http.StatusOK)
		var got Store
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode body %q: %v", w.Body.String(), err)
		}
		if got.OwnerID != "2" {
			t.Errorf("OwnerID = %q, want 2", got.OwnerID)
		}
		if got.StoreID == "" {
			t.Error("StoreID is empty")
		}
	})
	t.Run("DSH-13_CreateWithoutPermission", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).CreateDataStore, nil, "")
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToCreateDatastore)
	})
	t.Run("DSH-14_CreateServiceFailure", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{err: testutil.ErrBoom}).CreateDataStore, admin, "")
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToCreateDatastore)
	})
	t.Run("DSH-15_UpdateWithoutPermission", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).UpdateDataStore, nil, dsValidBody)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdateDatastore)
	})
	t.Run("DSH-16_DeleteWithoutPermission", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).DeleteDataStore, nil, dsValidBody)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToDeleteDatastore)
	})
	t.Run("DSH-17_ReadBadBody", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).ReadDataStore, admin, "{")
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
	})
	t.Run("DSH-18_UpdateBadBody", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).UpdateDataStore, admin, "{")
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
	})
	t.Run("DSH-19_DeleteBadBody", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).DeleteDataStore, admin, "{")
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
	})
	t.Run("DSH-20_UpdateReturnsStore", func(t *testing.T) {
		w := serve(NewHandler(&fakeDSService{}).UpdateDataStore, admin, dsValidBody)
		testutil.ExpectStatus(t, w, http.StatusOK)
		var got Store
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode body %q: %v", w.Body.String(), err)
		}
		if got.StoreID != "1" || got.OwnerID != "2" {
			t.Errorf("store = %+v, want store 1 owned by 2", got)
		}
	})
}
