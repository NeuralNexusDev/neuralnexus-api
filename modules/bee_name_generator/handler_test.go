package beenamegenerator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/bngpb"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

type bngMockStore struct {
	getBeeName              func() (string, error)
	uploadBeeName           func(beeName string) (string, error)
	deleteBeeName           func(beeName string) (string, error)
	submitBeeName           func(beeName string) (string, error)
	getBeeNameSuggestions   func(amount int64) ([]string, error)
	acceptBeeNameSuggestion func(beeName string) (string, error)
	rejectBeeNameSuggestion func(beeName string) (string, error)
}

var _ BNGStore = (*bngMockStore)(nil)

func (m *bngMockStore) GetBeeName() (string, error) { return m.getBeeName() }
func (m *bngMockStore) UploadBeeName(beeName string) (string, error) {
	return m.uploadBeeName(beeName)
}
func (m *bngMockStore) DeleteBeeName(beeName string) (string, error) {
	return m.deleteBeeName(beeName)
}
func (m *bngMockStore) SubmitBeeName(beeName string) (string, error) {
	return m.submitBeeName(beeName)
}
func (m *bngMockStore) GetBeeNameSuggestions(amount int64) ([]string, error) {
	return m.getBeeNameSuggestions(amount)
}
func (m *bngMockStore) AcceptBeeNameSuggestion(beeName string) (string, error) {
	return m.acceptBeeNameSuggestion(beeName)
}
func (m *bngMockStore) RejectBeeNameSuggestion(beeName string) (string, error) {
	return m.rejectBeeNameSuggestion(beeName)
}

var (
	bngAuthorizedSession   = &auth.Session{ID: "s1", UserID: "u1", Permissions: []string{"beenamegenerator:*"}}
	bngUnauthorizedSession = &auth.Session{ID: "s2", UserID: "u2", Permissions: []string{}}
)

func bngRequest(t *testing.T, method, target string, pathValues map[string]string, session *auth.Session) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	for k, v := range pathValues {
		r.SetPathValue(k, v)
	}
	if session != nil {
		r = r.WithContext(context.WithValue(r.Context(), mw.SessionKey, session))
	}
	return r
}

func bngRequireDetail(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", w.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func bngRequireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, want, w.Body.String())
	}
}

func TestHD01to02_GetBeeNameHandler(t *testing.T) {
	t.Run("HD-01_OK", func(t *testing.T) {
		s := &bngMockStore{getBeeName: func() (string, error) { return "Buzzy", nil }}
		h := GetBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/", nil, nil))
		bngRequireStatus(t, w, http.StatusOK)
		var got bngpb.BeeName
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Name != "Buzzy" {
			t.Errorf("body = %s, want a BeeName named Buzzy", w.Body.String())
		}
	})

	t.Run("HD-02_StoreError", func(t *testing.T) {
		s := &bngMockStore{getBeeName: func() (string, error) { return "", testerrors.ErrBoom }}
		h := GetBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/", nil, nil))
		bngRequireStatus(t, w, http.StatusInternalServerError)
		bngRequireDetail(t, w, msgFailedToGetBeeName)
	})
}

func TestHD03to06_UploadBeeNameHandler(t *testing.T) {
	t.Run("HD-03_Forbidden", func(t *testing.T) {
		h := UploadBeeNameHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPut, "/Buzzy", map[string]string{"name": "Buzzy"}, bngUnauthorizedSession))
		bngRequireStatus(t, w, http.StatusForbidden)
		bngRequireDetail(t, w, msgNoPermissionToUploadBeeNames)
	})

	t.Run("HD-04_EmptyName", func(t *testing.T) {
		h := UploadBeeNameHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPut, "/", map[string]string{"name": ""}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidName)
	})

	t.Run("HD-05_OK", func(t *testing.T) {
		s := &bngMockStore{uploadBeeName: func(name string) (string, error) { return name, nil }}
		h := UploadBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPut, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusOK)
		var got bngpb.BeeName
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Name != "Buzzy" {
			t.Errorf("body = %s, want a BeeName named Buzzy", w.Body.String())
		}
	})

	t.Run("HD-06_StoreError", func(t *testing.T) {
		s := &bngMockStore{uploadBeeName: func(string) (string, error) { return "", testerrors.ErrBoom }}
		h := UploadBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPut, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusInternalServerError)
		bngRequireDetail(t, w, msgFailedToUploadBeeName)
	})
}

func TestHD07to10_DeleteBeeNameHandler(t *testing.T) {
	t.Run("HD-07_Forbidden", func(t *testing.T) {
		h := DeleteBeeNameHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodDelete, "/Buzzy", map[string]string{"name": "Buzzy"}, bngUnauthorizedSession))
		bngRequireStatus(t, w, http.StatusForbidden)
		bngRequireDetail(t, w, msgNoPermissionToDeleteBeeNames)
	})

	t.Run("HD-08_EmptyName", func(t *testing.T) {
		h := DeleteBeeNameHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodDelete, "/", map[string]string{"name": ""}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidName)
	})

	t.Run("HD-09_OK", func(t *testing.T) {
		s := &bngMockStore{deleteBeeName: func(name string) (string, error) { return name, nil }}
		h := DeleteBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodDelete, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-10_StoreError", func(t *testing.T) {
		s := &bngMockStore{deleteBeeName: func(string) (string, error) { return "", testerrors.ErrBoom }}
		h := DeleteBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodDelete, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusInternalServerError)
		bngRequireDetail(t, w, msgFailedToDeleteBeeName)
	})
}

func TestHD11to13_SubmitBeeNameHandler(t *testing.T) {
	t.Run("HD-11_EmptyName", func(t *testing.T) {
		h := SubmitBeeNameHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/", map[string]string{"name": ""}, nil))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidName)
	})

	t.Run("HD-12_OK", func(t *testing.T) {
		s := &bngMockStore{submitBeeName: func(name string) (string, error) { return name, nil }}
		h := SubmitBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, nil))
		bngRequireStatus(t, w, http.StatusOK)
		var got bngpb.BeeName
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Name != "Buzzy" {
			t.Errorf("body = %s, want a BeeName named Buzzy", w.Body.String())
		}
	})

	t.Run("HD-13_StoreError", func(t *testing.T) {
		s := &bngMockStore{submitBeeName: func(string) (string, error) { return "", testerrors.ErrBoom }}
		h := SubmitBeeNameHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, nil))
		bngRequireStatus(t, w, http.StatusInternalServerError)
		bngRequireDetail(t, w, msgFailedToSubmitBeeName)
	})
}

func TestHD14to19_GetBeeNameSuggestionsHandler(t *testing.T) {
	t.Run("HD-14_Forbidden", func(t *testing.T) {
		h := GetBeeNameSuggestionsHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/5", map[string]string{"amount": "5"}, bngUnauthorizedSession))
		bngRequireStatus(t, w, http.StatusForbidden)
		bngRequireDetail(t, w, msgNoPermissionToGetBeeNameSuggestions)
	})

	t.Run("HD-15_EmptyAmount", func(t *testing.T) {
		h := GetBeeNameSuggestionsHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/", map[string]string{"amount": ""}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidAmountProvided)
	})

	t.Run("HD-16_ZeroAmount", func(t *testing.T) {
		h := GetBeeNameSuggestionsHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/0", map[string]string{"amount": "0"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidAmountProvided)
	})

	t.Run("HD-17_NonNumericAmount", func(t *testing.T) {
		h := GetBeeNameSuggestionsHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/abc", map[string]string{"amount": "abc"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidAmountProvided)
	})

	t.Run("HD-18_OK", func(t *testing.T) {
		var gotAmount int64
		s := &bngMockStore{getBeeNameSuggestions: func(amount int64) ([]string, error) {
			gotAmount = amount
			return []string{"a", "b"}, nil
		}}
		h := GetBeeNameSuggestionsHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/5", map[string]string{"amount": "5"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusOK)
		if gotAmount != 5 {
			t.Errorf("amount passed to store = %d, want 5", gotAmount)
		}
		var got bngpb.BeeNameSuggestions
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got.Suggestions) != 2 {
			t.Errorf("body = %s, want 2 suggestions", w.Body.String())
		}
	})

	t.Run("HD-19_StoreError", func(t *testing.T) {
		s := &bngMockStore{getBeeNameSuggestions: func(int64) ([]string, error) { return nil, testerrors.ErrBoom }}
		h := GetBeeNameSuggestionsHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodGet, "/5", map[string]string{"amount": "5"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusInternalServerError)
		bngRequireDetail(t, w, msgFailedToGetBeeNameSuggestions)
	})
}

func TestHD20to23_AcceptBeeNameSuggestionHandler(t *testing.T) {
	t.Run("HD-20_Forbidden", func(t *testing.T) {
		h := AcceptBeeNameSuggestionHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, bngUnauthorizedSession))
		bngRequireStatus(t, w, http.StatusForbidden)
		bngRequireDetail(t, w, msgNoPermissionToAcceptBeeNameSuggestions)
	})

	t.Run("HD-21_EmptyName", func(t *testing.T) {
		h := AcceptBeeNameSuggestionHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/", map[string]string{"name": ""}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidName)
	})

	t.Run("HD-22_OK", func(t *testing.T) {
		s := &bngMockStore{acceptBeeNameSuggestion: func(name string) (string, error) { return name, nil }}
		h := AcceptBeeNameSuggestionHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusOK)
		var got bngpb.BeeName
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Name != "Buzzy" {
			t.Errorf("body = %s, want a BeeName named Buzzy", w.Body.String())
		}
	})

	t.Run("HD-23_StoreError", func(t *testing.T) {
		s := &bngMockStore{acceptBeeNameSuggestion: func(string) (string, error) { return "", testerrors.ErrBoom }}
		h := AcceptBeeNameSuggestionHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusInternalServerError)
		bngRequireDetail(t, w, msgFailedToAcceptBeeNameSuggestion)
	})
}

func TestHD24to27_RejectBeeNameSuggestionHandler(t *testing.T) {
	t.Run("HD-24_Forbidden", func(t *testing.T) {
		h := RejectBeeNameSuggestionHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, bngUnauthorizedSession))
		bngRequireStatus(t, w, http.StatusForbidden)
		bngRequireDetail(t, w, msgNoPermissionToRejectBeeNameSuggestions)
	})

	t.Run("HD-25_EmptyName", func(t *testing.T) {
		h := RejectBeeNameSuggestionHandler(&bngMockStore{})
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/", map[string]string{"name": ""}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusBadRequest)
		bngRequireDetail(t, w, msgInvalidName)
	})

	t.Run("HD-26_OK", func(t *testing.T) {
		s := &bngMockStore{rejectBeeNameSuggestion: func(name string) (string, error) { return name, nil }}
		h := RejectBeeNameSuggestionHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-27_StoreError", func(t *testing.T) {
		s := &bngMockStore{rejectBeeNameSuggestion: func(string) (string, error) { return "", testerrors.ErrBoom }}
		h := RejectBeeNameSuggestionHandler(s)
		w := httptest.NewRecorder()
		h(w, bngRequest(t, http.MethodPost, "/Buzzy", map[string]string{"name": "Buzzy"}, bngAuthorizedSession))
		bngRequireStatus(t, w, http.StatusInternalServerError)
		bngRequireDetail(t, w, msgFailedToRejectBeeNameSuggestion)
	})
}
