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

const ndhValidBody = "{\"store_id\":\"1\",\"user_id\":\"2\",\"value\":3}"

func ndhServeAs(t *testing.T, h http.HandlerFunc, permissions []string, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := testutil.NewRequest(http.MethodPost, "/datastores/numbers", strings.NewReader(body))
	sess := &session.Session{ID: "s1", UserID: "u1", Permissions: permissions}
	r = r.WithContext(context.WithValue(r.Context(), reqctx.SessionKey, sess))
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func ndhServe(t *testing.T, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	return ndhServeAs(t, h, []string{"numberstore.admin"}, ndhValidBody)
}

func ndhRequireEchoedNumber(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	testutil.ExpectStatus(t, w, http.StatusOK)
	var got NumberData
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	if want := (NumberData{StoreID: "1", UserID: "2", Value: 3}); got != want {
		t.Fatalf("number = %+v, want %+v", got, want)
	}
}

func TestNumberHandlerErrors(t *testing.T) {
	t.Run("NDH-01_CreateExists", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: ErrNumberExists}).CreateNumber)
		testutil.ExpectStatus(t, w, http.StatusConflict)
		testutil.ExpectDetail(t, w, "A number already exists for this data store")
	})

	t.Run("NDH-02_CreateStoreOrUserNotFound", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: ErrNumberStoreOrUserNotFound}).CreateNumber)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Data store or user not found")
	})

	t.Run("NDH-03_CreateUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: testutil.ErrBoom}).CreateNumber)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to create numberstore")
	})

	t.Run("NDH-04_ReadNotFound", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: ErrNumberNotFound}).ReadNumber)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Number not found")
	})

	t.Run("NDH-05_ReadUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: testutil.ErrBoom}).ReadNumber)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to read numberstore")
	})

	t.Run("NDH-06_UpdateNotFound", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: ErrNumberNotFound}).UpdateNumber)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Number not found")
	})

	t.Run("NDH-07_UpdateUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: testutil.ErrBoom}).UpdateNumber)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to update numberstore")
	})

	t.Run("NDH-08_DeleteNotFound", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: ErrNumberNotFound}).DeleteNumber)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, "Number not found")
	})

	t.Run("NDH-09_DeleteUnexpectedError", func(t *testing.T) {
		w := ndhServe(t, NewHandler(&ndhService{err: testutil.ErrBoom}).DeleteNumber)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, "Failed to delete numberstore")
	})

	t.Run("NDH-10_InvalidIDsAre400", func(t *testing.T) {
		h := NewHandler(&ndhService{err: ErrNumberInvalidID})
		handlers := map[string]http.HandlerFunc{
			"Create": h.CreateNumber,
			"Read":   h.ReadNumber,
			"Update": h.UpdateNumber,
			"Delete": h.DeleteNumber,
		}
		for name, handler := range handlers {
			t.Run(name, func(t *testing.T) {
				w := ndhServe(t, handler)
				testutil.ExpectStatus(t, w, http.StatusBadRequest)
				testutil.ExpectDetail(t, w, "Data store and user ids must be numbers")
			})
		}
	})
}

func TestNDH11to20_NumberHandlerGaps(t *testing.T) {
	admin := []string{"numberstore.admin"}
	h := NewHandler(&ndhService{})

	t.Run("NDH-11_CreateReturnsNumber", func(t *testing.T) {
		ndhRequireEchoedNumber(t, ndhServe(t, h.CreateNumber))
	})
	t.Run("NDH-12_CreateWithoutPermission", func(t *testing.T) {
		w := ndhServeAs(t, h.CreateNumber, nil, ndhValidBody)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToCreateNumberstore)
	})
	t.Run("NDH-13_CreateBadBody", func(t *testing.T) {
		testutil.ExpectStatus(t, ndhServeAs(t, h.CreateNumber, admin, "{"), http.StatusBadRequest)
	})
	t.Run("NDH-14_ReadReturnsNumber", func(t *testing.T) {
		ndhRequireEchoedNumber(t, ndhServe(t, h.ReadNumber))
	})
	t.Run("NDH-15_ReadBadBody", func(t *testing.T) {
		testutil.ExpectStatus(t, ndhServeAs(t, h.ReadNumber, admin, "{"), http.StatusBadRequest)
	})
	t.Run("NDH-16_UpdateReturnsNumber", func(t *testing.T) {
		ndhRequireEchoedNumber(t, ndhServe(t, h.UpdateNumber))
	})
	t.Run("NDH-17_UpdateWithoutPermission", func(t *testing.T) {
		w := ndhServeAs(t, h.UpdateNumber, nil, ndhValidBody)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdateNumberstore)
	})
	t.Run("NDH-18_UpdateBadBody", func(t *testing.T) {
		testutil.ExpectStatus(t, ndhServeAs(t, h.UpdateNumber, admin, "{"), http.StatusBadRequest)
	})
	t.Run("NDH-19_DeleteReturnsNumber", func(t *testing.T) {
		ndhRequireEchoedNumber(t, ndhServe(t, h.DeleteNumber))
	})
	t.Run("NDH-20_DeleteBadBody", func(t *testing.T) {
		testutil.ExpectStatus(t, ndhServeAs(t, h.DeleteNumber, admin, "{"), http.StatusBadRequest)
	})
}
