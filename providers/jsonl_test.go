package providers

import (
	"encoding/json"
	"errors"
	"fmt"
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

func TestStreamJSONLCallsOnRowPerRow(t *testing.T) {
	in := strings.NewReader(`
{"name": "alice", "age": 30}
{"name": "bob", "age": 25}
{"name": "carol", "age": 41}
`)
	var names []string
	var seenKeys [][]string
	err := StreamJSONL(in, func(row map[string]any, firstKeys []string) error {
		names = append(names, row["name"].(string))
		seenKeys = append(seenKeys, firstKeys)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamJSONL: %v", err)
	}
	if got, want := names, []string{"alice", "bob", "carol"}; !reflect.DeepEqual(got, want) {
		t.Errorf("names: got %v, want %v", got, want)
	}
	// firstKeys must be the same slice across every callback invocation.
	wantKeys := []string{"name", "age"}
	for i, ks := range seenKeys {
		if !reflect.DeepEqual(ks, wantKeys) {
			t.Errorf("seenKeys[%d]: got %v, want %v", i, ks, wantKeys)
		}
	}
}

func TestStreamJSONLOnRowErrorStopsScan(t *testing.T) {
	in := strings.NewReader(`
{"a": 1}
{"a": 2}
{"a": 3}
`)
	stop := errors.New("stop")
	var seen int
	err := StreamJSONL(in, func(row map[string]any, _ []string) error {
		seen++
		if seen == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatalf("expected stop sentinel, got %v", err)
	}
	if seen != 2 {
		t.Errorf("seen: got %d, want 2 (scan should stop on first error)", seen)
	}
}

func TestLoadStdinJSONL(t *testing.T) {
	in := strings.NewReader(`
{"name": "alice", "age": 30}
{"name": "bob", "age": 25}
`)
	rows, firstKeys, err := LoadStdin(in)
	if err != nil {
		t.Fatalf("LoadStdin: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d, want 2", len(rows))
	}
	if !reflect.DeepEqual(firstKeys, []string{"name", "age"}) {
		t.Errorf("firstKeys: got %v, want [name age]", firstKeys)
	}
}

func TestLoadStdinSingleObject(t *testing.T) {
	// A single multi-line JSON object is parsed as one document and
	// dispatched through rowsFromTree, which produces one row per
	// top-level key.
	in := strings.NewReader(`{
  "alice": {"age": 30},
  "bob": {"age": 25}
}`)
	rows, firstKeys, err := LoadStdin(in)
	if err != nil {
		t.Fatalf("LoadStdin: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d, want 2", len(rows))
	}
	// firstKeys is nil for the single-doc path; resolveCols picks alpha order.
	if firstKeys != nil {
		t.Errorf("firstKeys: got %v, want nil for single-doc map root", firstKeys)
	}
}

func TestLoadStdinArrayOfObjects(t *testing.T) {
	// Top-level JSON array of objects is treated like JSONL: each element
	// becomes a row directly, no synthetic `key` column.
	in := strings.NewReader(`[
  {"name": "alice", "age": 30},
  {"name": "bob", "age": 25}
]`)
	rows, firstKeys, err := LoadStdin(in)
	if err != nil {
		t.Fatalf("LoadStdin: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d, want 2", len(rows))
	}
	if rows[0]["name"] != "alice" || rows[1]["name"] != "bob" {
		t.Errorf("rows: %+v", rows)
	}
	if _, hasKey := rows[0]["key"]; hasKey {
		t.Errorf("rows[0] should NOT have a synthetic `key` column for array-of-objects: %+v", rows[0])
	}
	if !reflect.DeepEqual(firstKeys, []string{"name", "age"}) {
		t.Errorf("firstKeys: got %v, want [name age]", firstKeys)
	}
}

func TestLoadStdinArrayOfScalars(t *testing.T) {
	// A heterogeneous array (or one without object elements) falls through
	// to rowsFromTree, which adds the `key` (index) column.
	in := strings.NewReader(`[1, 2, 3]`)
	rows, _, err := LoadStdin(in)
	if err != nil {
		t.Fatalf("LoadStdin: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows: got %d, want 3", len(rows))
	}
	for i, r := range rows {
		if r["key"] != fmt.Sprintf("%d", i) {
			t.Errorf("rows[%d].key: got %v, want %d", i, r["key"], i)
		}
	}
}

func TestLoadStdinEmpty(t *testing.T) {
	rows, firstKeys, err := LoadStdin(strings.NewReader(""))
	if err != nil {
		t.Fatalf("LoadStdin: %v", err)
	}
	if rows != nil || firstKeys != nil {
		t.Errorf("empty input should produce nil rows and nil firstKeys, got %+v / %v", rows, firstKeys)
	}
}

func TestStreamJSONLPreservesNumberType(t *testing.T) {
	in := strings.NewReader(`{"n": 42}` + "\n")
	var n any
	if err := StreamJSONL(in, func(row map[string]any, _ []string) error {
		n = row["n"]
		return nil
	}); err != nil {
		t.Fatalf("StreamJSONL: %v", err)
	}
	num, ok := n.(json.Number)
	if !ok || num.String() != "42" {
		t.Errorf("n: got %T(%v), want json.Number(42)", n, n)
	}
}
