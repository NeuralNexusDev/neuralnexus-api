package numbersds

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

type ndhService struct {
	err error
}

func (s *ndhService) Add(context.Context, *NumberData, float64) error { return s.err }
func (s *ndhService) Create(_ context.Context, n *NumberData) (*NumberData, error) {
	return n, s.err
}
func (s *ndhService) Read(_ context.Context, n *NumberData) (*NumberData, error) {
	return n, s.err
}
func (s *ndhService) Update(_ context.Context, n *NumberData) (*NumberData, error) {
	return n, s.err
}
func (s *ndhService) Delete(context.Context, *NumberData) error { return s.err }

func ndhServe(t *testing.T, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	r := testutil.NewRequest(http.MethodPost, "/datastores/numbers", strings.NewReader(`{"store_id":"1","user_id":"2","value":3}`))
	sess := &session.Session{ID: "s1", UserID: "u1", Permissions: []string{"numberstore.admin"}}
	r = r.WithContext(context.WithValue(r.Context(), reqctx.SessionKey, sess))
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func TestNumberHandlerErrors(t *testing.T) {
	t.Run("NDH-01_CreateExists", func(t *testing.T) {
		w := ndhServe(t, CreateNumberHandler(&ndhService{err: ErrNumberExists}))
		testutil.ExpectStatus(t, w, http.StatusConflict)
		testutil.ExpectDetail(t, w, "A number already exists for this data store")
	})

	t.Run("NDH-02_CreateStoreOrUserNotFound", func(t *testing.T) {
		w := ndhServe(t, CreateNumberHandler(&ndhService{err: ErrNumberStoreOrUserNotFound}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Data store or user not found")
	})

	t.Run("NDH-03_CreateUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, CreateNumberHandler(&ndhService{err: testutil.ErrBoom}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to create numberstore")
	})

	t.Run("NDH-04_ReadNotFound", func(t *testing.T) {
		w := ndhServe(t, ReadNumberHandler(&ndhService{err: ErrNumberNotFound}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Number not found")
	})

	t.Run("NDH-05_ReadUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, ReadNumberHandler(&ndhService{err: testutil.ErrBoom}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to read numberstore")
	})

	t.Run("NDH-06_UpdateNotFound", func(t *testing.T) {
		w := ndhServe(t, UpdateNumberHandler(&ndhService{err: ErrNumberNotFound}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Number not found")
	})

	t.Run("NDH-07_UpdateUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, UpdateNumberHandler(&ndhService{err: testutil.ErrBoom}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to update numberstore")
	})

	t.Run("NDH-08_DeleteNotFound", func(t *testing.T) {
		w := ndhServe(t, DeleteNumberHandler(&ndhService{err: ErrNumberNotFound}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Number not found")
	})

	t.Run("NDH-09_DeleteUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, DeleteNumberHandler(&ndhService{err: testutil.ErrBoom}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to delete numberstore")
	})

	t.Run("NDH-10_InvalidIDsAre400", func(t *testing.T) {
		handlers := map[string]func(NumberService) http.HandlerFunc{
			"Create": CreateNumberHandler,
			"Read":   ReadNumberHandler,
			"Update": UpdateNumberHandler,
			"Delete": DeleteNumberHandler,
		}
		for name, handler := range handlers {
			t.Run(name, func(t *testing.T) {
				w := ndhServe(t, handler(&ndhService{err: ErrNumberInvalidID}))
				testutil.ExpectStatus(t, w, http.StatusBadRequest)
				testutil.ExpectDetail(t, w, "Data store and user ids must be numbers")
			})
		}
	})
}
