package rbac

import (
	"slices"
	"testing"
)

func TestGR01to08FlattenGrants(t *testing.T) {
	flatten := func(t *testing.T, grants ...grant) []string {
		t.Helper()
		got, err := flattenGrants(grants)
		if err != nil {
			t.Fatalf("flattenGrants() err = %v", err)
		}
		return got
	}
	check := func(t *testing.T, got []string, want ...string) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	t.Run("GR-01_NoGrantsGiveAnEmptyNonNilList", func(t *testing.T) {
		got := flatten(t)
		if got == nil || len(got) != 0 {
			t.Fatalf("got %#v, want an empty non-nil list", got)
		}
	})
	t.Run("GR-02_BareNodesAreListedOncePerNode", func(t *testing.T) {
		check(t, flatten(t, grant{node: "a.x"}, grant{node: "a.x"}, grant{node: "b.y"}), "a.x", "b.y")
	})
	t.Run("GR-03_IntValuesMergeByTheMergeRule", func(t *testing.T) {
		check(t, flatten(t, grant{"n", ValueTypeInt, MergeMax, []byte("100")}, grant{"n", ValueTypeInt, MergeMax, []byte("1000")}, grant{"n", ValueTypeInt, MergeMax, []byte("20")}), "n:1000")
		check(t, flatten(t, grant{"n", ValueTypeInt, MergeMin, []byte("100")}, grant{"n", ValueTypeInt, MergeMin, []byte("1000")}, grant{"n", ValueTypeInt, MergeMin, []byte("20")}), "n:20")
		check(t, flatten(t, grant{"n", ValueTypeInt, MergeMax, []byte("-5")}, grant{"n", ValueTypeInt, MergeMax, []byte("-9")}), "n:-5")
	})
	t.Run("GR-04_StringValueIsTheFirstGrantInRoleOrder", func(t *testing.T) {
		check(t, flatten(t, grant{"n", ValueTypeString, MergeFirst, []byte(`"one"`)}, grant{"n", ValueTypeString, MergeFirst, []byte(`"two"`)}), "n:one")
	})
	t.Run("GR-05_ListValuesAreUnitedSortedAndDeduplicated", func(t *testing.T) {
		check(t, flatten(t, grant{"n", ValueTypeStringList, MergeUnion, []byte(`["b","a"]`)}, grant{"n", ValueTypeStringList, MergeUnion, []byte(`["c","a"]`)}), "n:a", "n:b", "n:c")
	})
	t.Run("GR-06_ValuesMayContainColons", func(t *testing.T) {
		check(t, flatten(t, grant{"n", ValueTypeStringList, MergeUnion, []byte(`["a:b"]`)}, grant{"s", ValueTypeString, MergeFirst, []byte(`"x:y:z"`)}), "n:a:b", "s:x:y:z")
	})
	t.Run("GR-07_ValuedNodesWithoutAValueAreBare", func(t *testing.T) {
		check(t, flatten(t, grant{"a", ValueTypeInt, MergeMax, nil}, grant{"b", ValueTypeStringList, MergeUnion, []byte(`[]`)}, grant{"c", "unknown", "", []byte(`1`)}), "a", "b", "c")
		check(t, flatten(t, grant{"n", ValueTypeInt, MergeMax, nil}, grant{"n", ValueTypeInt, MergeMax, []byte("7")}), "n:7")
	})
	t.Run("GR-08_StoredValuesOfTheWrongShapeAreErrors", func(t *testing.T) {
		for _, g := range []grant{
			{"n", ValueTypeInt, MergeMax, []byte(`"x"`)},
			{"n", ValueTypeInt, MergeMax, []byte(`1.5`)},
			{"n", ValueTypeString, MergeFirst, []byte(`5`)},
			{"n", ValueTypeStringList, MergeUnion, []byte(`"a"`)},
			{"n", ValueTypeStringList, MergeUnion, []byte(`{`)},
		} {
			if got, err := flattenGrants([]grant{g}); err == nil || got != nil {
				t.Errorf("flattenGrants(%+v) = (%v, %v), want an error", g, got, err)
			}
		}
	})
}
