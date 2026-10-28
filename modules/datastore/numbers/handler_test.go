package numbersds

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

func ndhRequireProblem(t *testing.T, w *httptest.ResponseRecorder, status int, detail string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, status, w.Body.String())
	}
	var p struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem: %v (body: %s)", err, w.Body.String())
	}
	if p.Detail != detail {
		t.Fatalf("detail = %q, want %q", p.Detail, detail)
	}
}

func TestNumberHandlerErrors(t *testing.T) {
	t.Run("NDH-01_CreateExists", func(t *testing.T) {
		w := ndhServe(t, CreateNumberHandler(&ndhService{err: ErrNumberExists}))
		ndhRequireProblem(t, w, http.StatusConflict, "A number already exists for this data store")
	})

	t.Run("NDH-02_CreateStoreOrUserNotFound", func(t *testing.T) {
		w := ndhServe(t, CreateNumberHandler(&ndhService{err: ErrNumberStoreOrUserNotFound}))
		ndhRequireProblem(t, w, http.StatusNotFound, "Data store or user not found")
	})

	t.Run("NDH-03_CreateUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, CreateNumberHandler(&ndhService{err: testutil.ErrBoom}))
		ndhRequireProblem(t, w, http.StatusInternalServerError, "Failed to create numberstore")
	})

	t.Run("NDH-04_ReadNotFound", func(t *testing.T) {
		w := ndhServe(t, ReadNumberHandler(&ndhService{err: ErrNumberNotFound}))
		ndhRequireProblem(t, w, http.StatusNotFound, "Number not found")
	})

	t.Run("NDH-05_ReadUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, ReadNumberHandler(&ndhService{err: testutil.ErrBoom}))
		ndhRequireProblem(t, w, http.StatusInternalServerError, "Failed to read numberstore")
	})

	t.Run("NDH-06_UpdateNotFound", func(t *testing.T) {
		w := ndhServe(t, UpdateNumberHandler(&ndhService{err: ErrNumberNotFound}))
		ndhRequireProblem(t, w, http.StatusNotFound, "Number not found")
	})

	t.Run("NDH-07_UpdateUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, UpdateNumberHandler(&ndhService{err: testutil.ErrBoom}))
		ndhRequireProblem(t, w, http.StatusInternalServerError, "Failed to update numberstore")
	})

	t.Run("NDH-08_DeleteNotFound", func(t *testing.T) {
		w := ndhServe(t, DeleteNumberHandler(&ndhService{err: ErrNumberNotFound}))
		ndhRequireProblem(t, w, http.StatusNotFound, "Number not found")
	})

	t.Run("NDH-09_DeleteUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, DeleteNumberHandler(&ndhService{err: testutil.ErrBoom}))
		ndhRequireProblem(t, w, http.StatusInternalServerError, "Failed to delete numberstore")
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
				ndhRequireProblem(t, w, http.StatusBadRequest, "Data store and user ids must be numbers")
			})
		}
	})
}
