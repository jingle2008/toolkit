package columns

import (
	"reflect"
	"testing"
)

// TestRenderFlat exercises the type-assertion + select + render path
// against a tiny inline Set so the helper has direct coverage,
// independent of any per-category registration.
func TestRenderFlat(t *testing.T) {
	t.Parallel()
	type item struct {
		Name, Status string
	}
	s := Set[item]{Columns: []Column[item]{
		{Title: "Name", Key: "name", Ratio: 0.5,
			Render: func(i item) string { return i.Name }},
		{Title: "Status", Key: "status", Ratio: 0.5,
			Render: func(i item) string { return i.Status }},
	}}
	items := []item{{"a", "ok"}, {"b", "fail"}}

	headers, rows, err := renderFlat(s, items, nil)
	if err != nil {
		t.Fatalf("renderFlat: %v", err)
	}
	if !reflect.DeepEqual(headers, []string{"NAME", "STATUS"}) {
		t.Errorf("headers: got %v, want [NAME STATUS]", headers)
	}
	want := [][]string{{"a", "ok"}, {"b", "fail"}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows: got %v, want %v", rows, want)
	}

	// Selected columns: only "status".
	_, rows, err = renderFlat(s, items, []string{"status"})
	if err != nil {
		t.Fatalf("renderFlat selected: %v", err)
	}
	if !reflect.DeepEqual(rows, [][]string{{"ok"}, {"fail"}}) {
		t.Errorf("selected rows: got %v", rows)
	}

	// Wrong type → error, not panic.
	if _, _, err := renderFlat(s, "not a slice", nil); err == nil {
		t.Error("renderFlat with wrong type: expected error")
	}

	// Unknown key → error.
	if _, _, err := renderFlat(s, items, []string{"bogus"}); err == nil {
		t.Error("renderFlat with unknown key: expected error")
	}
}

// TestRenderGrouped exercises the type-assertion + select + render +
// sorted-keys iteration path against a tiny inline GroupedSet.
func TestRenderGrouped(t *testing.T) {
	t.Parallel()
	type item struct {
		V string
	}
	g := GroupedSet[item]{Columns: []GroupedColumn[item]{
		{Title: "Key", Key: "key", Ratio: 0.5,
			Render: func(k string, _ item) string { return k }},
		{Title: "Val", Key: "val", Ratio: 0.5,
			Render: func(_ string, i item) string { return i.V }},
	}}
	items := map[string][]item{
		"b": {{V: "z"}},
		"a": {{V: "x"}, {V: "y"}},
	}

	headers, rows, err := renderGrouped(g, items, nil)
	if err != nil {
		t.Fatalf("renderGrouped: %v", err)
	}
	if !reflect.DeepEqual(headers, []string{"KEY", "VAL"}) {
		t.Errorf("headers: got %v", headers)
	}
	// Sorted-key iteration: a's items first (x then y), then b's (z).
	want := [][]string{{"a", "x"}, {"a", "y"}, {"b", "z"}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows: got %v, want %v", rows, want)
	}

	// Wrong type → error.
	if _, _, err := renderGrouped(g, []item{}, nil); err == nil {
		t.Error("renderGrouped with wrong type: expected error")
	}
}
