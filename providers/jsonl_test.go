package providers

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLoadJSONLOrderedKeys(t *testing.T) {
	in := strings.NewReader(`
# leading comment
{"name": "alice", "age": 30, "city": "Paris"}

{"name": "bob", "age": 25, "city": "Berlin"}
# trailing comment
`)
	rows, firstKeys, err := LoadJSONL(in)
	if err != nil {
		t.Fatalf("LoadJSONL: %v", err)
	}
	wantFirstKeys := []string{"name", "age", "city"}
	if !reflect.DeepEqual(firstKeys, wantFirstKeys) {
		t.Errorf("firstKeys: got %v, want %v", firstKeys, wantFirstKeys)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0]["name"] != "alice" || rows[1]["name"] != "bob" {
		t.Errorf("rows: %+v", rows)
	}
	// json.Number preserves int/float distinction, matching loadJSON.
	if got, ok := rows[0]["age"].(json.Number); !ok || got.String() != "30" {
		t.Errorf("rows[0].age = %T(%v), want json.Number(30)", rows[0]["age"], rows[0]["age"])
	}
}

func TestLoadJSONLMixedSchemas(t *testing.T) {
	in := strings.NewReader(`
{"a": 1, "b": 2}
{"b": 3, "c": 4}
`)
	rows, firstKeys, err := LoadJSONL(in)
	if err != nil {
		t.Fatalf("LoadJSONL: %v", err)
	}
	// firstKeys reflects the first object only — extras in later rows are
	// still parsed into the row, just not echoed in firstKeys.
	wantFirstKeys := []string{"a", "b"}
	if !reflect.DeepEqual(firstKeys, wantFirstKeys) {
		t.Errorf("firstKeys: got %v, want %v", firstKeys, wantFirstKeys)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if _, ok := rows[1]["c"]; !ok {
		t.Errorf("rows[1] should retain key c, got %+v", rows[1])
	}
}

func TestLoadJSONLMalformedLineFails(t *testing.T) {
	in := strings.NewReader(`
{"a": 1}
not valid json
`)
	_, _, err := LoadJSONL(in)
	if err == nil {
		t.Fatal("expected error on malformed line")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error should mention line number, got %q", err)
	}
}

func TestLoadJSONLNonObjectFails(t *testing.T) {
	in := strings.NewReader(`["not", "an", "object"]` + "\n")
	_, _, err := LoadJSONL(in)
	if err == nil {
		t.Fatal("expected error when first line is not an object")
	}
}

func TestLoadJSONLEmptyInput(t *testing.T) {
	rows, firstKeys, err := LoadJSONL(strings.NewReader(""))
	if err != nil {
		t.Fatalf("LoadJSONL: %v", err)
	}
	if rows != nil {
		t.Errorf("rows: got %+v, want nil", rows)
	}
	if firstKeys != nil {
		t.Errorf("firstKeys: got %v, want nil", firstKeys)
	}
}
