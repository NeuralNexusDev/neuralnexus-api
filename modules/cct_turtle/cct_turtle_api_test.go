package cctturtle

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/public"
)

func TestGetTurtleCode(t *testing.T) {
	t.Run("CT-01_StartupLuaServed", func(t *testing.T) {
		want, err := fs.ReadFile(public.FS, "cct_turtle/startup.lua")
		if err != nil {
			t.Fatalf("read embedded startup.lua: %v", err)
		}
		if len(want) == 0 {
			t.Fatal("embedded startup.lua is empty")
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cct-turtle/startup.lua", nil)
		w := httptest.NewRecorder()

		GetTurtleCode(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != string(want) {
			t.Fatalf("body does not match the embedded startup.lua")
		}
	})
}

func TestGetTurtleUpdatingCode(t *testing.T) {
	t.Run("CT-02_UpdatingStartupLuaServed", func(t *testing.T) {
		want, err := fs.ReadFile(public.FS, "cct_turtle/updating_startup.lua")
		if err != nil {
			t.Fatalf("read embedded updating_startup.lua: %v", err)
		}
		if len(want) == 0 {
			t.Fatal("embedded updating_startup.lua is empty")
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cct-turtle/startup.lua", nil)
		w := httptest.NewRecorder()

		GetTurtleUpdatingCode(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != string(want) {
			t.Fatalf("body does not match the embedded updating_startup.lua")
		}
	})
}
