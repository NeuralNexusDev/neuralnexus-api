package beenamegenerator

import "testing"

func TestTY01to02_NewBeeName(t *testing.T) {
	t.Run("TY-01_NormalName", func(t *testing.T) {
		got := NewBeeName("Buzzy")
		if got == nil {
			t.Fatal("NewBeeName() = nil, want non-nil")
		}
		if got.Name != "Buzzy" {
			t.Errorf("Name = %q, want %q", got.Name, "Buzzy")
		}
	})

	t.Run("TY-02_EmptyName", func(t *testing.T) {
		got := NewBeeName("")
		if got == nil {
			t.Fatal("NewBeeName() = nil, want non-nil")
		}
		if got.Name != "" {
			t.Errorf("Name = %q, want empty string", got.Name)
		}
	})
}

func TestTY03to04_NewBeeNameSuggestions(t *testing.T) {
	t.Run("TY-03_PopulatedSlice", func(t *testing.T) {
		got := NewBeeNameSuggestions([]string{"a", "b"})
		if got == nil {
			t.Fatal("NewBeeNameSuggestions() = nil, want non-nil")
		}
		if len(got.Suggestions) != 2 || got.Suggestions[0] != "a" || got.Suggestions[1] != "b" {
			t.Errorf("Suggestions = %v, want [a b]", got.Suggestions)
		}
	})

	t.Run("TY-04_NilSlice", func(t *testing.T) {
		got := NewBeeNameSuggestions(nil)
		if got == nil {
			t.Fatal("NewBeeNameSuggestions() = nil, want non-nil")
		}
		if got.Suggestions != nil {
			t.Errorf("Suggestions = %v, want nil", got.Suggestions)
		}
	})
}
