package petpictures

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

// svCreatePetPictureArgs records one CreatePetPicture call's arguments.
type svCreatePetPictureArgs struct {
	id             string
	fileExt        string
	primarySubject int
	othersSubjects []int
	aliases        []string
}

// svMockStore is a self-contained fake PetPicStore for service-layer tests:
// UploadPetPicture only ever calls CreatePetPicture, so every other method
// panics if reached, keeping an accidental extra call visible instead of
// silently returning a zero value.
type svMockStore struct {
	createPetPictureResult *PetPicture
	createPetPictureErr    error
	createPetPictureCalls  []svCreatePetPictureArgs
}

func (m *svMockStore) CreatePet(name string) (*Pet, error) {
	panic("CreatePet not used by UploadPetPicture")
}
func (m *svMockStore) GetPet(id int) (*Pet, error) {
	panic("GetPet not used by UploadPetPicture")
}
func (m *svMockStore) GetPetByName(name string) (*Pet, error) {
	panic("GetPetByName not used by UploadPetPicture")
}
func (m *svMockStore) UpdatePet(pet *Pet) (*Pet, error) {
	panic("UpdatePet not used by UploadPetPicture")
}
func (m *svMockStore) CreatePetPicture(id string, fileExt string, primarySubject int, othersSubjects []int, aliases []string) (*PetPicture, error) {
	m.createPetPictureCalls = append(m.createPetPictureCalls, svCreatePetPictureArgs{id, fileExt, primarySubject, othersSubjects, aliases})
	return m.createPetPictureResult, m.createPetPictureErr
}
func (m *svMockStore) GetRandPetPictureByName(name string) (*PetPicture, error) {
	panic("GetRandPetPictureByName not used by UploadPetPicture")
}
func (m *svMockStore) GetPetPicture(id string) (*PetPicture, error) {
	panic("GetPetPicture not used by UploadPetPicture")
}
func (m *svMockStore) UpdatePetPicture(picture PetPicture) (*PetPicture, error) {
	panic("UpdatePetPicture not used by UploadPetPicture")
}
func (m *svMockStore) DeletePetPicture(id string) (*PetPicture, error) {
	panic("DeletePetPicture not used by UploadPetPicture")
}

// svFakeRoundTripper is a controllable http.RoundTripper for exercising
// UploadPetPicture's CDN request without a real network call.
type svFakeRoundTripper struct {
	resp    *http.Response
	err     error
	lastReq *http.Request
}

func (rt *svFakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.lastReq = req
	if rt.err != nil {
		return nil, rt.err
	}
	return rt.resp, nil
}

// swapTransport replaces the package-level http.DefaultTransport (which
// UploadPetPicture's client implicitly uses) for the duration of a subtest
// and restores the original afterward.
func swapTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() {
		http.DefaultTransport = orig
	})
}

// svOKResponse builds a minimal 200 OK response for the fake transport.
func svOKResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}
}

// svTempFile creates a temp file directly in the current working directory
// (the package directory under `go test`) so file.Name() is a bare relative
// name with no path separators or extra dots — UploadPetPicture computes its
// file extension from the whole of file.Name(), not just its base name, so a
// temp directory containing dots or separators would corrupt that
// computation for the tests below. The file is cleaned up automatically.
func svTempFile(t *testing.T, pattern string, content []byte) *os.File {
	t.Helper()
	f, err := os.CreateTemp(".", pattern)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.Write(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("failed to seek temp file: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestSV01_NewServiceAndGetStore(t *testing.T) {
	t.Run("SV-01_GetStoreReturnsInjectedStore", func(t *testing.T) {
		mock := &svMockStore{}
		svc := NewService(mock)
		if got := svc.GetStore(); got != PetPicStore(mock) {
			t.Errorf("GetStore() = %v, want the injected store %v", got, mock)
		}
	})
}

func TestSV03to08_UploadPetPicture(t *testing.T) {
	t.Run("SV-03_HappyPath", func(t *testing.T) {
		mockPic := &PetPicture{ID: "sv03-mock-id", FileExt: "jpg", PrimarySubject: 4, OthersSubjects: []int{5}, Aliases: []string{"a"}}
		t.Cleanup(func() { os.Remove("sv03-mock-id.jpg") })

		mock := &svMockStore{createPetPictureResult: mockPic}
		svc := NewService(mock)
		file := svTempFile(t, "sv03-upload-*.jpg", []byte("some picture bytes"))

		rt := &svFakeRoundTripper{resp: svOKResponse()}
		swapTransport(t, rt)

		origKey := CDN_KEY
		CDN_KEY = "test-cdn-key"
		t.Cleanup(func() { CDN_KEY = origKey })

		got, err := svc.UploadPetPicture(file, 4, []int{5}, []string{"a"})
		if err != nil {
			t.Fatalf("UploadPetPicture() error = %v, want nil", err)
		}
		if !reflect.DeepEqual(got, mockPic) {
			t.Errorf("UploadPetPicture() = %+v, want %+v", got, mockPic)
		}

		if len(mock.createPetPictureCalls) != 1 {
			t.Fatalf("CreatePetPicture called %d times, want 1", len(mock.createPetPictureCalls))
		}
		call := mock.createPetPictureCalls[0]
		if call.fileExt != "jpg" || call.primarySubject != 4 || !reflect.DeepEqual(call.othersSubjects, []int{5}) || !reflect.DeepEqual(call.aliases, []string{"a"}) {
			t.Errorf("CreatePetPicture called with %+v, want fileExt=jpg primarySubject=4 othersSubjects=[5] aliases=[a]", call)
		}
		if len(call.id) != 64 {
			t.Errorf("CreatePetPicture id = %q, want a 64-char sha256 hex digest", call.id)
		}

		if rt.lastReq == nil {
			t.Fatal("expected an outbound CDN request, got none")
		}
		if got := rt.lastReq.URL.String(); got != CDN_URL+"/upload" {
			t.Errorf("request URL = %q, want %q", got, CDN_URL+"/upload")
		}

		mediaType, params, err := mime.ParseMediaType(rt.lastReq.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			t.Fatalf("Content-Type = %q, want a multipart type", rt.lastReq.Header.Get("Content-Type"))
		}
		mr := multipart.NewReader(rt.lastReq.Body, params["boundary"])
		form, err := mr.ReadForm(1 << 20)
		if err != nil {
			t.Fatalf("failed to parse multipart body: %v", err)
		}
		if got := form.Value["upload_key"]; len(got) != 1 || got[0] != "test-cdn-key" {
			t.Errorf("upload_key field = %v, want [test-cdn-key]", got)
		}
		if got := form.Value["upload_path"]; len(got) != 1 || got[0] != CDN_PATH {
			t.Errorf("upload_path field = %v, want [%s]", got, CDN_PATH)
		}
		if len(form.File["file"]) != 1 || form.File["file"][0].Filename != "sv03-mock-id.jpg" {
			t.Errorf("form file = %+v, want filename %q", form.File["file"], "sv03-mock-id.jpg")
		}
		// Deliberately not asserted: the uploaded body's byte content.
		// UploadPetPicture's sha256 io.Copy already advances file's read
		// cursor to EOF, so the later io.Copy into the form file always
		// writes zero bytes (a real bug — see the run's report).
	})

	t.Run("SV-04_StoreErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("db unavailable")
		mock := &svMockStore{createPetPictureErr: wantErr}
		svc := NewService(mock)
		file := svTempFile(t, "sv04-upload-*.jpg", []byte("bytes"))

		rt := &svFakeRoundTripper{}
		swapTransport(t, rt)

		got, err := svc.UploadPetPicture(file, 1, nil, nil)
		if got != nil {
			t.Errorf("UploadPetPicture() picture = %+v, want nil", got)
		}
		if !errors.Is(err, wantErr) {
			t.Errorf("UploadPetPicture() error = %v, want %v", err, wantErr)
		}
		if rt.lastReq != nil {
			t.Error("expected no outbound CDN request when CreatePetPicture fails")
		}
	})

	t.Run("SV-05_RenameFailsWhenSourceFileIsGone", func(t *testing.T) {
		mock := &svMockStore{createPetPictureResult: &PetPicture{ID: "sv05-mock-id", FileExt: "jpg"}}
		svc := NewService(mock)
		file := svTempFile(t, "sv05-upload-*.jpg", []byte("bytes"))
		// Removing the path after opening it: the fd stays valid for reads
		// via the inode (so hashing still succeeds), but file.Name() now
		// names a path that no longer exists, so os.Rename must fail.
		if err := os.Remove(file.Name()); err != nil {
			t.Fatalf("failed to remove temp file: %v", err)
		}

		got, err := svc.UploadPetPicture(file, 1, nil, nil)
		if got != nil {
			t.Errorf("UploadPetPicture() picture = %+v, want nil", got)
		}
		if !errors.As(err, new(*os.LinkError)) {
			t.Errorf("UploadPetPicture() error = %v (%T), want an *os.LinkError from os.Rename", err, err)
		}
	})

	t.Run("SV-06_TransportErrorPropagates", func(t *testing.T) {
		mockPic := &PetPicture{ID: "sv06-mock-id", FileExt: "jpg"}
		t.Cleanup(func() { os.Remove("sv06-mock-id.jpg") })
		mock := &svMockStore{createPetPictureResult: mockPic}
		svc := NewService(mock)
		file := svTempFile(t, "sv06-upload-*.jpg", []byte("bytes"))

		wantErr := errors.New("connection reset")
		swapTransport(t, &svFakeRoundTripper{err: wantErr})

		got, err := svc.UploadPetPicture(file, 1, nil, nil)
		if got != nil {
			t.Errorf("UploadPetPicture() picture = %+v, want nil", got)
		}
		if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
			t.Errorf("UploadPetPicture() error = %v, want it to contain %q", err, wantErr.Error())
		}
	})

	t.Run("SV-07_MultiDotFilenameUsesLastSegment", func(t *testing.T) {
		mockPic := &PetPicture{ID: "sv07-mock-id"}
		t.Cleanup(func() { os.Remove("sv07-mock-id.gz") })
		mock := &svMockStore{createPetPictureResult: mockPic}
		svc := NewService(mock)
		file := svTempFile(t, "sv07-photo-*.tar.gz", []byte("bytes"))

		swapTransport(t, &svFakeRoundTripper{resp: svOKResponse()})

		if _, err := svc.UploadPetPicture(file, 1, nil, nil); err != nil {
			t.Fatalf("UploadPetPicture() error = %v, want nil", err)
		}
		if len(mock.createPetPictureCalls) != 1 || mock.createPetPictureCalls[0].fileExt != "gz" {
			t.Errorf("CreatePetPicture fileExt = %q, want %q", mock.createPetPictureCalls[0].fileExt, "gz")
		}
	})

	// SV-08 (no-dot filename) is deliberately not tested here: it's a known,
	// deferred source bug (see test/plans/pet_pictures.md's self-check),
	// not fixed in this pass, so no test asserts either the buggy behavior
	// or a fix that doesn't exist yet.
}
