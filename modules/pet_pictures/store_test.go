package petpictures

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// ppUnusedTCPPort returns a port likely free right now, so a later connect
// to it fails fast with "connection refused" instead of hanging.
func ppUnusedTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find an unused port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close probe listener: %v", err)
	}
	return port
}

// ppSetUnreachableDatabaseURL points DATABASE_URL at a closed local port. It
// must stay a bare "scheme://user:pass@host:port" DSN — store.go appends
// "/pet_pictures" itself. pgxpool.New parses eagerly but dials lazily, so
// this never trips database.GetDB's log.Fatal; the query fails later with a
// genuine connection-refused error.
func ppSetUnreachableDatabaseURL(t *testing.T) {
	t.Helper()
	port := ppUnusedTCPPort(t)
	t.Setenv("DATABASE_URL", fmt.Sprintf("postgres://user:pass@127.0.0.1:%d", port))
}

func ppUniqueID(prefix string) string {
	return prefix + "-" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

func TestST01_NewStore(t *testing.T) {
	t.Run("ST-01_WrapsGivenPool", func(t *testing.T) {
		if s := NewStore(nil); s == nil {
			t.Error("NewStore(nil) = nil, want a non-nil PetPicStore")
		}
	})
}

func TestST02to05_CreatePet(t *testing.T) {
	t.Run("ST-04_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.CreatePet(ppUniqueID("pet")); err == nil {
			t.Error("CreatePet() error = nil, want a connection error")
		}
	})
}

func TestST07to09_GetPetByName(t *testing.T) {
	t.Run("ST-09_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.GetPetByName("anything"); err == nil {
			t.Error("GetPetByName() error = nil, want a connection error")
		}
	})
}

func TestST10to12_UpdatePet(t *testing.T) {
	t.Run("ST-12_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.UpdatePet(&Pet{ID: 1, Name: "x"}); err == nil {
			t.Error("UpdatePet() error = nil, want a connection error")
		}
	})
}

func TestST13to16_CreatePetPicture(t *testing.T) {
	t.Run("ST-15_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.CreatePetPicture(ppUniqueID("pic"), "jpg", 1, nil, nil); err == nil {
			t.Error("CreatePetPicture() error = nil, want a connection error")
		}
	})
}

func TestST19to20_GetPetPicture(t *testing.T) {
	t.Run("ST-20_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.GetPetPicture("anything"); err == nil {
			t.Error("GetPetPicture() error = nil, want a connection error")
		}
	})
}

func TestST21_UpdatePetPicture(t *testing.T) {
	t.Run("ST-21_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.UpdatePetPicture(PetPicture{ID: "anything"}); err == nil {
			t.Error("UpdatePetPicture() error = nil, want a connection error")
		}
	})
}

func TestST22to24_DeletePetPicture(t *testing.T) {
	t.Run("ST-24_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.DeletePetPicture("anything"); err == nil {
			t.Error("DeletePetPicture() error = nil, want a connection error")
		}
	})
}
